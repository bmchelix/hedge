/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training_data

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"hedge/common/client"
	db2 "hedge/common/db"
	"hedge/common/service"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/data"
	"hedge/edge-ml-service/pkg/helpers"
)

const BATCH_SIZE int64 = 5

var RowCountLimit = 100

type DataCollectorInterface interface {
	Execute() error
	ExecuteWithFileNames(trainingDataFile string) error
	GenerateConfig(trainingConfigFile string) error
	GenerateSample(dataSampleSize int) ([]string, error)
	GetTrainingDataSample(start, end int64, rowCountByGroup *sync.Map) ([][]string, []string, error)
	GetSampleDataSetFromDB(start int64, end int64) ([][]*data.TSDataElement, error)
	GetMlStorage() helpers.MLStorageInterface
	SetPreProcessor(preProcessor helpers.PreProcessorInterface)
	GetPreProcessor() helpers.PreProcessorInterface
	SetFetchStatusManager(statusFileName string)
	GetFetchStatusManager() FetchStatusManagerInterface
}

type DataCollector struct {
	PreProcessor         helpers.PreProcessorInterface
	LoggingClient        logger.LoggingClient
	AppConfig            *config.MLMgmtConfig
	DataStoreProvider    service.DataStoreProvider
	BaseQuery            string
	ChunkSize            int
	MlStorage            helpers.MLStorageInterface
	TrainingDataFileName string
	Client               client.HTTPClient
	FetchStatusManager   FetchStatusManagerInterface
}

func (dc *DataCollector) Execute() error {
	trainingDataFile := dc.MlStorage.GetTrainingDataFileName()
	trainingConfigFile := dc.MlStorage.GetMLModelConfigFileName(true)
	err := dc.GenerateConfig(trainingConfigFile)
	if err != nil {
		return err
	}
	return dc.ExecuteWithFileNames(trainingDataFile)
}

// Generates training data as per the initialized values in DataCollector, Takes help from Pre-processor
func (dc *DataCollector) ExecuteWithFileNames(trainingDataFile string) error {

	endTime := time.Now().Unix()
	var startTime int64

	//endTime = 1663144002
	var count int64 = 0
	var loopCount int64 = 0
	var minTime int64 = endTime - dc.PreProcessor.GetMLModelConfig().MLDataSourceConfig.TrainingDataSourceConfig.DataCollectionTotalDurationSec

	// Delete existing training_data.csv before downloading

	err := dc.MlStorage.InitializeFile(trainingDataFile)
	if err != nil {
		return err
	}
	// Create a thread-safe map for row counting by group
	rowCountByGroup := &sync.Map{}

	duration := dc.PreProcessor.GetMLModelConfig().MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs * BATCH_SIZE
	//for count < dataCollector.trainingDataConfig.TrainingDataCount {
	for endTime > minTime {
		loopCount++
		//startTime = endTime - duration
		startTime = intMax(endTime-duration, minTime)
		records, headers, err := dc.GetTrainingDataSample(startTime, endTime, rowCountByGroup)
		if err != nil {
			dc.LoggingClient.Errorf("data collection being stopped, will terminate data-collection, error: %v", err)
			return err
		}
		if len(records) > 0 {
			count = count + int64(len(records))
			err = dc.MlStorage.AddTrainingDataToFileWithFileName(records, headers, trainingDataFile)
			//err = dc.addTrainingDataToFile(records, headers, trainingDataFile)
			if err != nil {
				dc.LoggingClient.Errorf("error while adding data to training file, will terminate data-collection, error: %v", err)
				return err
			}
		}
		endTime = startTime
		// Allowing buffer of 10*BATCH_SIZE to go back in time to fetch required quantity of training data
		// There might be gaps in data generation or some of the metric collection frequency is much higher than the sample size that
		// is configured for data collection
		//commented temporarily if loopCount >= dataCollector.trainingDataConfig.TrainingDataCount*10 {
		//	return errors.New("No valid data found for the metrics selected")
		//}
	}
	if count == 0 {
		return db2.ErrNotFound
	}
	//	dc.trainingDataFileName = trainingDataFile
	err = dc.AnalyzeRowCounts(rowCountByGroup, dc.PreProcessor.GetMLModelConfig().Name)
	if err != nil {
		return err
	}

	return nil
}

