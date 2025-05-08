/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package broker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"
	"text/template"
	"time"

	mqttConfig "hedge/common/config"
	"hedge/common/db"
	"hedge/common/utils"
	"hedge/edge-ml-service/internal/training"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/data"
	"hedge/edge-ml-service/pkg/helpers"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"

	"hedge/common/client"
	"hedge/common/models"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
)

type MLBrokerInferencing struct {
	preprocessor                   helpers.PreProcessorInterface
	featureCollectionByGroup       map[string][]data.InferenceData
	Client                         client.HTTPClient
	httpSender                     *transforms.HTTPSender
	mqttSender                     training.MqttSender
	eventPipelineTriggerUrl        string
	mlEventConfigs                 []config.MLEventConfig
	mutex                          sync.Mutex
	lc                             logger.LoggingClient
	ReadMessageBus                 string
	eventStatByGroup               map[string]EventStats // so we need not create duplicates & we can close when applicable
	predictionInputPayloadTemplate *template.Template
	notificationChannel            <-chan string
	mlModelConfigName              string
	isRealTime                     bool
}

type EventStats struct {
	Open           bool
	LastSeverity   string
	PredLoss       map[string]interface{}
	RelatedMetrics []string
	EventName      string
	EventMessage   string
	Thresholds     map[string]interface{}
}

const EVENT_PUB_ERR_MSG = "failed to trigger event-publisher for the anomaly event"

var mqttSender training.MqttSender

type Data struct {
	Data [][]interface{} `json:"data"`
}

type InputWrapper struct {
	CollectedData   map[string]data.InferenceData
	HyperParameters map[string]interface{}
}

func NewMLBrokerInference(
	mlModelConfig *config.MLModelConfig,
	mlAlgoDefinition *config.MLAlgorithmDefinition,
	mlBrokerConfig *MLBrokerConfig,
	service interfaces.ApplicationService,
	notificationChannel <-chan string,
	isRealTime bool,
) *MLBrokerInferencing {

	mlInf := new(MLBrokerInferencing)
	mlInf.preprocessor = helpers.NewPreProcessor(service.LoggingClient(), mlModelConfig, mlAlgoDefinition, false)
	if mlInf.preprocessor == nil {
		// Mostly there was no way to group the features
		return nil
	}
	mlInf.Client = &http.Client{}
	mlInf.featureCollectionByGroup = make(map[string][]data.InferenceData)
	mlInf.ReadMessageBus = mlBrokerConfig.ReadMessageBus
	mlInf.notificationChannel = notificationChannel
	mlInf.mlModelConfigName = mlModelConfig.Name
	mlInf.isRealTime = isRealTime

	//Fetch anomaly event config based on training trainingData and ModelVersion
	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(service)
	dbClient := redis.NewDBClient(dbConfig)
	// Temp so our code doesn't bomb in dev env
	if mlModelConfig.ModelConfigVersion == 0 {
		mlModelConfig.ModelConfigVersion = 1
	}
	mlInf.lc = service.LoggingClient()
	eventConfigs, err := dbClient.GetAllMLEventConfigsByConfig(mlModelConfig.MLAlgorithm, mlModelConfig.Name)
	if err != nil {
		service.LoggingClient().Errorf("Error reading MLEventConfigs: %v", err)
		return nil
	}
	service.LoggingClient().Infof("MLEventConfigs for model %s: %v", mlModelConfig.Name, eventConfigs)

	var helperError error
	mlInf.predictionInputPayloadTemplate, helperError = helpers.CreateTemplate(mlAlgoDefinition.PredictionPayloadTemplate.Template, mlAlgoDefinition.Type)
	if helperError != nil {
		service.LoggingClient().Errorf("Error creating prediction input payload template: %v", helperError)
		return nil
	}
	mlInf.mlEventConfigs = eventConfigs
	mlInf.eventStatByGroup = make(map[string]EventStats)

	if mqttSender == nil {
		publishMLPredictionTopic, err := service.GetAppSetting("PublishMLPredictionTopic")
		if err != nil {
			service.LoggingClient().Errorf("Error Building MQTT client: Error: %v", err)
		}
		config, err := mqttConfig.BuildMQTTSecretConfig(service, publishMLPredictionTopic, client.HedgeMLBrokerServiceKey)
		if err != nil {
			service.LoggingClient().Errorf("Error Building MQTT client: Error: %v", err)
		} else {

			MQTTSender := transforms.NewMQTTSecretSender(config, mqttConfig.GetPersistOnError(service))
			mlInf.mqttSender = MQTTSender
		}
	} else {
		mlInf.mqttSender = mqttSender //for testing purpose only
	}

	return mlInf
}

func (inf *MLBrokerInferencing) FilterByFeatureNames(edgexcontext interfaces.AppFunctionContext, evntData interface{}) (continuePipeline bool, result interface{}) {
	if evntData == nil {
		return false, errors.New("no Event Received")
	}

	event, ok := evntData.(dtos.Event)
	if !ok {
		edgexcontext.LoggingClient().Info("Skipping event processing in FilterByFeatureNames")
		return false, nil
	}

	for profile, features := range inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeaturesByProfile {
		if event.ProfileName != profile {
			continue
		}
		// filter out only the readings that are applicable for the model configuration

		featureReadings := make([]dtos.BaseReading, 0)
		for _, reading := range event.Readings {
			for _, f := range features {
				if f.Type == "METRIC" {
					if reading.ResourceName == f.Name {
						featureReadings = append(featureReadings, reading)
						break
					}
				}
			}
		}
		event.Readings = featureReadings

		edgexcontext.LoggingClient().Debugf("matching event->readings: %v", event)
		return true, event
	}
	return false, nil
}

