/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package helpers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/data"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
)

type PreProcessorInterface interface {
	GetPreProcessor(
		lc logger.LoggingClient,
		mlModelConfig *config.MLModelConfig,
		mlAlgoDefinition *config.MLAlgorithmDefinition,
		isTrainingContext bool,
	) PreProcessorInterface

	TakeSample(tsDataElements []*data.TSDataElement) (bool, []*data.TSDataElement)

	GroupSamples(sampledData []*data.TSDataElement) map[string][]*data.TSDataElement
	TransformToFeature(
		groupName string,
		sampledData []*data.TSDataElement,
	) (map[int64][]interface{}, bool)

	GetMLModelConfig() *config.MLModelConfig

	GetMLAlgorithmDefinition() *config.MLAlgorithmDefinition

	GetFeaturesToIndex() map[string]int

	AccumulateMultipleTimeseriesFeaturesSets(
		releaseCandidates map[string][]data.InferenceData,
	) (bool, map[string]data.InferenceData)

	GetOutputFeaturesToValues(sampledData []*data.TSDataElement) map[string]interface{}

	GetOutputFeaturesToIndex() map[string]int
}

var PreProcessorInterfaceImpl PreProcessorInterface

type PreProcessor struct {
	MLModelConfig                 *config.MLModelConfig
	MLAlgoDefinition              *config.MLAlgorithmDefinition
	lastSamplingStartTime         int64
	FeaturesToIndex               map[string]int
	sampledData                   sync.Map
	sampledTimeseriesFeaturesSets map[string]data.InferenceData
	mutex                         sync.Mutex // To be used to ensure that the sample collection
	// Multiple flattened samples grouped by device or metadata
	logger                    logger.LoggingClient
	deviceCountForSameProfile map[string]int
	SamplingIntervalSecs      int64
	IsTrainingContext         bool
	PreviousFeaturesByGroup   map[string][]interface{}
	OutputFeaturesToIndex     map[string]int
}

const NotInitializedValue = -9999

func NewPreProcessor(
	lc logger.LoggingClient,
	mlModelConfig *config.MLModelConfig,
	mlAlgoDefinition *config.MLAlgorithmDefinition,
	isTrainingContext bool,
) PreProcessorInterface {
	// For unit test case writing, we override the PreProcessorInterfaceImpl
	if PreProcessorInterfaceImpl == nil {
		PreProcessorInterfaceImpl = &PreProcessor{}
	}
	// Call implements preprocessor

	return PreProcessorInterfaceImpl.GetPreProcessor(
		lc,
		mlModelConfig,
		mlAlgoDefinition,
		isTrainingContext,
	)
}

func (p *PreProcessor) GetPreProcessor(
	lc logger.LoggingClient,
	mlModelConfig *config.MLModelConfig,
	mlAlgoDefinition *config.MLAlgorithmDefinition,
	isTrainingContext bool,
) PreProcessorInterface {
	preProcessor := &PreProcessor{
		MLModelConfig:     mlModelConfig,
		MLAlgoDefinition:  mlAlgoDefinition,
		IsTrainingContext: isTrainingContext,
	}
	if isTrainingContext {
		preProcessor.SamplingIntervalSecs = mlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.SamplingIntervalSecs
	} else {
		preProcessor.SamplingIntervalSecs = mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.SamplingIntervalSecs
	}

	// Previously this was only supported for sliding window configurations
	preProcessor.PreviousFeaturesByGroup = make(map[string][]interface{})

	// Validate training data configuration
	if len(mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys) == 0 {
		if len(mlModelConfig.MLDataSourceConfig.FeaturesByProfile) == 1 {
			mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys = make([]string, 1)
			mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys[0] = "deviceName"
		} else {
			lc.Errorf("Training Data configuration is incorrect for : %s, need to provide grouping metadata when multiple profiles", mlModelConfig.Name)
			return nil
		}
	}
	if mlAlgoDefinition.Type == TIMESERIES_ALGO_TYPE {
		preProcessor.OutputFeaturesToIndex = preProcessor.generateOutputFeatureMapping()
	}

	// 1-hot encoding is now done in python code
	preProcessor.FeaturesToIndex = mlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex
	var externalFeatures []string
	for profile, features := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
		for _, feature := range features {
			if feature.FromExternalDataSource {
				externalFeatures = append(externalFeatures, BuildFeatureName(profile, feature.Name))
			}
		}
	}
	preProcessor.sampledTimeseriesFeaturesSets = make(map[string]data.InferenceData)
	preProcessor.lastSamplingStartTime = time.Now().Unix()
	preProcessor.logger = lc
	return preProcessor
}