func (dc *DataCollector) GenerateConfig(trainingConfigFile string) error {
	// Write the training config file; kind of take a snapshop of the config at that time
	// Update trainingData config with algo information

	err := dc.MlStorage.InitializeFile(trainingConfigFile)
	if err != nil {
		return err
	}

	// Create a composite ML config containing both the ML model config and the ML algorithm definition
	compositeConfig := config.MLConfig{
		MLModelConfig:    *dc.PreProcessor.GetMLModelConfig(),
		MLAlgoDefinition: *dc.PreProcessor.GetMLAlgorithmDefinition(),
	}

	// update MLModelConfig for externalFeatures
	if len(compositeConfig.MLModelConfig.MLDataSourceConfig.ExternalFeatures) > 0 {
		if len(compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile) == 0 {
			compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile = make(map[string][]config.Feature)
		}
		compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile["external"] = compositeConfig.MLModelConfig.MLDataSourceConfig.ExternalFeatures
		// Rebuild feature to Index map again or rather it should have been right earlier, so fix that
	}

	// Marshal the composite config to JSON
	bytes, err := json.MarshalIndent(compositeConfig, "", " ")
	if err != nil {
		return err
	}
	err = os.WriteFile(trainingConfigFile, bytes, 0644)
	return err
}

// Generates training data sample as per the initialized values in DataCollector, Takes help from Pre-processor
func (dc *DataCollector) GenerateSample(dataSampleSize int) ([]string, error) {

	endTime := time.Now().Unix()
	var startTime int64

	var count int = 0
	var loopCount int64 = 0
	var minTime int64 = endTime - dc.PreProcessor.GetMLModelConfig().MLDataSourceConfig.TrainingDataSourceConfig.DataCollectionTotalDurationSec

	duration := dc.PreProcessor.GetMLModelConfig().MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs * BATCH_SIZE
	sampleData := make([]string, 0)

	for endTime > minTime {
		loopCount++
		//startTime = endTime - duration
		startTime = intMax(endTime-duration, minTime)
		records, headers, err := dc.GetTrainingDataSample(startTime, endTime, nil)
		dc.LoggingClient.Debugf("initial data sample records count: %d", len(records))

		if len(sampleData) == 0 {
			//write header once
			sampleData = append(sampleData, strings.Join(headers, ","))
		}
		if err != nil {
			dc.LoggingClient.Errorf("error from GetTrainingDataSample, exiting, error: %v", err)
			return sampleData, err
		}
		if len(records) > 0 {
			// Limit records length if exceeds dataSampleSize
			if len(records) > dataSampleSize {
				records = records[:dataSampleSize]
			}
			count = count + len(records)

			for _, recordArray := range records {
				//sampleData is an array of strings, with each string containing CSVs
				sampleData = append(sampleData, strings.Join(recordArray, ","))
			}
		}
		endTime = startTime
		if count >= dataSampleSize {
			break // break here
		}
		// Allowing buffer of 10*BATCH_SIZE to go back in time to fetch required quantity of training data
		// There might be gaps in data generation or some of the metric collection frequency is much higher than the sample size that
		// is configured for data collection
		//commented temporarily if loopCount >= dataCollector.trainingDataConfig.TrainingDataCount*10 {
		//	return errors.New("No valid data found for the metrics selected")
		//}
	}
	//	dc.trainingDataFileName = trainingDataFile
	// Write the training config file; kind of take a snapshop of the config at that time
	// Update trainingData config with algo information
	if count == 0 {
		return sampleData, db2.ErrNotFound
	}
	return sampleData, nil
}