func (inf *MLBrokerInferencing) TakeSample(edgexcontext interfaces.AppFunctionContext, evntData interface{}) (continuePipeline bool, result interface{}) {

	edgexcontext.LoggingClient().Debug("Taking a sample from enriched data stream")

	if evntData == nil {
		return false, errors.New("no event received")
	}
	event, ok := evntData.(dtos.Event)
	if !ok {
		edgexcontext.LoggingClient().Info("Skipping event processing in TakeSample")
		return true, evntData
	}
	tags := event.Tags
	// Convert com_m to tsData

	// Need to do this for all readings
	accumulatedTSSamples := make([]*data.TSDataElement, len(event.Readings))
	groupByMetaData := make(map[string]string, 0)
	var metaData map[string]string
	if len(inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.GroupOrJoinKeys) == 0 ||
		utils.Contains(inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.GroupOrJoinKeys, "deviceName") {
		groupByMetaData["deviceName"] = event.DeviceName
	}
	// Iterate over all Tags and fill-in against the groupBy metaData and/or metaData
	for metaKey, metaVal := range tags {
		if inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.GroupOrJoinKeys != nil && utils.Contains(inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.GroupOrJoinKeys, metaKey) {
			groupByMetaData[metaKey] = fmt.Sprintf("%v", metaVal)
		}
		for _, f := range inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeaturesByProfile[event.ProfileName] {
			if f.Name == metaKey {
				if metaData == nil {
					metaData = make(map[string]string, 0)
				}
				metaData[metaKey] = fmt.Sprintf("%v", metaVal)
			}
		}
	}

	for i, reading := range event.Readings {

		value, err := getValue(reading.Value, reading.ValueType)
		if err != nil {
			edgexcontext.LoggingClient().Warn(err.Error())
			return false, nil
		}

		tsData := data.TSDataElement{
			Profile:         event.ProfileName,
			DeviceName:      event.DeviceName,
			Timestamp:       reading.Origin / 1000000000,
			MetricName:      reading.ResourceName,
			Value:           value,
			Tags:            tags,
			GroupByMetaData: groupByMetaData,
			MetaData:        metaData,
		}

		// MetaData & GroupByMetaData to be based out of trainingData Config

		accumulatedTSSamples[i] = &tsData
	}

	// Depending on the sampling interval, keep accumulating more samples to return the collected samples for further processing
	return inf.preprocessor.TakeSample(accumulatedTSSamples)
}

func (inf *MLBrokerInferencing) GroupSamples(ctx interfaces.AppFunctionContext, params interface{}) (continuePipeline bool, result interface{}) {
	sampleRawData, ok := params.([]*data.TSDataElement)
	if len(sampleRawData) == 0 || !ok {
		return false, nil
	}
	currentDataGroup := inf.preprocessor.GroupSamples(sampleRawData)
	// Add new data to current dataMap and then decide what to send
	if len(currentDataGroup) == 0 {
		return false, nil
	}
	inf.lc.Debugf("currentDataGroup: %v", currentDataGroup)
	return true, currentDataGroup
}

func (inf *MLBrokerInferencing) ConvertToFeatures(ctx interfaces.AppFunctionContext, params interface{}) (continuePipeline bool, result interface{}) {
	tsDataByGroup, ok := params.(map[string][]*data.TSDataElement)
	if !ok {
		return false, nil
	}
	inf.lc.Debugf("tsDataByGroup: %v", tsDataByGroup)
	inferenceDataRowsByGroup := make(map[string][]data.InferenceData)

	for groupKey, tsData := range tsDataByGroup {
		deviceNameMap := make(map[string]struct{})

		for _, tsElem := range tsData {
			deviceNameMap[tsElem.DeviceName] = struct{}{}
		}
		uniqueDeviceNames := make([]string, 0, len(deviceNameMap))
		for deviceName := range deviceNameMap {
			uniqueDeviceNames = append(uniqueDeviceNames, deviceName)
		}

		outputFeat := inf.preprocessor.GetOutputFeaturesToValues(tsData)
		featureByTime, completeFeaturesFound := inf.preprocessor.TransformToFeature(groupKey, tsData)
		inferenceInputDataRows := make([]data.InferenceData, len(featureByTime))

		if completeFeaturesFound {
			i := 0
			for ts, inferData := range featureByTime {
				// Need to add the timestamp so we use it when generating the event, consider batch import when we still want to keep the event time accurate
				inferenceInputDataRows[i] = data.InferenceData{
					GroupName:  groupKey,
					Devices:    uniqueDeviceNames,
					Data:       [][]interface{}{inferData},
					TimeStamp:  ts,
					OutputData: outputFeat,
					Tags:       tsData[0].Tags,
				}
				i++
				// inferenceDataRows = append(inferenceDataRows, inferenceInputDataWrapper)
			}
			inferenceDataRowsByGroup[groupKey] = inferenceInputDataRows
		}
	}

	if len(inferenceDataRowsByGroup) > 0 {
		inf.lc.Debugf("After ConvertToFeatures: inferenceDataRows: %v", inferenceDataRowsByGroup)
		return true, inferenceDataRowsByGroup
	}
	return false, nil
}

// We already have the data, we just ensure that we have data count per device/group so we satisfy the stabilization interval of ML event config
// Since we can get multiple sets, we built an array of lots each lot representing a set with the lot size needed to infer event
// input: map[string][]data.InferenceData, returns completed lots only as map[grp__lot#__idx]data.InferenceData
// Why grp__lotNo_recordNo as a key is so that we can pass this as-is for prediction as batch
func (inf *MLBrokerInferencing) CollectAndReleaseFeature(ctx interfaces.AppFunctionContext, params interface{}) (continuePipeline bool, result interface{}) {

	// if timeseries prediction, return AccumulateMultipleTimeseriesFeaturesSets
	if inf.preprocessor.GetMLAlgorithmDefinition().Type == helpers.TIMESERIES_ALGO_TYPE {
		return true, params
	}

	currentFeaturesWrapperByGroupName := params.(map[string][]data.InferenceData)

	// Add to collectionList, if the collection threshold exceeds, release it from new collection by removing it
	for group, featureWrapperRow := range currentFeaturesWrapperByGroupName {
		inf.featureCollectionByGroup[group] = append(inf.featureCollectionByGroup[group], featureWrapperRow...)
	}
	stabilizationCount := 1
	if len(inf.mlEventConfigs) > 0 {
		stabilizationCount = inf.mlEventConfigs[0].StabilizationPeriodByCount
	}

	// map[grp__lot#__idx]data.InferenceData
	releaseCandidates := make(map[string]data.InferenceData)
	// releaseCandidates := make(map[string]data.InferenceData)
	deleteCandidateCountByGroup := make(map[string]int)
	for group, featureRows := range inf.featureCollectionByGroup {
		ctx.LoggingClient().Debugf("Group: %v, featureRows: %v", group, featureRows)

		if len(featureRows) >= stabilizationCount {
			// remove and add the remaining back
			numberOfLots := len(featureRows) / stabilizationCount

			for i, featureRow := range featureRows {
				lotNo := i / stabilizationCount
				// if i(no of featureRows)=5, numberOfLots=2, then we need to break when i=5
				if i >= numberOfLots*stabilizationCount {
					break
				}
				correlationId := fmt.Sprintf("%s__%d__%d", group, lotNo, i-lotNo*stabilizationCount)
				releaseCandidates[correlationId] = featureRow
				deleteCandidateCountByGroup[group] = i
			}
		}
	}

	if len(releaseCandidates) > 0 {
		inf.lc.Debugf("release candidates for inferencing: %v", releaseCandidates)
		// remove from collection
		for group, count := range deleteCandidateCountByGroup {
			if len(inf.featureCollectionByGroup[group]) == count+1 {
				inf.featureCollectionByGroup[group] = make([]data.InferenceData, 0)
			} else {
				inf.featureCollectionByGroup[group] = inf.featureCollectionByGroup[group][count:]
			}
		}
		return true, releaseCandidates
	}
	return false, nil
}