func (p *PreProcessor) generateOutputFeatureMapping() map[string]int {
	mlModelConfig := p.MLModelConfig
	outputFeaturesToIndex := make(map[string]int)
	currentIndex := 0
	// Add Timestamp and GroupOrJoinKeys first
	outputFeaturesToIndex["Timestamp"] = currentIndex
	currentIndex++
	for _, key := range mlModelConfig.MLDataSourceConfig.GroupOrJoinKeys {
		outputFeaturesToIndex[key] = currentIndex
		currentIndex++
	}
	// Add isOutput == true features from featuresByProfile
	for profileName, features := range mlModelConfig.MLDataSourceConfig.FeaturesByProfile {
		for _, feature := range features {
			if feature.IsOutput {
				featureFullName := fmt.Sprintf("%s#%s", profileName, feature.Name)
				outputFeaturesToIndex[featureFullName] = currentIndex
				currentIndex++
			}
		}
	}
	return outputFeaturesToIndex
}

func (p *PreProcessor) GetOutputFeaturesToIndex() map[string]int {
	return p.OutputFeaturesToIndex
}

func (p *PreProcessor) GetMLModelConfig() *config.MLModelConfig {
	return p.MLModelConfig
}

func (p *PreProcessor) GetMLAlgorithmDefinition() *config.MLAlgorithmDefinition {
	return p.MLAlgoDefinition
}

func (p *PreProcessor) GetFeaturesToIndex() map[string]int {
	return p.FeaturesToIndex
}

/*func (p *PreProcessor) SetPreviousFeaturesByGroup(previousFeaturesByGroup map[string][]interface{}) {
	p.PreviousFeaturesByGroup = previousFeaturesByGroup
}*/

// Collects the sample for the sampling interval.Assumes that the filtering was done earlier to only
// pass the metrics that is relevant for training configuration
func (p *PreProcessor) TakeSample(
	tsDataElements []*data.TSDataElement,
) (bool, []*data.TSDataElement) {
	// If snapshot collection pending, keep accumulating the metrics for each device

	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.addTSDataElement(tsDataElements)
	currentTimeSecs := time.Now().Unix()
	dataCollectionGapInSecs := currentTimeSecs - p.lastSamplingStartTime

	// SamplingInterval to be taken from the context ie whether it is training or prediction
	if dataCollectionGapInSecs >= p.SamplingIntervalSecs {
		var tsSamples = make([]*data.TSDataElement, 0)
		p.sampledData.Range(func(key, element any) bool {
			if tsDataElement, ok := element.(*data.TSDataElement); ok {
				tsSamples = append(tsSamples, tsDataElement)
			}
			return true // continue iterating
		})

		p.lastSamplingStartTime = currentTimeSecs
		p.logger.Tracef("Draining the sample: %v", tsSamples)
		p.sampledData = sync.Map{}
		return true, tsSamples
	}
	return false, nil
}

func (p *PreProcessor) addTSDataElement(tsDataElements []*data.TSDataElement) {
	for _, tsDataElement := range tsDataElements {
		// Need to allow multiple samples for same dataElement
		var key string
		if p.MLModelConfig.MLDataSourceConfig.SupportSlidingWindow {
			key = fmt.Sprintf(
				"%s_%s_%d",
				tsDataElement.DeviceName,
				tsDataElement.MetricName,
				tsDataElement.Timestamp,
			)
		} else {
			// Optimize storage if one sample per sampling interval
			key = fmt.Sprintf("%s_%s", tsDataElement.DeviceName, tsDataElement.MetricName)
		}
		p.sampledData.Store(key, tsDataElement)
	}
}

// Group by device name or metadata, the given sample during one sampling interval
func (p *PreProcessor) GroupSamples(
	sampledData []*data.TSDataElement,
) map[string][]*data.TSDataElement {

	samplesByGroupName := make(map[string][]*data.TSDataElement, 0)

	for _, tsSampleData := range sampledData {
		if tsSampleData == nil {
			continue
		}

		// Get groupBy keys from config
		var dataOk = false
		var groupingKeyParts []string
		for _, groupByKey := range p.MLModelConfig.MLDataSourceConfig.GroupOrJoinKeys {
			dataOk = true
			if grpVal, ok := tsSampleData.GroupByMetaData[groupByKey]; ok {
				sanitizedGrpVal := strings.ReplaceAll(grpVal, " ", "_")
				groupingKeyParts = append(groupingKeyParts, sanitizedGrpVal)
			} else {
				dataOk = false
				break
			}
		}
		groupingKey := strings.Join(groupingKeyParts, "_")
		if !dataOk {
			// Ignore this sample and continue since this one didn't have the grouping data
			continue
		}

		if samplesByGroupName[groupingKey] == nil {
			samplesByGroupName[groupingKey] = make([]*data.TSDataElement, 0)
		}
		samplesByGroupName[groupingKey] = append(samplesByGroupName[groupingKey], tsSampleData)
	}
	return samplesByGroupName
}