// Get a sample training data chunk
func (dc *DataCollector) GetTrainingDataSample(start, end int64, rowCountByGroup *sync.Map) ([][]string, []string, error) {
	numColumns := len(dc.PreProcessor.GetFeaturesToIndex()) //len(tc.trainingDataConfig.TrainingDataQueries)
	// Need to fix this based on grouping, For now don't allow same metric name to be repeated
	featureNames := make([]string, numColumns)
	for featureName, columnNo := range dc.PreProcessor.GetFeaturesToIndex() {
		featureNames[columnNo] = featureName
	}

	rawTSData, err := dc.GetSampleDataSetFromDB(start, end)
	if err != nil {
		dc.LoggingClient.Warnf("No training data found for date range from %d to %d, error: %v", start, end, err)
		return nil, nil, err
	}

	trgDataSet := make([][]string, 0)
	rowNo := 0
	for _, tsDataRow := range rawTSData { // Each row represents metrics all at the exact timestamp
		groupedTSDataRow := dc.PreProcessor.GroupSamples(tsDataRow)
		for groupName, tsSamples := range groupedTSDataRow {
			dc.LoggingClient.Debugf("Creating feature set for group: %s", groupName)
			// We should not lose the groupName, for now assume it is in sample Data under GroupBy
			featureRow, featuresComplete := dc.PreProcessor.TransformToFeature(groupName, tsSamples)
			if !featuresComplete {
				dc.LoggingClient.Debugf("Complete feature set not found for group: %s", groupName)
				continue
			}
			for _, dataRow := range featureRow {

				if len(dataRow) != numColumns {
					errMsg := fmt.Sprintf("issue with training data, expected feature count: %d, found: %d\n", numColumns, len(dataRow))
					return nil, nil, errors.New(errMsg)
				}

				newRow := make([]string, numColumns)
				for col, val := range dataRow {
					newRow[col] = fmt.Sprintf("%v", val)
				}
				trgDataSet = append(trgDataSet, newRow)
				rowNo++

				if rowCountByGroup != nil {
					// Safely increment the row count for the group
					for {
						value, _ := rowCountByGroup.LoadOrStore(groupName, 0)
						currentCount := value.(int)
						if rowCountByGroup.CompareAndSwap(groupName, currentCount, currentCount+1) {
							break
						}
					}
				}
			}
		}
	}

	if rowNo > 0 {
		dc.LoggingClient.Debugf("Sample feature collected, size=%d", rowNo)
	} else {
		dc.LoggingClient.Debugf("No valid training feature found for the date range from %d to %d", start, end)
	}
	return trgDataSet, featureNames, nil
}

func (dc *DataCollector) AnalyzeRowCounts(rowCountByGroup *sync.Map, mlModelConfigName string) error {
	atLeastOneGroupWithEnoughRows := false

	// Check if at least 1 of groups has enough rows
	rowCountByGroup.Range(func(key, value interface{}) bool {
		groupName := key.(string)
		rowCount := value.(int)
		if rowCount >= RowCountLimit {
			dc.LoggingClient.Debugf("Group %s has sufficient rows: %d", groupName, rowCount)
			atLeastOneGroupWithEnoughRows = true
			return false
		}
		return true
	})

	if !atLeastOneGroupWithEnoughRows {
		errMsg := fmt.Sprintf("Not enough data has been collected for training the ML model: %s, no group names have >= 100 records", mlModelConfigName)
		dc.LoggingClient.Errorf(errMsg)
		return errors.New(errMsg)
	}

	return nil
}