type EventBuilderData struct {
	Tags        map[string]bool
	Profile     string
	Predictions map[float64]bool
	Data        [][]interface{}
}

func (inf *MLBrokerInferencing) AccumulateMultipleTimeseriesFeaturesSets(ctx interfaces.AppFunctionContext, params interface{}) (continuePipeline bool, result interface{}) {
	if inf.preprocessor.GetMLAlgorithmDefinition().Type == helpers.TIMESERIES_ALGO_TYPE {
		ctx.LoggingClient().Info("taking accumulated timeseries features sets from data stream")
		// map[string][]data.InferenceData
		// releaseCandidates := params.(map[string]data.InferenceData)
		// map[string][]data.InferenceData
		releaseCandidates := params.(map[string][]data.InferenceData)

		// depending on the TimeseriesFeaturesSetsCount, keep accumulating more samples to return the collected samples for further processing
		return inf.preprocessor.AccumulateMultipleTimeseriesFeaturesSets(releaseCandidates)
	} else {
		return true, params
	}
}

func (inf *MLBrokerInferencing) GetAndPublishPredictions(ctx interfaces.AppFunctionContext, params interface{}) (continuePipeline bool, result interface{}) {
	ctx.LoggingClient().Info("Running Inference for " + ctx.PipelineId())

	if params == nil {
		return false, errors.New("no event received")
	}

	modelConfigNameRequested := ""
	onDemand := !inf.isRealTime
	// Simulation scenario, should move this to beginning so we don't incur processing cost
	if onDemand {
		select {
		case modelConfigNameRequested = <-inf.notificationChannel:
			ctx.LoggingClient().Info("Running inference for " + modelConfigNameRequested)
		default:
			ctx.LoggingClient().Info("Not Running inference")
		}
		if modelConfigNameRequested != inf.mlModelConfigName {
			return false, fmt.Errorf("stopping pipeline: requested ml config name: %s, found: %s",
				modelConfigNameRequested, inf.mlModelConfigName)
		}
	}

	mlAlgoType := inf.preprocessor.GetMLAlgorithmDefinition().Type
	inf.lc.Debugf("mlAlgoType is: %v", mlAlgoType)

	inputData := params.(map[string]data.InferenceData)
	inputWrapper := InputWrapper{inputData, inf.preprocessor.GetMLAlgorithmDefinition().HyperParameters}
	inf.lc.Debugf("inputData: %v", inputData)
	inf.lc.Debugf("hyperParameters: %v", inf.preprocessor.GetMLAlgorithmDefinition().HyperParameters)

	inf.mutex.Lock()
	defer inf.mutex.Unlock()

	// Execute the template with the inputWrapper data
	// The payload contains JSON data, where the structure is dynamically determined by the specific template that was applied.
	var payload bytes.Buffer
	if err := inf.predictionInputPayloadTemplate.Execute(&payload, inputWrapper); err != nil {
		inf.lc.Errorf("Error applying the input payload template for prediction: %v", err)
		return false, err
	}

	// We assume the output is always of type map[string]interface{}
	inferences, err := inf.getPredictions(payload)
	if err != nil {
		inf.lc.Errorf("Error getting inferences: %v", err)
		return false, err
	}

	inf.lc.Infof("Got inferences for model config name %s: %v", inf.mlModelConfigName, inferences)

	if *inf.preprocessor.GetMLAlgorithmDefinition().PublishPredictionsRequired {
		err = inf.buildAndPublishMLPredictions(ctx, inferences, inputData)
		if err != nil {
			return false, err
		}
	}

	if *inf.preprocessor.GetMLAlgorithmDefinition().AutoEventGenerationRequired {

		if len(inf.mlEventConfigs) == 0 {
			inf.lc.Errorf("no event config found, either not configured or not synced with node: mlConfigName: %s", inf.preprocessor.GetMLModelConfig().Name)
			return false, nil
		}

		// Need to handle multiple predictions during the Stabilization period/count and collapse them into one
		// if they are all open for same event or they are all closed for same event

		processedKeys := map[string]struct{}{}

		//	var chosenPrediction interface{}
		events := make([]models.HedgeEvent, 0)
		var newEvents []models.HedgeEvent
		for groupNameWithLotNoAndRecordNo, _ := range inferences {
			// keep building valid events
			if _, exist := processedKeys[groupNameWithLotNoAndRecordNo]; !exist {
				processedKeys, newEvents = inf.processPredictionToGenerateEvent(groupNameWithLotNoAndRecordNo, inputData, inferences, processedKeys)
				if len(newEvents) > 0 {
					events = append(events, newEvents...)
				}
			}
			// if already considered in earlier call, then nothing new to process, just return with status-quo
		}

		// Check if there are any events detected
		if len(events) == 0 {
			return false, nil
		} else {
			ctx.LoggingClient().Infof("new event generated, could be open or closed status: events-> %v", events)
			return true, events
		}
	}
	return false, nil
}

