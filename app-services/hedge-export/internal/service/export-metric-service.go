/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"hedge/app-services/hedge-export/internal/db/redis"
	"hedge/app-services/hedge-export/internal/dto"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MetricExportService struct {
	edgexSdk  interfaces.ApplicationService
	appconfig *dto.AppConfig
	DbClient  *redis.DBClient
	Quit      chan bool
}

var client http.Client

func NewMetricExportService(service interfaces.ApplicationService, appconfig *dto.AppConfig) *MetricExportService {
	exportService := new(MetricExportService)
	exportService.edgexSdk = service
	exportService.appconfig = appconfig
	exportService.Quit = make(chan bool)
	return exportService
}

func (exportService *MetricExportService) SendCurrentData(metricConfig dto.MetricExportConfig, startTime, endTime int64) error {
	exportService.edgexSdk.LoggingClient().Info("Export Metric Configuration: ", metricConfig)

	trainingData, err := exportService.getData(metricConfig, startTime, endTime)
	if err != nil || trainingData.Status != "success" {
		return err
	}
	exportService.edgexSdk.LoggingClient().Info("Exporting data: ", trainingData)

	// transform to ADE metric structure and send for training
	adeMetrics := exportService.transformToADEMetricInput(trainingData)
	exportService.edgexSdk.LoggingClient().Info("ADE Metrics ", trainingData)
	if len(adeMetrics) > 0 {
		exportService.edgexSdk.LoggingClient().Info("ADE Payload being sent")
		// Send to ADE AIF ingestion service
		return exportService.publishToMLServiceProvider(adeMetrics)
	} else {
		exportService.edgexSdk.LoggingClient().Info("No Export data found, check if it is being generated or not")
		return nil
	}

}

func (exportService *MetricExportService) getData(metricConfig dto.MetricExportConfig, startTime, endTime int64) (*dto.TimeSeriesResponse, error) {
	iotUrl := exportService.appconfig.VictoriaMetricsURL + "/api/v1/query_range"
	request, err := http.NewRequest("GET", iotUrl, nil)
	request.Header.Set("Content-type", "application/json")
	if err != nil {
		return nil, fmt.Errorf("Failed to create request %v", err)
	}

	var query string
	q := request.URL.Query()
	// Get a copy of the query values.
	if metricConfig.IsAllDevices {
		query = fmt.Sprintf("%s", metricConfig.Metric)
	} else {
		devices := "device=~\"" + strings.Join(metricConfig.Devices, "|") + "\""
		query = fmt.Sprintf("%s{%s}", metricConfig.Metric, devices)
	}

	q.Add("start", strconv.FormatInt(startTime, 10))
	q.Add("end", strconv.FormatInt(endTime, 10))
	q.Add("query", query)             // Add a new value to the set.
	request.URL.RawQuery = q.Encode() // Encode and assign back to the original query.
	fmt.Println(request.URL)
	resp, err := client.Do(request)

	if err != nil {
		return nil, fmt.Errorf("Failed to get metric data from DB %v", err)
	}
	defer resp.Body.Close()
	var timeSeriesResponse dto.TimeSeriesResponse
	err = json.NewDecoder(resp.Body).Decode(&timeSeriesResponse)
	if err != nil {
		return nil, fmt.Errorf("Failed read metric data body %v", err)
	}
	fmt.Println(timeSeriesResponse.Status)
	return &timeSeriesResponse, nil
}