// For now, the TransformToFeature creates a row of feature from multiple rows on time-series data,
// each of which is just for one metric only.
// We pick the latest metric value if there are many
// Returns a map of timestamp to completeFeature & whether the feature set is complete
// map of ts is added in here for batch import scenario when all the data gets ingested in bulk and when creating events we need actual timestamp
func (p *PreProcessor) TransformToFeature(
	groupName string,
	sampledData []*data.TSDataElement,
) (map[int64][]interface{}, bool) {

	// Adding support for sliding window which means that we pick the latest value at that point in time (snapshot)
	// What we do for training applies to inferencing as well

	featuresLength := len(p.FeaturesToIndex)
	if !p.IsTrainingContext {
		// for prediction flow - exclude all external features
		featuresLength = len(p.FeaturesToIndex) - len(p.MLModelConfig.MLDataSourceConfig.ExternalFeatures)
	}

	currentFeatureVals := make([]interface{}, featuresLength)
	// Initialize the feature to some un-initialized value or to the previous data
	for i := range currentFeatureVals {
		currentFeatureVals[i] = NotInitializedValue
	}

	// Fill the external feature value with empty string because the actual value will be provided by customer
	if p.IsTrainingContext {
		p.setEmptyStringValueForExternalFeatures(currentFeatureVals)
	}

	if _, ok := p.FeaturesToIndex[config.TIMESTAMP_FEATURE_COL_NAME]; ok {
		currentFeatureVals[0] = int64(NotInitializedValue)
	}
	if p.MLAlgoDefinition.GroupByAttributesRequired {
		for _, groupKey := range p.MLModelConfig.MLDataSourceConfig.GroupOrJoinKeys {
			if _, ok := p.FeaturesToIndex[groupKey]; ok {
				currentFeatureVals[p.FeaturesToIndex[groupKey]] = ""
			}
		}
	}

	var keyFound bool
	var index int

	// sampledData can contain multiple feature values in the sample duration,
	// Depending on useSlidingWindow, we pick the last one or all the feature sets can be built in the sampling interval
	featureSetByTime := make(map[int64][]interface{})
	ts := int64(0)
	for i, tsData := range sampledData {
		p.logger.Debugf("i: %d, tsData: %v", i, tsData)

		featureLookupKey := BuildFeatureName(tsData.Profile, tsData.MetricName)
		index, keyFound = p.FeaturesToIndex[featureLookupKey]
		if !keyFound {
			continue
		}

		if currentFeatureVals[index] == NotInitializedValue {
			ts = tsData.Timestamp
		}
		currentFeatureVals[index] = tsData.Value

		if _, ok := p.FeaturesToIndex[config.TIMESTAMP_FEATURE_COL_NAME]; ok &&
			currentFeatureVals[0].(int64) < tsData.Timestamp {
			// Keep the latest timestamp
			currentFeatureVals[0] = tsData.Timestamp
		}

		for groupKey, val := range tsData.GroupByMetaData {
			if _, ok := p.FeaturesToIndex[groupKey]; ok {
				currentFeatureVals[p.FeaturesToIndex[groupKey]] = val
			}
		}
		// Lookout to match the metaData feature

		if tsData.MetaData != nil {
			for metaDataName, metaVal := range tsData.MetaData {
				featureLookupKey = BuildFeatureName(tsData.Profile, metaDataName)
				index, keyFound = p.FeaturesToIndex[featureLookupKey]
				if keyFound && currentFeatureVals[index] == NotInitializedValue {
					currentFeatureVals[index] = metaVal
				}
			}
		}
		isCurrentRowFull := true
		for idx, val := range currentFeatureVals {
			if p.MLAlgoDefinition.GroupByAttributesRequired {
				// make sure it's not isExternal feature (Classification)
				if val == "" && !(idx == (len(currentFeatureVals) - 1)) {
					isCurrentRowFull = false
					break
				}
			}
			if val == NotInitializedValue {
				isCurrentRowFull = false
				break
			}
		}
		if isCurrentRowFull {
			featureSetByTime[ts] = currentFeatureVals
			currentFeatureVals = make([]interface{}, featuresLength)
			for i := range currentFeatureVals {
				currentFeatureVals[i] = NotInitializedValue
			}
			// Below repeat to typecast to int64
			if _, ok := p.FeaturesToIndex[config.TIMESTAMP_FEATURE_COL_NAME]; ok {
				currentFeatureVals[0] = int64(NotInitializedValue)
			}
			for _, groupKey := range p.MLModelConfig.MLDataSourceConfig.GroupOrJoinKeys {
				if _, ok := p.FeaturesToIndex[groupKey]; ok {
					currentFeatureVals[p.FeaturesToIndex[groupKey]] = ""
				}
			}
		}
	}

	featuresComplete := false
	if len(featureSetByTime) > 0 {
		featuresComplete = true
		p.logger.Debugf("complete feature set by time: %#v", featureSetByTime)
	} else if previousFeatureValues, found := p.PreviousFeaturesByGroup[groupName]; found {
		// Check and take from previous sampling interval
		for i := range currentFeatureVals {
			featuresComplete = true
			if currentFeatureVals[i] == NotInitializedValue {
				if previousFeatureValues[i] != NotInitializedValue {
					currentFeatureVals[i] = previousFeatureValues[i]
				} else {
					featuresComplete = false
				}
			}
		}
		if featuresComplete {
			featureSetByTime[ts] = currentFeatureVals
		}
	}

	// Now set the previousValues
	if p.MLModelConfig.MLDataSourceConfig.SupportSlidingWindow {
		p.PreviousFeaturesByGroup[groupName] = currentFeatureVals
	}

	return featureSetByTime, featuresComplete
}