// Inputs: groupNameWithLotNoAndRecordNo: fully qualified group with lot number and record number that needs to be processed for event generation
//
//	inputDataMap: Full input map that was used to call the prediction, we need to use the data from it when we build the event
//	inferences: actual prediction map for all groupsWithLot no and recordNo in the lot
//	predictionsByActualGroup: Predictions by actual groupName ( no lot number or record no, eg deviceName).
//
// Returns:
//
//	Updated predictionsByActualGroup accounting for new group number that is being processed. If the shortlisted predicted was already processed, just return
//	event: New event if any after checking if the new event is to be generated, 1 event per stabilization period is accounted for in here
//
// Naming convention of groupName__lot#_recordNumber is used
func (inf *MLBrokerInferencing) processPredictionToGenerateEvent(groupNameWithLotNoAndRecordNo string, inputDataMap map[string]data.InferenceData, inferences map[string]interface{}, processedKeys map[string]struct{}) (map[string]struct{}, []models.HedgeEvent) {

	processedKeys[groupNameWithLotNoAndRecordNo] = struct{}{}
	// Step-1: Decode the group to get actual group & lot number. We use this to get multiple predictions as per the stabilizationCount

	groupName, lotNo := getGroupName(groupNameWithLotNoAndRecordNo)

	stabilizationCount := 1
	if len(inf.mlEventConfigs) > 0 {
		stabilizationCount = inf.mlEventConfigs[0].StabilizationPeriodByCount
	}

	// Step-2: Find which prediction to pick from among the multiple within the stabilization count, take the latest from them

	var lastInference interface{}
	// Latest eventStat during the stabilization period
	var lastEventStats []*EventStats
	lastGrpKey := ""

	eventStatsInStablizationPeriod := make([]*EventStats, 0)
	grpKey := ""

	for i := 0; i < stabilizationCount; i++ {
		if inf.preprocessor.GetMLAlgorithmDefinition().Type == helpers.TIMESERIES_ALGO_TYPE {
			grpKey = groupName
		} else {
			grpKey = fmt.Sprintf("%s__%d__%d", groupName, lotNo, i)
		}
		inf.lc.Debugf("grpKey: %s", grpKey)

		if _, found := inferences[grpKey]; found {
			lastInference = inferences[grpKey]
			inf.lc.Debugf("evaluating prediction: GroupKey: %s, %v", grpKey, lastInference)
			inf.lc.Debugf("inputDataMap[%s].Data[0]: %v", grpKey, inputDataMap[grpKey].Data[0])
			eventStat, err := inf.InferPredictionSeverity(lastInference, inputDataMap[grpKey].Data[0])
			//eventStat = [] if the threshold is not breached ie eventStat always holds Open events
			if err == nil {
				lastGrpKey = grpKey
				eventStatsInStablizationPeriod = append(eventStatsInStablizationPeriod, eventStat...)
				lastEventStats = eventStat
				inf.lc.Debugf("\teventStat: %v", eventStat)
			} else {
				inf.lc.Errorf("\terror getting eventStats for inference group: %v, err: %s", grpKey, err.Error())
			}
			processedKeys[grpKey] = struct{}{}
		}
	}
	if len(eventStatsInStablizationPeriod) == 0 {
		// no new event, strange? At least a close event expected - fixed that
		inf.lc.Errorf("impossible->no open/close event for the prediction group: %s, sample prediction: %v", grpKey, lastInference)
		return processedKeys, nil
	}

	// Step-3: We now have the eventStats for each prediction, pick one that are consistent in the stabilization count duration
	status := eventStatsInStablizationPeriod[0].Open
	eventName := eventStatsInStablizationPeriod[0].EventName
	for _, eventStat := range eventStatsInStablizationPeriod {
		if eventStat.Open != status || eventStat.EventName != eventName {
			inf.lc.Infof("predictions were different in the stabilization count period, so ignoring from event generation")
			return processedKeys, nil
		}
	}

	// Step-4: Generate event(s) from the lastEventStats gathered above, Only the last one is what we take since we need to consider one per stabilization count period

	events := make([]models.HedgeEvent, 0)
	for _, eventStat := range lastEventStats {

		if inf.checkIfNewEvent(groupName, eventStat) {
			eventBuilderData := &EventBuilderData{
				Tags: make(map[string]bool),
				//Predictions: make(map[float64]bool),
				Data: make([][]interface{}, 0), // Initialize empty slice for input data
			}

			inputs := inputDataMap[lastGrpKey]
			for _, input := range inputs.Data {
				eventBuilderData.Data = append(eventBuilderData.Data, input)
			}
			for _, tag := range inputs.Tags {
				eventBuilderData.Tags[tag.(string)] = true
			}

			event := inf.buildEvent(groupName, eventBuilderData, *eventStat)
			events = append(events, event)
			inf.lc.Infof("New event: %v", event)
		}
	}

	return processedKeys, events
}

// buildAndPublishMLPredictions builds and publishes predictions. It also persists the predictions in OpenSearch
func (inf *MLBrokerInferencing) buildAndPublishMLPredictions(
	ctx interfaces.AppFunctionContext,
	inferences map[string]interface{},
	inputData map[string]data.InferenceData,
) error {

	// groupKey in here is of the form GroupName__lot#__recordNumber, so need to split to actual groupName
	for groupKey, prediction := range inferences {
		var inputDataByDevice []map[string]interface{}

		// loop over each device's data array (each row in Data)
		for _, deviceData := range inputData[groupKey].Data {
			metricsMap := make(map[string]interface{})
			// loop over each metric name and corresponding index
			for metricName, index := range inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeatureNameToColumnIndex {
				// metricName := strings.Split(feature, "#")[1]
				if index < len(deviceData) {
					metricsMap[metricName] = deviceData[index]
				}
			}
			inputDataByDevice = append(inputDataByDevice, metricsMap)
		}

		// Save definitions for later use
		mlModelConfig := inf.preprocessor.GetMLModelConfig()
		mlAlgorithmDefinition := inf.preprocessor.GetMLAlgorithmDefinition()
		predictionLabel := mlAlgorithmDefinition.PredictionName
		if predictionLabel == "" {
			predictionLabel = "prediction"
		}

		prediction = inf.buildPredictionPayload(prediction)
		inf.lc.Infof("inputDataByDevice: %+v, prediction: %+v", inputDataByDevice, prediction)

		// Get the actual GroupName before exporting externally
		groupName, _ := getGroupName(groupKey)

		correlationId := buildCorrelationId(groupName, inf.preprocessor.GetMLModelConfig().Name)

		// Set the prediction label for dashboard diplay

		// Create the prediction structure to transmit
		timestamp, valid := inputDataByDevice[len(inputDataByDevice)-1]["Timestamp"].(int64)
		if !valid {
			timestamp = time.Now().Unix()
		}
		mlPrediction := models.MLPrediction{
			EntityType:           mlModelConfig.MLDataSourceConfig.EntityType,
			EntityName:           groupName,
			Devices:              inputData[groupKey].Devices,
			PredictionMessage:    mlModelConfig.Message,
			MLAlgoName:           mlModelConfig.MLAlgorithm,
			ModelName:            mlModelConfig.Name,
			MLAlgorithmType:      mlAlgorithmDefinition.Type,
			PredictionThresholds: mlAlgorithmDefinition.ThresholdUserParameterMap,
			InputDataByDevice:    inputDataByDevice,
			CorrelationId:        correlationId,
			Prediction:           prediction,
			PredictionDataType:   models.FLOAT,
			PredictionName:       predictionLabel,
			Created:              timestamp,
			Modified:             timestamp,
			Tags:                 inputData[groupKey].Tags,
		}

		ok, _ := inf.mqttSender.MQTTSend(ctx, mlPrediction)
		if !ok {
			inf.lc.Errorf("failed to publish ML prediction for groupName: %s", groupKey)
			return fmt.Errorf("failed to to publish prediction for groupName: %s", groupKey)
		} else {
			inf.lc.Infof("ML prediction published to mqtt for groupName %s: %v", groupKey, mlPrediction)
		}
	}

	return nil
}