func (exportService *MetricExportService) transformToADEMetricInput(trainingData *dto.TimeSeriesResponse) []dto.AdeMetric {
	adeMetrics := make([]dto.AdeMetric, len(trainingData.Data.Result))

	for i, metricData := range trainingData.Data.Result {
		metricName := metricData.Metric["__name__"].(string)
		hostName := metricData.Metric["edgeNode"].(string)
		isKPI := false

		entityTypeId := metricData.Metric["profile"].(string)
		entityName := metricData.Metric["device"].(string)
		if strings.Contains(entityName, " ") {
			entityName = strings.Replace(entityName, " ", "-", -1)
		}

		if metricName == "CallDrops" || metricName == "TurbinePower" {
			isKPI = true
			/*	if metricName == "CallDrops" && entityName != "Telco-Tower-A" {
				continue
			}*/
		}

		source := metricData.Metric["edgeNode"].(string) // From config..
		entityId := hostName + ":" + entityTypeId + ":" + entityName

		labels := dto.Labels{
			MetricName:   metricName,
			Hostname:     metricData.Metric["edgeNode"].(string),
			EntityTypeId: entityTypeId,
			EntityName:   entityName,
			EntityId:     entityId,
			HostType:     "hedge-IoT",
			IsKPI:        isKPI,
			Unit:         metricData.Metric["valuetype"].(string),
			//Unit:         "Uint64",
			Source: source,
		}

		samples := make([]dto.Sample, len(metricData.Values))

		for index, data := range metricData.Values {
			sample := dto.Sample{}
			for i := 0; i < 2; i++ {
				value := data.([]interface{})[i]
				switch val := value.(type) {
				case float64:
					//fmt.Println("This is a float64 and time")
					sample.Timestamp = int64(val * 1000)
				case string:
					metricValue, err := strconv.ParseFloat(string(val), 64)
					if err == nil {
						sample.Value = metricValue
					}
				}
			}
			samples[index] = sample
		}

		metric := dto.AdeMetric{
			Labels:  labels,
			Samples: samples,
		}
		adeMetrics[i] = metric
	}
	return adeMetrics
}

func (exportService *MetricExportService) publishToMLServiceProvider(adeMetric []dto.AdeMetric) error {
	payload := new(bytes.Buffer)
	json.NewEncoder(payload).Encode(adeMetric)
	dat, _ := json.Marshal(adeMetric)
	fmt.Println(string(dat))

	request, err := http.NewRequest("POST", exportService.appconfig.HelixExportUrl, payload)
	request.Header.Set("Content-type", "application/json")
	// Need to read the below from secrets, commenting out this since this code is not used, this will be restored when we
	// do down-sampling of the data before exporting to BHOM
	//request.Header.Set("Authorization", exportService.appconfig.HelixSecretKey)
	if err != nil {
		return fmt.Errorf("Failed to create post request %v", err)
	}

	resp, err := client.Do(request)

	if err != nil {
		return fmt.Errorf("Failed to write metric data %v", err)
	}
	fmt.Printf("Response from posting data to ADE, %v\n", resp)
	return nil
}

func (exportService *MetricExportService) ExportMetricData(dbClient *redis.DBClient) {
	var start, end int64
	var duration time.Duration
	conn := dbClient.Pool.Get()
	var inter = ""
	defer conn.Close()
	fmt.Println("Export Metrics Details process started")
	interval, err := dbClient.GetIntervalExportData()
	if interval != nil {
		inter = fmt.Sprintf("%d", int(interval.(float64)))
	}
	if err != nil {
		dbClient.SetIntervalExportData(exportService.appconfig.ExportBatchFrequency)
	}
	if inter != "" {
		duration, err = time.ParseDuration(inter + "m")
		if err != nil {
			exportService.edgexSdk.LoggingClient().Error("Error parsing Export Batch Frequency")
		}
	} else {
		duration, err = time.ParseDuration(exportService.appconfig.ExportBatchFrequency + "m")
		if err != nil {
			exportService.edgexSdk.LoggingClient().Error("Error parsing Export Batch Frequency")
		}
	}
	end = time.Now().Unix()
	for {
		select {
		case q := <-exportService.Quit:
			if q {
			}
			exportService.edgexSdk.LoggingClient().Info("Exiting ExportMetricData")
			return
		case <-time.After(duration):
			start = end
			end = time.Now().Unix()
			metricConfigs, err := dbClient.GetAllMetricExportData()
			if err != nil {
				exportService.edgexSdk.LoggingClient().Warn(err.Error())
			}
			for _, config := range metricConfigs {
				err = exportService.SendCurrentData(config, start, end)
				if err != nil {
					exportService.edgexSdk.LoggingClient().Warn(err.Error())
				}
			}
		}
	}
}

func (exportService *MetricExportService) ResetExportData() {
	exportService.Quit <- true
	go exportService.ExportMetricData(exportService.DbClient)
	exportService.edgexSdk.LoggingClient().Error("ExportData restarted")
	return
}