// TODO: map[string][]data.InferenceData
func (p *PreProcessor) AccumulateMultipleTimeseriesFeaturesSets(
	releaseCandidates map[string][]data.InferenceData,
) (bool, map[string]data.InferenceData) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// accumulate data
	for groupKey, featuresDataArr := range releaseCandidates {
		featuresData := featuresDataArr[0]
		group := strings.Split(groupKey, "__")[0]
		if existingData, exists := p.sampledTimeseriesFeaturesSets[group]; exists {
			// append new feature set/s to the existing entry
			existingData.Data = append(existingData.Data, featuresData.Data...)
			p.sampledTimeseriesFeaturesSets[group] = existingData
		} else {
			// initialize a new entry in sampledTimeseriesFeaturesSets
			p.sampledTimeseriesFeaturesSets[group] = data.InferenceData{
				GroupName: featuresData.GroupName,
				Devices:   featuresData.Devices,
				Data:      featuresData.Data,
				Tags:      featuresData.Tags,
				TimeStamp: featuresData.TimeStamp,
			}
		}
	}

	// check if all accumulated features have reached the required count
	for _, accumulatedFeaturesSets := range p.sampledTimeseriesFeaturesSets {
		if len(
			accumulatedFeaturesSets.Data,
		) < p.MLModelConfig.MLDataSourceConfig.InputContextCount {
			// if any entry has less data than required, keep accumulating
			return false, nil
		}
	}

	accumulatedData := p.sampledTimeseriesFeaturesSets
	// reset sampled data
	p.logger.Tracef(
		"Draining the sample for timeseries prediction: %v",
		p.sampledTimeseriesFeaturesSets,
	)
	p.sampledTimeseriesFeaturesSets = make(map[string]data.InferenceData)

	return true, accumulatedData
}

func (p *PreProcessor) GetOutputFeaturesToValues(
	sampledData []*data.TSDataElement,
) map[string]interface{} {
	output := make(map[string]interface{})

	p.logger.Debugf("MLDataSourceConfig: %+v", p.MLModelConfig.MLDataSourceConfig)
	for _, d := range sampledData {
		feat := p.MLModelConfig.MLDataSourceConfig.FeaturesByProfile[d.Profile]
		for _, f := range feat {
			if f.IsOutput && f.Name == d.MetricName {
				output[d.MetricName] = d.Value
			}
		}
	}
	return output
}

func (p *PreProcessor) setEmptyStringValueForExternalFeatures(features []interface{}) {
	for _, externalFeature := range p.MLModelConfig.MLDataSourceConfig.ExternalFeatures {
		featureIndex := p.FeaturesToIndex["external#"+externalFeature.Name]
		features[featureIndex] = ""
	}
}