// Also keeps track of last open event in MLBrokerInferencing#openEventsByGroup map
func (inf *MLBrokerInferencing) checkIfNewEvent(groupName string, currentEventStat *EventStats) bool {
	generateEvent := false
	if _, ok := inf.eventStatByGroup[groupName]; !ok {
		generateEvent = true
	} else {
		previousAnomalyState := inf.eventStatByGroup[groupName]
		if previousAnomalyState.Open != currentEventStat.Open {
			generateEvent = true
		} else if previousAnomalyState.Open && currentEventStat.Open && previousAnomalyState.LastSeverity != currentEventStat.LastSeverity {
			generateEvent = true
		}
	}

	inf.eventStatByGroup[groupName] = *currentEventStat
	// persist this now

	inf.lc.Infof("event summary: group: %s, will generate?: %t, stats: %v", groupName, generateEvent, *currentEventStat)
	return generateEvent
}

func (inf *MLBrokerInferencing) SetHttpSender(eventPipelineTriggerUrl string) *MLBrokerInferencing {
	sender := transforms.NewHTTPSender(eventPipelineTriggerUrl, "application/json", false)
	inf.httpSender = sender
	inf.eventPipelineTriggerUrl = eventPipelineTriggerUrl
	return inf
}

// Infers prediction for all algo types by explicitly calling specific prediction inference APIs
// returns eventStats with open/close event status
func (inf *MLBrokerInferencing) InferPredictionSeverity(prediction interface{}, inputData interface{}) ([]*EventStats, error) {
	var eventCandidatesStats []*EventStats

	// Now infer the severity and whether it is open/close event
	switch inf.preprocessor.GetMLAlgorithmDefinition().Type {
	case helpers.CLASSIFICATION_ALGO_TYPE:
		err := inf.inferPredictionSeverityForClassification(prediction, &eventCandidatesStats)
		if err != nil {
			return nil, err
		}
	case helpers.ANOMALY_ALGO_TYPE:
		err := inf.inferPredictionSeverityForAnomaly(prediction, &eventCandidatesStats)
		if err != nil {
			return nil, err
		}
	case helpers.TIMESERIES_ALGO_TYPE:
		err := inf.inferPredictionSeverityForTimeseries(prediction, &eventCandidatesStats)
		if err != nil {
			return nil, err
		}
	case helpers.REGRESSION_ALGO_TYPE:
		err := inf.inferPredictionSeverityForRegression(prediction, &eventCandidatesStats, inputData)
		if err != nil {
			return nil, err
		}
	default:
		inf.lc.Warnf("Unknown algorithm type: %s", inf.preprocessor.GetMLAlgorithmDefinition().Type)
		return nil, fmt.Errorf("unknown algorithm type: %s", inf.preprocessor.GetMLAlgorithmDefinition().Type)
	}

	return eventCandidatesStats, nil
}

func (inf *MLBrokerInferencing) inferPredictionSeverityForClassification(prediction interface{}, eventCandidatesStats *[]*EventStats) error {
	predMap, ok := prediction.(map[string]interface{})
	if !ok {
		return fmt.Errorf("failed to cast prediction to map[string]interface{} for 'Classification'")
	}
	confidence, ok := predMap["confidence"].(float64)
	if !ok {
		return fmt.Errorf("confidence value not found or invalid")
	}
	class, ok := predMap["class"].(string)
	if !ok {
		return fmt.Errorf("class value not found or invalid")
	}

	// get the event definition and thresholds per class
	eventDefinitions := inf.mlEventConfigs
	for _, eventDefinition := range eventDefinitions {
		conditions := eventDefinition.Conditions

		// Classification: expected max 1 condition (1 severity level) per event and 2 threshold per condition - for class name and for confidence
		thresholds := conditions[0].ThresholdsDefinitions
		if len(thresholds) == 0 {
			inf.lc.Errorf("Threshold for 'Classification' algorithm is not configured, model: %s", inf.preprocessor.GetMLModelConfig().Name)
			return fmt.Errorf("threshold for 'Classification' algorithm is not configured, model: %s", inf.preprocessor.GetMLModelConfig().Name)
		}

		eventStats := &EventStats{
			Open:           false,
			PredLoss:       make(map[string]interface{}),
			LastSeverity:   "",
			EventName:      eventDefinition.EventName,
			EventMessage:   eventDefinition.Description,
			Thresholds:     make(map[string]interface{}),
			RelatedMetrics: []string{},
		}
		openEvent := false
		var err error
		var predictionValue interface{}
		var confidenceThreshold config.ThresholdDefinition

		for _, threshold := range thresholds {
			_, ok := threshold.ThresholdValue.(string)
			if ok {
				predictionValue = class
			} else {
				confidenceThreshold = threshold
				predictionValue = confidence
			}
			openEvent, err = inf.computeSeverityForPrediction(threshold, predictionValue)
			if err != nil {
				return err
			}
		}

		if openEvent {
			eventStats.Open = true
			eventStats.PredLoss["Predicted class"] = class
			eventStats.PredLoss["Confidence"] = confidence
			eventStats.LastSeverity = conditions[0].SeverityLevel
			eventStats.RelatedMetrics = []string{class}
			eventStats.Thresholds["Confidence"] = confidenceThreshold
		} // else we already have a closed event
		*eventCandidatesStats = append(*eventCandidatesStats, eventStats)
		inf.lc.Debugf("Computed event for'Classification', event stats: %v", eventStats)
	}

	return nil
}

func (inf *MLBrokerInferencing) inferPredictionSeverityForAnomaly(prediction interface{}, eventCandidatesStats *[]*EventStats) error {
	// Use anomaly detection-specific logic
	predictionValue, ok := prediction.(float64)
	if !ok {
		return fmt.Errorf("failed to cast prediction value to float64 for 'Anomaly'")
	}

	eventDefinitions := inf.mlEventConfigs
	conditions := eventDefinitions[0].Conditions // for Anomaly there is only 1 event defined

	eventStats := &EventStats{
		Open:           false,
		PredLoss:       make(map[string]interface{}),
		LastSeverity:   "",
		EventName:      eventDefinitions[0].EventName,
		EventMessage:   eventDefinitions[0].Description,
		Thresholds:     make(map[string]interface{}),
		RelatedMetrics: make([]string, len(inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeatureNameToColumnIndex)), // Pre-allocate slice with required length
	}

	for _, condition := range conditions {
		// For Anomaly: expected max 3 conditions per event (1 for each severity level), 1 threshold per each condition
		thresholds := condition.ThresholdsDefinitions
		if len(thresholds) == 0 {
			inf.lc.Debugf("Thresholds for 'Anomaly' algorithm are not configured, model: %s", inf.preprocessor.GetMLModelConfig().Name)
			return fmt.Errorf("thresholds for 'Anomaly' algorithm are not configured, model: %s", inf.preprocessor.GetMLModelConfig().Name)
		}

		anomalyFound, err := inf.computeSeverityForPrediction(thresholds[0], predictionValue)
		if err != nil {
			return err
		}
		if anomalyFound {
			eventStats.Open = true
			eventStats.PredLoss["Prediction"] = predictionValue
			eventStats.LastSeverity = condition.SeverityLevel
			eventStats.Thresholds["Prediction"] = thresholds[0]
			// for Anomaly: RelatedMetrics is a list of metrics names defined in FeatureNameToColumnIndex:
			for metricName, i := range inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeatureNameToColumnIndex {
				if i < len(eventStats.RelatedMetrics) {
					eventStats.RelatedMetrics[i] = metricName
				}
			}
			break
		}
	}

	// If none of the conditions are satisfied, it means there is no Open event, so just add a close event
	*eventCandidatesStats = append(*eventCandidatesStats, eventStats)
	inf.lc.Debugf("Computed eventStats for 'Anomaly': %v", eventStats)
	return nil
}