func (dc *DataCollector) GetSampleDataSetFromDB(start int64, end int64) ([][]*data.TSDataElement, error) {

	var timeSeriesResponse data.TimeSeriesResponse
	//http://clm-aus-swwne9.bmc.com:8428/api/v1/query_range?query=TurbinePower{edgeNode="raspi-pune.bmc.com", device=~"AltaNS14"}&start=1604289920&end=1604304320&step=10
	if dc.Client == nil {
		dc.Client = &http.Client{}
	}
	iotUrl := dc.DataStoreProvider.GetDataURL() + "/query_range"

	request, err := http.NewRequest("GET", iotUrl, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-type", "application/json")
	dc.DataStoreProvider.SetAuthHeader(request)
	q := request.URL.Query()
	if len(dc.BaseQuery) == 0 {
		return nil, errors.New("Error with Training data, training job will be skipped")
	}
	q.Add("query", dc.BaseQuery) // Add a new value to the set
	// Add time range and steps
	stepSize := dc.PreProcessor.GetMLModelConfig().MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs // Sampling interval in secs
	stepSizStr := strconv.FormatInt(stepSize, 10)
	q.Add("step", stepSizStr) // 10 seconds

	//query=fmt.Sprintf("?query=%s&start=%d&end=%d&step=%d",query, currentTime,startTime,stepSize)
	q.Add("start", strconv.FormatInt(start, 10))
	q.Add("end", strconv.FormatInt(end, 10))

	request.URL.RawQuery = q.Encode() // Encode and assign back to the original query.
	resp, err := dc.Client.Do(request)
	if err != nil {
		dc.LoggingClient.Errorf("error fetching data from data provider: error: %v", err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		dc.LoggingClient.Errorf("http status not 200 while fetching data: http Status: %d", resp.StatusCode)
		return nil, errors.New(fmt.Sprintf("failed to fetch data, http status: %s", resp.Status))
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&timeSeriesResponse)
	if err != nil || timeSeriesResponse.Status == "error" {
		dc.LoggingClient.Errorf("error decoding the body error: %v", err)
		return nil, err
	}

	valueStr := ""

	// Each row of data will contain multiple timeseries data across all metrices for a given timestamp
	// We are flattening the victoria data such that for each timestamp ( 1 row), we have all the metric values
	// #Timestamps = chunkSize, #datappints per timestamp = len(timeSeriesResponse.Data.Result)

	rawDataGroupedByTime := make(map[int64][]*data.TSDataElement)
	for _, sample := range timeSeriesResponse.Data.Result {
		// Need to group by timestamp in here

		//__name__:"CallDrops"
		//device:"Telco-Tower-A"
		//devicelabels:"Telco-Tower-A,Telcommunication Tower Call Status and temperature"
		//edgeNode:"raspi-pune.bmc.com"
		//profile:"Telco-Tower"
		//site:"Mojave"
		//valuetype:"Uint64"

		metricName := sample.Metric["__name__"].(string)
		var deviceName string
		if sample.Metric["deviceName"] != nil && sample.Metric["deviceName"] != "" {
			deviceName = sample.Metric["deviceName"].(string)
		} else {
			// data issue, not possible
			deviceName = "Unknown"
		}

		//
		// Go over the labels in each group and only pull the configured ones from the Victoria data group
		//

		//nodeName := sample.Metric["edgeNode"].(string)
		profile := ""
		if sample.Metric["profileName"] != nil {
			profile = sample.Metric["profileName"].(string) // Comma delimited labels
		}
		//Add the metaData from the data based on matching metaData from training Config
		metaData := make(map[string]string)
		groupByMeta := make(map[string]string)
		for metaKey, metaValue := range sample.Metric {
			// If metaKey is in Features list, include in here
			FeatureKey := helpers.BuildFeatureName(profile, metaKey)
			if _, ok := dc.PreProcessor.GetFeaturesToIndex()[FeatureKey]; ok {
				metaData[metaKey] = metaValue.(string)
			}
			// Now about the groupBy
			if Contains(dc.PreProcessor.GetMLModelConfig().MLDataSourceConfig.GroupOrJoinKeys, metaKey) {
				groupByMeta[metaKey] = metaValue.(string)
			} else if metaKey == "device" && Contains(dc.PreProcessor.GetMLModelConfig().MLDataSourceConfig.GroupOrJoinKeys, "deviceName") {
				// Special handling for device since this is called as device in victoria while deviceName at other places
				// need to fix for uniformity, this is temporary
				groupByMeta["deviceName"] = metaValue.(string)
			}
		}

		for k, value := range sample.Values {
			valueStrArray := value.([]interface{})
			timeStamp := int64(valueStrArray[0].(float64))
			valueStr = valueStrArray[1].(string)
			// Somehow we get more than the configured size from victoria
			if k < dc.ChunkSize {
				// Builds tsdata
				//tc.preProcessor.TakeSample()
				value, err := strconv.ParseFloat(valueStr, 64)
				if err == nil {
					tsData := data.TSDataElement{
						Timestamp:       timeStamp,
						Profile:         profile,
						MetricName:      metricName,
						Value:           value,
						MetaData:        metaData,
						GroupByMetaData: groupByMeta,
						DeviceName:      deviceName,
					}

					if _, ok := rawDataGroupedByTime[timeStamp]; !ok {
						rawDataGroupedByTime[timeStamp] = make([]*data.TSDataElement, 0)
					}
					tsDataElements := rawDataGroupedByTime[timeStamp]
					rawDataGroupedByTime[timeStamp] = append(tsDataElements, &tsData)
				} else {
					dc.LoggingClient.Errorf("Error in converting the downloaded data to float for metricName = %s", metricName)
				}
				//Output should be for each row, an array of tsDataElements
			}
		}
	}
	rawData := make([][]*data.TSDataElement, 0, len(rawDataGroupedByTime))

	for _, value := range rawDataGroupedByTime {
		rawData = append(rawData, value)
	}
	return rawData, nil
}

func (dc *DataCollector) GetMlStorage() helpers.MLStorageInterface {
	return dc.MlStorage
}

func (dc *DataCollector) SetPreProcessor(preProcessor helpers.PreProcessorInterface) {
	dc.PreProcessor = preProcessor
}

func (dc *DataCollector) GetPreProcessor() helpers.PreProcessorInterface {
	return dc.PreProcessor
}

func (dc *DataCollector) SetFetchStatusManager(statusFileName string) {
	dc.FetchStatusManager = NewFetchStatusManager(statusFileName)
}

func (dc *DataCollector) GetFetchStatusManager() FetchStatusManagerInterface {
	return dc.FetchStatusManager
}

// Builds query string without the timestamp and without base URL
func BuildQuery(mlModelConfig *config.MLModelConfig) string {

	profiles := make([]string, 0)
	metricNames := make([]string, 0)

	for profile, features := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
		if !Contains(profiles, profile) {
			profiles = append(profiles, profile)
		}
		for _, feature := range features {
			//  METRIC temp only for backward compatibility
			if (feature.Type == "METRIC" || feature.Type == "NUMERIC") && !feature.FromExternalDataSource {
				if !Contains(metricNames, feature.Name) {
					metricNames = append(metricNames, feature.Name)
				}
			}
		}

		// Can't put any criteria for METADATA since we always get all of that from Victoria
	}
	// Build the query until here
	query := "{"
	query += " __name__=~\"" + strings.Join(metricNames, "|") + "\""

	// Now build filter criteria
	if mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.Filters != nil {
		var operand string
		var value string
		for _, filterDef := range mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.Filters {
			// leftOpe, operand, right value
			// Special handling for labels
			// Profile Name part of the filter not handled as yet, so filter will fail when dealing with same labels

			value = filterDef.Value

			switch filterDef.Operator {
			case "CONTAINS":
				operand = "=~"
				value = fmt.Sprintf(".*%s.*", value)
				value = strings.Replace(value, ",", ".*|.*", -1) // matches values that contain any of these substrates as substrings.
			case "EQUALS":
				operand = "=~"
				values := strings.Split(value, ",")
				regexPattern := strings.Join(values, "|")
				value = fmt.Sprintf("^(%s)$", regexPattern) // matches exactly and only the values
			case "EXCLUDES":
				operand = "!~"
				value = fmt.Sprintf(".*%s.*", value)
				value = strings.Replace(value, ",", ".*|.*", -1) // matches values that contain any of these substrates as substrings.
			}
			query += fmt.Sprintf(", %s %s\"%s\"", filterDef.Label, operand, value)
		}
	}

	query += "}"
	return query
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Max returns the larger of x or y.
func intMax(x, y int64) int64 {
	if x < y {
		return y
	}
	return x
}