func (inf *MLBrokerInferencing) inferPredictionSeverityForTimeseries(prediction interface{}, eventCandidatesStats *[]*EventStats) error {
	predMap, ok := prediction.(map[string]interface{})
	if !ok {
		return fmt.Errorf("failed to cast prediction to map[string]interface{} for 'Timeseries'")
	}

	predictionsRaw, ok := predMap["predictions"].([]interface{})
	if !ok {
		return fmt.Errorf("'predictions' value not found or invalid")
	}

	// Convert to [][]interface{}
	var predictions [][]interface{}
	for _, rawRow := range predictionsRaw {
		row, ok := rawRow.([]interface{})
		if !ok {
			return fmt.Errorf("invalid row in 'predictions'")
		}
		predictions = append(predictions, row)
	}

	// get the event definition and thresholds per class
	eventDefinitions := inf.mlEventConfigs
	for _, eventDefinition := range eventDefinitions {
		// Timeseries: expected 1 condition per event (severity level MINOR/MAJOR/CRITICAL), a few thresholds per condition
		conditions := eventDefinition.Conditions
		// One event stats per event Definition
		eventStats := &EventStats{
			Open:           false,
			PredLoss:       make(map[string]interface{}),
			LastSeverity:   "",
			EventName:      eventDefinition.EventName,
			EventMessage:   eventDefinition.Description,
			Thresholds:     make(map[string]interface{}),
			RelatedMetrics: []string{},
		}
		openEvent := false
		var err error

		for _, condition := range conditions {
			thresholds := condition.ThresholdsDefinitions
			if len(thresholds) == 0 {
				inf.lc.Debugf("Thresholds for 'Timeseries' algorithm are not configured, model: %s", inf.preprocessor.GetMLModelConfig().Name)
				return fmt.Errorf("thresholds for 'Timeseries' algorithm are not configured, model: %s", inf.preprocessor.GetMLModelConfig().Name)
			}

			var predictionValue interface{}
			// For timeseries, the prediction is sequence of predictions aka timeseries
			for _, row := range predictions {
				for _, threshold := range thresholds {
					inf.lc.Debugf("OutputFeaturesToIndex: %v", inf.preprocessor.GetOutputFeaturesToIndex())
					inf.lc.Debugf("FeatureNameToColumnIndex: %v", inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeatureNameToColumnIndex)

					columnIndex, exists := inf.preprocessor.GetOutputFeaturesToIndex()[threshold.Label]
					if !exists {
						inf.lc.Errorf("Threshold label %s not found in OutputFeaturesToIndex", threshold.Label)
						continue
					}
					predictionValue = row[columnIndex]
					if predictionValue == nil {
						inf.lc.Errorf("Prediction value is nil for column index %d, row: %v", columnIndex, row)
						continue
					}
					inf.lc.Debugf("Prediction value: %v for threshold: %v", predictionValue, threshold)

					openEvent, err = inf.computeSeverityForPrediction(threshold, predictionValue)
					if err != nil {
						inf.lc.Errorf("Error computing severity for prediction: %v, error: %v", predictionValue, err)
						return err
					}
					// The operator is always AND:
					if openEvent {
						eventStats.PredLoss[threshold.Label] = predictionValue
						eventStats.Thresholds[threshold.Label] = threshold
						break
					}
				}
				if openEvent {
					eventStats.Open = true
					eventStats.LastSeverity = condition.SeverityLevel
					// for Timeseries: RelatedMetrics is a list of metrics names defined in FeatureNameToColumnIndex:
					for metricName, i := range inf.preprocessor.GetOutputFeaturesToIndex() {
						if i != 0 { // skip Timestamp
							eventStats.RelatedMetrics = append(eventStats.RelatedMetrics, metricName)
						}
					}
					break // if there is match in the row - skip the rest of rows
				}
			}
			if openEvent {
				break
			}

		}

		*eventCandidatesStats = append(*eventCandidatesStats, eventStats)
		inf.lc.Debugf("Computed event Stats for 'Timeseries': %v", eventStats)

	}

	return nil
}

func (inf *MLBrokerInferencing) inferPredictionSeverityForRegression(prediction interface{}, eventCandidatesStats *[]*EventStats, inputData interface{}) error {
	// Use regression detection-specific logic
	predictedFeatureValue, ok := prediction.(float64) // the value is predicted for 1 feature of 1 profile
	if !ok {
		return fmt.Errorf("failed to cast prediction value to float64 for 'Regression'")
	}
	inf.lc.Debugf("Predicted feature value: %.2f", predictedFeatureValue)

	eventDefinitions := inf.mlEventConfigs
	for _, eventDefinition := range eventDefinitions {
		conditions := eventDefinition.Conditions
		// For Regression: expected max 1 threshold per condition
		thresholds := conditions[0].ThresholdsDefinitions
		if len(thresholds) == 0 {
			inf.lc.Errorf("Thresholds for 'Regression' algorithm are not configured, model: %s", inf.preprocessor.GetMLModelConfig().Name)
			return fmt.Errorf("thresholds for 'Regression' algorithm are not configured, model: %s", inf.preprocessor.GetMLModelConfig().Name)
		}

		var actualFeatureValue float64
		featureNameToAnalyze := inf.getOutputFeatureFromConfig()
		if featureNameToAnalyze != "" {
			value, err := inf.getOutputFeatureValue(featureNameToAnalyze, inputData)
			if err != nil {
				inf.lc.Errorf("Failed to get actual feature value for '%s': %v", featureNameToAnalyze, err)
				return fmt.Errorf("failed to get actual feature value for '%s': %v", featureNameToAnalyze, err)
			} else {
				actualFeatureValue = value
				inf.lc.Debugf("Actual feature '%s' value successfully retrieved: %f", featureNameToAnalyze, actualFeatureValue)
			}
		} else {
			inf.lc.Errorf("No isOutput==true feature found in the model config %s, features list: %v", inf.preprocessor.GetMLModelConfig().Name, inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeaturesByProfile)
			return fmt.Errorf("no isOutput==true feature found in the model config %s, features list: %v", inf.preprocessor.GetMLModelConfig().Name, inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeaturesByProfile)
		}

		percentageChange, err := calculatePercentageChange(actualFeatureValue, predictedFeatureValue)
		if err != nil {
			inf.lc.Errorf("error while calculating percentage change: %s\n", err.Error())
			return fmt.Errorf("error while calculating percentage change: %s\n", err.Error())
		} else {
			inf.lc.Debugf("The percentage change is %.2f%%\n", percentageChange)
		}

		eventStats := &EventStats{
			EventName:      eventDefinitions[0].EventName,
			Open:           false,
			PredLoss:       make(map[string]interface{}),
			LastSeverity:   "",
			EventMessage:   eventDefinitions[0].Description,
			Thresholds:     make(map[string]interface{}),
			RelatedMetrics: make([]string, len(inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeatureNameToColumnIndex)), // Pre-allocate slice with required length
		}

		// Find the relevant feature per profile
		regressionFound, err := inf.computeSeverityForPrediction(thresholds[0], percentageChange)
		if err != nil {
			return err
		}
		if regressionFound {
			eventStats.Open = true
			eventStats.PredLoss["Prediction"] = predictedFeatureValue
			eventStats.LastSeverity = conditions[0].SeverityLevel
			eventStats.RelatedMetrics = []string{featureNameToAnalyze}
			eventStats.Thresholds[featureNameToAnalyze] = thresholds[0]
		}
		// One event ( open or close per eventDefintion, we donot support multiple event definitions from UI for regression
		// however, this is still kept since we might define 2 events - BelowAvg, AboveAvg etc
		*eventCandidatesStats = append(*eventCandidatesStats, eventStats)
		inf.lc.Debugf("Computed eventStats 'Regression': %v", eventStats)
	}

	return nil
}

func (inf *MLBrokerInferencing) computeSeverityForPrediction(thresholdDefinition config.ThresholdDefinition, prediction interface{}) (bool, error) {
	switch thresholdDefinition.Operator {
	case config.BETWEEN_OPERATOR:
		predictionValue, ok := prediction.(float64)
		if !ok {
			inf.lc.Errorf("Prediction value %v is not a float64 for operator %s", prediction, thresholdDefinition.Operator)
			return false, fmt.Errorf("invalid type for prediction value: %T", prediction)
		}
		if predictionValue >= thresholdDefinition.LowerThreshold && predictionValue < thresholdDefinition.UpperThreshold {
			return true, nil
		}
	case config.EQUAL_TO_OPERATOR:
		// Attempt to interpret prediction as a string (Classification case only)
		predictionValue, isString := prediction.(string)
		inf.lc.Debugf("is predictionValue string? %v: %v", predictionValue, isString)

		if !isString {
			predictionFloat, isFloat := prediction.(float64)
			if !isFloat {
				inf.lc.Errorf("Prediction value %v is of unsupported type %T for operator %s", prediction, prediction, thresholdDefinition.Operator)
				return false, fmt.Errorf("invalid type for prediction value: %T", prediction)
			}
			thresholdFloat, isFloat := thresholdDefinition.ThresholdValue.(float64)
			if !isFloat {
				inf.lc.Warnf("Threshold value %v is of unsupported type %T for operator %s", thresholdDefinition.ThresholdValue, thresholdDefinition.ThresholdValue, thresholdDefinition.Operator)
				return false, fmt.Errorf("invalid type for threshold value: %T", thresholdDefinition.ThresholdValue)
			}
			inf.lc.Debugf("predictionFloat %2f, thresholdFloat %2f", predictionFloat, thresholdFloat)

			if math.Abs(predictionFloat-thresholdFloat) < 1e-2 {
				return true, nil
			}
			return false, nil
		}

		// Handle the case where both are strings
		thresholdValue, isString := thresholdDefinition.ThresholdValue.(string)
		if !isString {
			inf.lc.Warnf("Threshold value %v is of unsupported type %T for operator %s", thresholdDefinition.ThresholdValue, thresholdDefinition.ThresholdValue, thresholdDefinition.Operator)
			return false, fmt.Errorf("invalid type for threshold value: %T", thresholdDefinition.ThresholdValue)
		}
		inf.lc.Debugf("predictionValue %s, thresholdValue %s", predictionValue, thresholdValue)

		if predictionValue == thresholdValue {
			return true, nil
		}
		return false, nil
	case config.GREATER_THAN_OPERATOR:
		predictionValue, ok := prediction.(float64)
		if !ok {
			inf.lc.Errorf("Prediction value %v is not a float64 for operator %s", prediction, thresholdDefinition.Operator)
			return false, fmt.Errorf("invalid type for prediction value: %T", prediction)
		}
		thresholdValue, err := utils.ToFloat64(thresholdDefinition.ThresholdValue)
		if err != nil {
			inf.lc.Warnf("Wrong threshold value type for operator: %s", thresholdDefinition.Operator)
			return false, fmt.Errorf("wrong threshold value type for operator: %s", thresholdDefinition.Operator)
		}

		if predictionValue >= thresholdValue {
			return true, nil
		}
	case config.LESS_THAN_OPERATOR:
		predictionValue, ok := prediction.(float64)
		if !ok {
			inf.lc.Errorf("Prediction value %v is not a float64 for operator %s", prediction, thresholdDefinition.Operator)
			return false, fmt.Errorf("invalid type for prediction value: %T", prediction)
		}
		thresholdValue, err := utils.ToFloat64(thresholdDefinition.ThresholdValue)
		if err != nil {
			inf.lc.Warnf("Wrong threshold value type for operator: %s", thresholdDefinition.Operator)
			return false, fmt.Errorf("wrong threshold value type for operator: %s", thresholdDefinition.Operator)
		}

		if predictionValue < thresholdValue {
			return true, nil
		}
	default:
		inf.lc.Warnf("Unknown threshold operator type: %s", thresholdDefinition.Operator)
		return false, fmt.Errorf("unknown threshold operator type: %s", thresholdDefinition.Operator)
	}

	return false, nil
}

func (inf *MLBrokerInferencing) getPredictions(data bytes.Buffer) (map[string]interface{}, error) {
	inf.lc.Infof("Data sent to getPredictions: %v", data.String())
	dataBytes := data.Bytes()

	predictEndPoint := inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.PredictionDataSourceConfig.PredictionEndPointURL
	req, err := http.NewRequest("POST", predictEndPoint, bytes.NewReader(dataBytes))
	if err != nil {
		return nil, err
	}
	resp, err := inf.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		inf.lc.Errorf("Error getting the prediction, Check the inferencing end Point URL")
		inf.lc.Errorf("inferencing Endpoint: %s", predictEndPoint)
		return nil, errors.New("error getting the prediction")
	}
	defer resp.Body.Close()
	predByte, err := io.ReadAll(resp.Body)
	if err != nil {
		inf.lc.Errorf("Error reading the body, Check the inferencing end Point URL")
		return nil, errors.New("error getting the prediction")
	}

	var prediction map[string]interface{}
	err = json.Unmarshal(predByte[:], &prediction)
	if err != nil {
		inf.lc.Errorf("error reading the body from inferencing URL, error: %v", err)
		return nil, err
	}

	return prediction, nil
}

func (inf *MLBrokerInferencing) PublishEvents(ctx interfaces.AppFunctionContext, params interface{}) (continuePipeline bool, result interface{}) {
	ctx.LoggingClient().Info("Running Inference for " + ctx.PipelineId())

	if params == nil {
		return false, errors.New("no event received")
	}

	events := params.([]models.HedgeEvent)

	ctx.LoggingClient().Infof("About to publish anomaly events: %v", events)

	errCount := 0
	for _, event := range events {
		if ok, _ := inf.httpSender.HTTPPost(ctx, event); !ok {
			ctx.LoggingClient().Errorf("%s, event: %v", EVENT_PUB_ERR_MSG, event)
			errCount++
		} else {
			ctx.LoggingClient().Infof("Anomaly event successfully published: %v", event)
		}
	}
	if errCount == len(events) {
		return false, errors.New(EVENT_PUB_ERR_MSG)
	}
	return false, nil
}

func (inf *MLBrokerInferencing) buildEvent(groupName string, eventBuilderData *EventBuilderData, eventStats EventStats) models.HedgeEvent {

	currentTime := time.Now().Unix() * 1000
	correlationId := buildCorrelationId(groupName, inf.preprocessor.GetMLModelConfig().Name)

	event := models.HedgeEvent{
		Id:             "",
		DeviceName:     groupName,
		Class:          models.BASE_EVENT_CLASS,
		EventType:      inf.preprocessor.GetMLAlgorithmDefinition().Type,
		Name:           eventStats.EventName,
		Msg:            fmt.Sprintf("%s:%s", groupName, eventStats.EventMessage),
		Severity:       eventStats.LastSeverity,
		RelatedMetrics: eventStats.RelatedMetrics,
		Thresholds:     eventStats.Thresholds,
		ActualValues:   eventStats.PredLoss,
		Modified:       currentTime,
		EventSource:    "HEDGE",
		CorrelationId:  correlationId,
	}

	if eventStats.Open {
		event.Status = "Open"
		event.Created = currentTime
	} else {
		event.Status = "Closed"
	}

	event.AdditionalData = make(map[string]string)
	if eventBuilderData.Tags != nil {
		for key, val := range eventBuilderData.Tags {
			event.AdditionalData[key] = strconv.FormatBool(val)
		}
	}
	// Accessing the metrics data from severityData

	for metricName, index := range inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeatureNameToColumnIndex {
		if len(eventBuilderData.Data) > 0 && index < len(eventBuilderData.Data[0]) {
			event.AdditionalData[metricName] = fmt.Sprintf("%f", eventBuilderData.Data[0][index])
		}
	}

	inf.lc.Infof("Event built for Group: %s, Status: %s", event.DeviceName, event.Status)
	return event
}

func (inf *MLBrokerInferencing) getOutputFeatureFromConfig() string {
	for profile, features := range inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeaturesByProfile {
		for _, feature := range features {
			if feature.IsOutput {
				return helpers.BuildFeatureName(profile, feature.Name)
			}
		}
	}
	return ""
}

func (inf *MLBrokerInferencing) getOutputFeatureValue(featureName string, inputData interface{}) (float64, error) {
	featureArray, ok := inputData.([]interface{})
	if !ok {
		return 0, fmt.Errorf("inputData is not of type []interface{}, got %T", inputData)
	}

	// Get the index for the feature name from the map
	index, exists := inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeatureNameToColumnIndex[featureName]
	if !exists {
		return 0, fmt.Errorf("feature name '%s' not found in map", featureName)
	}
	if index < 0 || index >= len(featureArray) {
		return 0, fmt.Errorf("index %d for feature '%s' is out of bounds", index, featureName)
	}
	// Convert the value to float64 because this is expected value
	outputFeatureValue, ok := featureArray[index].(float64)
	if !ok {
		return 0, fmt.Errorf("output feature value is not of type float64, got %T", featureArray[index])
	}

	return outputFeatureValue, nil
}

// Builds prediction payload to also contain the name of the prediction not just the value
// important for timeseries where we predict multiple metrics without reference to which metric we are predicting
// the above creates a challenge when we publish it
func (inf *MLBrokerInferencing) buildPredictionPayload(prediction interface{}) interface{} {

	mlAlgorithmDefinition := inf.preprocessor.GetMLAlgorithmDefinition()
	predictionLabel := mlAlgorithmDefinition.PredictionName
	if predictionLabel == "" {
		predictionLabel = "prediction"
	}
	if mlAlgorithmDefinition.Type != helpers.TIMESERIES_ALGO_TYPE {
		return prediction
	}
	/* Output format for timeseries
	{
	[
	            [
	                1734533580,
	                "AltaNS12",
	                1071.16,
	                13.96
	            ],
	            [
	                1734533640,
	                "AltaNS12",
	                1020.31,
	                14.95
	            ]
	        ]
	}
	*/

	outputFeatures2IndexMap := inf.preprocessor.GetOutputFeaturesToIndex()
	// map becos we have prediction, min & max range predictions
	predictionMap, ok := prediction.(map[string]interface{})
	if !ok {
		return nil
	}

	xformedPedictionMap := make(map[string][]map[string]interface{})
	for predKay, _ := range predictionMap {
		predictions, ok := predictionMap[predKay].([]interface{})
		if !ok {
			return nil
		}
		xformedPedictionMap[predKay] = make([]map[string]interface{}, len(predictions))
		for k, pred := range predictions {
			xformedPedictionMap[predKay][k] = make(map[string]interface{})
			predTSValues, ok := pred.([]interface{})
			if !ok {
				return nil
			}
			for metricName, i := range outputFeatures2IndexMap {
				val := predTSValues[i]
				if i > 1 || i == 0 {
					// ie not equal to deviceName
					//predTSValues[i-1] = map[string]interface{}{metricName: val}
					xformedPedictionMap[predKay][k][metricName] = val
				}
			}
		}

	}

	return xformedPedictionMap
}
