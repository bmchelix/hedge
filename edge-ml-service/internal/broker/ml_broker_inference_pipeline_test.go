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
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"hedge/common/models"
	"hedge/edge-ml-service/pkg/helpers"
	svcmocks "hedge/mocks/hedge/common/service"
	helpersmocks "hedge/mocks/hedge/edge-ml-service/pkg/helpers"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"

	r "hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/data"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var mlModelConfig = config.MLModelConfig{
	Name:               "WindTurbineAnomaly",
	Version:            "2",
	Description:        "Wind Turbine Anomaly",
	MLAlgorithm:        "HedgeAnomaly",
	Enabled:            true,
	ModelConfigVersion: 0,
	MLDataSourceConfig: config.MLModelDataConfig{
		FeatureNameToColumnIndex: make(map[string]int),
		TrainingDataSourceConfig: config.TrainingDataSourceConfig{
			SamplingIntervalSecs: 2,
		},
		PredictionDataSourceConfig: config.PredictionDataSourceConfig{
			SamplingIntervalSecs: 2,
		},
	},
	LocalModelStorageDir: "/tmp",
	TrainedModelCount:    1,
}

var mlDbClient redis.MockMLDbInterface
var mockedHttpClient = &svcmocks.MockHTTPClient{}
var mockedMqttSender = &svcmocks.MockMqttSender{}
var mockedPreProcessor = &helpersmocks.MockPreProcessorInterface{}
var mockResponse = &http.Response{}

func init() {
	InstantiateMLBrokerDBClient()
}

func InstantiateMLBrokerDBClient() {
	mlDbClient = redis.MockMLDbInterface{}
	r.MLDbClientImpl = &mlDbClient
	mlDbClient.On("GetDbClient", mock.Anything).Return(r.MLDbClientImpl)

	mlDbClient.On("GetAllMLEventConfigsByConfig", "HedgeAnomaly", mock.Anything, mock.Anything).Return([]config.MLEventConfig{CreateEventConfig(helpers.ANOMALY_ALGO_TYPE)}, nil)
	mlDbClient.On("GetAllMLEventConfigsByConfig", "HedgeClassification", mock.Anything, mock.Anything).Return([]config.MLEventConfig{CreateEventConfig(helpers.CLASSIFICATION_ALGO_TYPE)}, nil)
	mlDbClient.On("GetAllMLEventConfigsByConfig", "HedgeTimeseries", mock.Anything, mock.Anything).Return([]config.MLEventConfig{CreateEventConfig(helpers.TIMESERIES_ALGO_TYPE)}, nil)
	mlDbClient.On("GetAllMLEventConfigsByConfig", "HedgeRegression", mock.Anything, mock.Anything).Return([]config.MLEventConfig{CreateEventConfig(helpers.REGRESSION_ALGO_TYPE)}, nil)
}

func CreateEventConfig(algoType string) config.MLEventConfig {
	switch algoType {
	case helpers.ANOMALY_ALGO_TYPE:
		return config.NewDefaultAnomalyEventConfig("HedgeAnomaly", "WindTurbineAnomaly")
	case helpers.CLASSIFICATION_ALGO_TYPE:
		eventConfig := config.MLEventConfig{
			MLAlgorithm:       "HedgeClassification",
			MlModelConfigName: "HedgeClassificationTrainingConfig",
			EventName:         "Car identified",
			Description:       "event message",
			Conditions: []config.Condition{
				{
					ThresholdsDefinitions: []config.ThresholdDefinition{
						{
							Label:          "Car",
							ThresholdValue: 5.5,
							Operator:       config.LESS_THAN_OPERATOR,
						},
					},
					SeverityLevel: models.SEVERITY_MINOR,
				},
			},
			StabilizationPeriodByCount: 3,
		}
		return eventConfig
	case helpers.TIMESERIES_ALGO_TYPE:
		eventConfig := config.MLEventConfig{
			MLAlgorithm:       "HedgeTimeseries",
			MlModelConfigName: "HedgeTimeseriesTrainingConfig",
			EventName:         "High metric value identified",
			Description:       "event message",
			Conditions: []config.Condition{
				{
					ThresholdsDefinitions: []config.ThresholdDefinition{
						{
							Label:          "WindTurbine#WindSpeed",
							ThresholdValue: 10.1,
							Operator:       config.EQUAL_TO_OPERATOR,
						},
						{
							Label:          "WindTurbine#TurbinePower",
							LowerThreshold: 10.5,
							UpperThreshold: 11.5,
							Operator:       config.BETWEEN_OPERATOR,
						},
					},
					SeverityLevel: models.SEVERITY_MINOR,
				},
			},
			StabilizationPeriodByCount: 1,
		}
		return eventConfig
	case helpers.REGRESSION_ALGO_TYPE:
		eventConfig := config.MLEventConfig{
			MLAlgorithm:       "HedgeRegression",
			MlModelConfigName: "HedgeRegressionTrainingConfig",
			EventName:         "Regression identified",
			Description:       "event message",
			Conditions: []config.Condition{
				{
					ThresholdsDefinitions: []config.ThresholdDefinition{
						{
							Label:          "WindTurbine#TurbinePower",
							ThresholdValue: 5.5,
							Operator:       config.LESS_THAN_OPERATOR,
						},
					},
					SeverityLevel: models.SEVERITY_MINOR,
				},
			},
			StabilizationPeriodByCount: 3,
		}
		return eventConfig
	}
	return config.MLEventConfig{}
}

// Valid Constructor that can be used for most test cases
func BuildMLBrokerPipelineMocks(algoType string) (*MLBrokerInferencing, interfaces.ApplicationService, interfaces.AppFunctionContext) {
	service := utils.NewApplicationServiceMock(nil).AppService
	ctx := utils.NewApplicationServiceMock(nil).AppFunctionContext
	ctx.On("ApplyValues", "http://hedge-event-publisher:48102/trigger").Return("http://hedge-event-publisher:48102/trigger", nil)
	trainingConfigValid := mlModelConfig
	featuresByProfile := make(map[string][]config.Feature, 0)
	f1 := config.Feature{
		Type:    "METRIC",
		Name:    "WindSpeed",
		IsInput: true,
		//IsOutput: false,
	}
	f2 := config.Feature{
		Type:     "METRIC",
		Name:     "TurbinePower",
		IsInput:  true,
		IsOutput: true,
	}

	featuresByProfile["WindTurbine"] = []config.Feature{f1, f2}
	// Case of no features, so return nil
	trainingConfigValid.MLDataSourceConfig.FeaturesByProfile = featuresByProfile
	mlModelConfig.MLDataSourceConfig.PredictionDataSourceConfig.PredictionEndPointURL = "http://localhost:48096" + "/predict/" + mlModelConfig.MLAlgorithm + "/" + mlModelConfig.Name
	mlAlgoDefinition := &config.MLAlgorithmDefinition{}

	trainingConfigValid.MLDataSourceConfig.FeatureNameToColumnIndex = make(map[string]int)
	trainingConfigValid.MLDataSourceConfig.FeatureNameToColumnIndex["WindTurbine#WindSpeed"] = 0
	trainingConfigValid.MLDataSourceConfig.FeatureNameToColumnIndex["WindTurbine#TurbinePower"] = 1

	switch algoType {
	case helpers.ANOMALY_ALGO_TYPE:
		mlAlgoDefinition.PredictionPayloadTemplate.Template = helpers.ANOMALY_PAYLOAD_TEMPLATE
		mlAlgoDefinition.Type = algoType
	case helpers.CLASSIFICATION_ALGO_TYPE:
		mlAlgoDefinition.PredictionPayloadTemplate.Template = helpers.CLASSIFICATION_PAYLOAD_TEMPLATE
		mlAlgoDefinition.Type = algoType
		trainingConfigValid.MLAlgorithm = "HedgeClassification"
		trainingConfigValid.Name = "HedgeClassificationModel"
	case helpers.TIMESERIES_ALGO_TYPE:
		mlAlgoDefinition.PredictionPayloadTemplate.Template = helpers.TIMESERIES_PAYLOAD_TEMPLATE
		mlAlgoDefinition.Type = algoType
		trainingConfigValid.MLAlgorithm = "HedgeTimeseries"
		trainingConfigValid.Name = "HedgeTimeseriesModel"

		trainingConfigValid.MLDataSourceConfig.FeatureNameToColumnIndex["Timestamp"] = 0
		trainingConfigValid.MLDataSourceConfig.FeatureNameToColumnIndex["WindTurbine#WindSpeed"] = 1
		trainingConfigValid.MLDataSourceConfig.FeatureNameToColumnIndex["WindTurbine#TurbinePower"] = 2
		trainingConfigValid.MLDataSourceConfig.InputContextCount = 1
	case helpers.REGRESSION_ALGO_TYPE:
		mlAlgoDefinition.PredictionPayloadTemplate.Template = helpers.REGRESSION_PAYLOAD_TEMPLATE
		mlAlgoDefinition.Type = algoType
		trainingConfigValid.MLAlgorithm = "HedgeRegression"
		trainingConfigValid.Name = "HedgeRegressionModel"

		featuresByProfile := make(map[string][]config.Feature, 0)
		f1 := config.Feature{
			Type:     "METRIC",
			Name:     "WindSpeed",
			IsInput:  true,
			IsOutput: false,
		}
		f2 := config.Feature{
			Type:     "METRIC",
			Name:     "TurbinePower",
			IsInput:  true,
			IsOutput: true,
		}
		featuresByProfile["WindTurbine"] = []config.Feature{f1, f2}
		trainingConfigValid.MLDataSourceConfig.FeaturesByProfile = featuresByProfile
	}

	mlBrokerInf := NewMLBrokerInference(&trainingConfigValid, mlAlgoDefinition, &MLBrokerConfig{}, service, make(chan string), true)

	return mlBrokerInf, service, ctx
}

// Tests to validate the contructor for the pipeline, error scenario and the normal scenario
func TestNewTestMLBrokerInferencing(t *testing.T) {

	service := utils.NewApplicationServiceMock(nil).AppService

	// Case of no features, so return nil
	mlBrokerInfErr := NewMLBrokerInference(&mlModelConfig, &config.MLAlgorithmDefinition{}, &MLBrokerConfig{}, service, nil, false)
	assert.Nil(t, mlBrokerInfErr)

	// Test #2: When there are valid features
	mlBrokerInf, _, _ := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)

	assert.IsType(t, &MLBrokerInferencing{}, mlBrokerInf)

}

func TestMLBrokerInferencing_FilterByFeatureNames(t *testing.T) {

	inf, _, ctx := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)

	eventDataMatchingMLFeature := buildMetricEvent("WindSpeed", "Float64", "1.090002e+01")
	continuePipeline, event := inf.FilterByFeatureNames(ctx, eventDataMatchingMLFeature)
	if continuePipeline != true {
		t.Errorf("FilterByFeatureNames() continuePipeline = %v, want %v", continuePipeline, true)
	}
	if !reflect.DeepEqual(event, eventDataMatchingMLFeature) {
		t.Errorf("FilterByFeatureNames() gotResult = %v, want %v", event, eventDataMatchingMLFeature)
	}

	eventDataNotMatchingFeature := buildMetricEvent("NonExistingMetric", "Uint64", "12")
	continuePipeline, event = inf.FilterByFeatureNames(ctx, eventDataNotMatchingFeature)
	if continuePipeline != true {
		t.Errorf("FilterByFeatureNames() continuePipeline = %v, want %v", continuePipeline, false)
	}
	ev := event.(dtos.Event)
	if len(ev.Readings) != 0 {
		t.Errorf("FilterByFeatureNames() expected empty reading, got %v", ev.Readings)
	}
	eventDataNotMatchingProfile := buildMetricEvent("NotMatchingProfile", "Uint64", "12")
	eventDataNotMatchingProfile.ProfileName = "Rotor"
	continuePipeline, _ = inf.FilterByFeatureNames(ctx, eventDataNotMatchingProfile)
	if continuePipeline != false {
		t.Errorf("FilterByFeatureNames() continuePipeline = %v, want %v", continuePipeline, false)
	}

}

func TestMLBrokerInferencing_TakeSample(t *testing.T) {

	inf, _, ctx := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)

	eventMetric := buildMetricEvent("WindSpeed", "Float64", "1.090002e+01")
	continuePipeline, _ := inf.TakeSample(ctx, eventMetric)
	if continuePipeline {
		t.Errorf("TakeSample() continuePipeline = %v, want %v", continuePipeline, false)
	}
	eventMetric = buildMetricEvent("TurbinePower", "Float64", "1.090002e+01")
	continuePipeline, _ = inf.TakeSample(ctx, eventMetric)
	if continuePipeline {
		t.Errorf("TakeSample() continuePipeline = %v, want %v", continuePipeline, false)
	}

	eventMetric = buildMetricEvent("Uint64Metric", "Uint64", "4")
	continuePipeline, _ = inf.TakeSample(ctx, eventMetric)
	if continuePipeline {
		t.Errorf("TakeSample() continuePipeline = %v, want %v", continuePipeline, false)
	}

	//After 2 secs, new sampling will start so we get the samples collected so far in the 1st 2 secs
	time.Sleep(2010 * time.Millisecond)
	eventMetric = buildMetricEvent("BoolMetric", "Bool", "false")
	_, samples := inf.TakeSample(ctx, eventMetric)
	if samples == nil {
		t.Errorf("TakeSample() expected samples, found nil")
	}
	if len(samples.([]*data.TSDataElement)) != 4 {
		t.Errorf("TakeSample() expected 4 samples of type []*data.TSDataElement) , found %v", samples)
	}

}

// Test GroupSamples
func TestMLBrokerInferencing_GroupSamples(t *testing.T) {

	inf, _, ctx := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)

	samples := getSamplesInSamplingInterval(t, inf, ctx)

	shouldContinue, groups := inf.GroupSamples(ctx, samples)
	if shouldContinue != true {
		t.Errorf("GroupSamples() shouldContinue expected: true, found %v", shouldContinue)
	}
	if groups == nil || groups.(map[string][]*data.TSDataElement) == nil {
		t.Errorf("GroupSamples() groups expected not nil, found nil")
	}
	actualGroup := groups.(map[string][]*data.TSDataElement)

	if len(actualGroup["AltaNS12"]) < 2 {
		t.Errorf("GroupSamples() expected > 2 samples of type []*data.TSDataElement) , found %d", len(actualGroup["AltaNS12"]))
	}

}

// Test ConvertToFeatures & CollectAndReleaseFeature together since the later needs data from earlier step
func TestMLBrokerInferencing_ConvertToFeaturesAndReleaseFeature(t *testing.T) {

	inf, _, ctx := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)
	inf.mlEventConfigs[0].StabilizationPeriodByCount = 2
	samples := getSamplesInSamplingInterval(t, inf, ctx)
	groups := make(map[string][]*data.TSDataElement)
	groups["AltaNS12"] = samples

	shouldContinue, featureGrps := inf.ConvertToFeatures(ctx, groups)
	if shouldContinue != true {
		t.Errorf("ConvertToFeatures() shouldContinue expected: true, found %v", shouldContinue)
	}
	if featureGrps == nil || featureGrps.(map[string][]data.InferenceData) == nil {
		t.Errorf("ConvertToFeatures() features expected valid feature groups, found nil")
	}
	features := featureGrps.(map[string][]data.InferenceData)
	if inputFeatures, ok := features["AltaNS12"]; ok {
		// Make sure the number of features is as per training data config
		// Expected feature count for 1-profile case
		expectedFeatureCount := len(inf.preprocessor.GetMLModelConfig().MLDataSourceConfig.FeaturesByProfile["WindTurbine"])
		if len(inputFeatures[0].Data[0]) != expectedFeatureCount {
			t.Errorf("ConvertToFeatures() expected featureSize with size %d,, found %v", expectedFeatureCount, len(inputFeatures[0].Data[0]))
		}
	} else {
		t.Errorf("ConvertToFeatures() expected feature group: AltaNS12, found: %v", features)
	}

	// Threshold is 2 to release for inferencing and event generation
	// CollectAndReleaseFeature->Test-1: Threshold of 2 not met, so not to continue the pipeline, collect data within the receiver
	shouldContinue, releasedCandidates := inf.CollectAndReleaseFeature(ctx, featureGrps)
	if shouldContinue != false {
		t.Errorf("CollectAndReleaseFeature() shouldContinue expected: false, found %v", shouldContinue)
	}
	if releasedCandidates != nil {
		t.Errorf("CollectAndReleaseFeature() releasedCandidates expected nil, found %v", featureGrps)
	}

	// CollectAndReleaseFeature->Test-2: Threshold of 2 met
	groups["AltaNS14"] = getSamplesInSamplingInterval(t, inf, ctx)
	shouldContinue, featureGrps = inf.ConvertToFeatures(ctx, groups)
	// Validate 2 groups
	if shouldContinue != true {
		t.Errorf("CollectAndReleaseFeature() shouldContinue expected: true, found %v", shouldContinue)
	}
	features = featureGrps.(map[string][]data.InferenceData)
	if len(features) != 2 {
		t.Errorf("CollectAndReleaseFeature() features Group count expected 2, found %v", features)
	}
	shouldContinue, featureGrps = inf.ConvertToFeatures(ctx, groups)
	if shouldContinue != true {
		t.Errorf("ConvertToFeatures() shouldContinue expected: true, found %v", shouldContinue)
	}
	if featureGrps == nil {
		t.Errorf("ConvertToFeatures() featureGrps expected not nil, found nil")
	}

	// Test for the case when there is no sample
	_, sample2 := inf.TakeSample(ctx, buildMetricEvent("TurbinePower", "Float64", "3.200002e+01"))
	groups["AltaNS12"] = sample2.([]*data.TSDataElement)
	groups["AltaNS14"] = sample2.([]*data.TSDataElement)
	shouldContinue, _ = inf.ConvertToFeatures(ctx, groups)
	if shouldContinue != false {
		t.Errorf("ConvertToFeatures() shouldContinue expected: false, found %v", shouldContinue)
	}
}

func TestMLBrokerInferencing_InferPredictionSeverity(t *testing.T) {
	// Mock MLBrokerInferencing with necessary dependencies
	infForAnomalyType, _, _ := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)

	//Test case: Anomaly found with Minor Severity
	stats, err := infForAnomalyType.InferPredictionSeverity(3.1, nil)
	assert.Nil(t, err)
	assert.NotNil(t, stats)
	assert.True(t, stats[0].Open)
	assert.Equal(t, models.SEVERITY_MAJOR, stats[0].LastSeverity)

	infForClassificationType, _, _ := BuildMLBrokerPipelineMocks(helpers.CLASSIFICATION_ALGO_TYPE)

	//Test case: Anomaly found with Minor Severity
	stats, err = infForClassificationType.InferPredictionSeverity(map[string]interface{}{"class": "Car", "confidence": 2.1}, nil)
	assert.Nil(t, err)
	assert.NotNil(t, stats)
	assert.True(t, stats[0].Open)
	assert.Equal(t, models.SEVERITY_MINOR, stats[0].LastSeverity)

	infForTimeseriesType, _, _ := BuildMLBrokerPipelineMocks(helpers.TIMESERIES_ALGO_TYPE)

	//Test case: Anomaly found with Minor Severity in the predicted features values
	stats, err = infForTimeseriesType.InferPredictionSeverity(map[string]interface{}{
		"predictions": []interface{}{
			[]interface{}{179878787897897.0, 10.1, 11.1},
			[]interface{}{179878787897898.0, 13.1, 14.1},
		},
		"min_boundary": []interface{}{
			[]interface{}{179878787897897.0, 10.0, 11.0},
			[]interface{}{179878787897898.0, 13.0, 14.0},
		},
		"max_boundary": []interface{}{
			[]interface{}{179878787897897.0, 10.5, 11.5},
			[]interface{}{179878787897898.0, 13.5, 14.5},
		},
	}, nil)
	assert.Nil(t, err)
	assert.NotNil(t, stats)
	assert.True(t, stats[0].Open)
	assert.Equal(t, models.SEVERITY_MINOR, stats[0].LastSeverity)

	infForRegressionType, _, _ := BuildMLBrokerPipelineMocks(helpers.REGRESSION_ALGO_TYPE)

	//Test case: Regression found with Minor Severity
	stats, err = infForRegressionType.InferPredictionSeverity(2.1, []interface{}{2.1, 10.0})
	assert.Nil(t, err)
	assert.NotNil(t, stats)
	assert.True(t, stats[0].Open)
	assert.Equal(t, models.SEVERITY_MINOR, stats[0].LastSeverity)
}

func TestMLBrokerInferencing_PublishEvent(t *testing.T) {

	anomalyEvent := models.HedgeEvent{
		Id:               "xxx",
		Class:            "OT_EVENT",
		EventType:        "ANOMALY",
		DeviceName:       "AltaNS12",
		Name:             "High Vibration",
		Msg:              "High Vibration detected",
		Severity:         "High",
		Profile:          "WindTurbine",
		SourceNode:       "clm-pun-xxxx",
		Status:           "Open",
		RelatedMetrics:   []string{"Vibration"},
		Thresholds:       map[string]interface{}{"Prediction": 10.0},
		ActualValues:     map[string]interface{}{"PredictionValue": 10.0},
		ActualValueStr:   "40",
		Unit:             "m/s2",
		Version:          0,
		AdditionalData:   nil,
		IsNewEvent:       true,
		IsRemediationTxn: false,
	}
	anomalyEvents := []models.HedgeEvent{anomalyEvent}
	inf, _, ctx := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)
	triggerURL := "http://hedge-event-publisher:48102/trigger"
	ctx.(*mocks.AppFunctionContext).On("ApplyValues", triggerURL).Return(triggerURL, nil)
	ctx.(*mocks.AppFunctionContext).On("MetricsManager").Return(nil)
	inf.SetHttpSender(triggerURL)
	_, err := inf.PublishEvents(ctx, anomalyEvents)
	if err == nil {
		t.Errorf("PublishEvent() expected error, found nil error")
	}
}

func buildMetricEvent(metricName string, valueType string, value string) dtos.Event {
	event := dtos.Event{
		Id:          "edgex-ev-id-01",
		DeviceName:  "AltaNS12",
		ProfileName: "WindTurbine",
		SourceName:  metricName,
		Origin:      1698236064346219000,
		Readings:    nil,
		Tags:        nil,
	}
	reading := dtos.BaseReading{
		ValueType:     valueType,
		Units:         "pascal",
		SimpleReading: dtos.SimpleReading{Value: value},
	}
	reading.ResourceName = event.SourceName
	reading.DeviceName = event.DeviceName
	reading.ProfileName = event.ProfileName
	reading.Origin = event.Origin + 10
	reading.Id = fmt.Sprintf("%s-rd", event.Id)

	event.Readings = []dtos.BaseReading{reading}
	tags := make(map[string]interface{})
	tags["model"] = "SXU980"
	event.Tags = tags
	return event
}

// Utility method to get a sample during a sampling interval, needed this since the pipeline code depends on outcome from earlier steps
func getSamplesInSamplingInterval(t *testing.T, inf *MLBrokerInferencing, ctx interfaces.AppFunctionContext) []*data.TSDataElement {

	event1 := buildMetricEvent("WindSpeed", "Float64", "1.090002e+01")
	event2 := buildMetricEvent("TurbinePower", "Float64", "3.200002e+01")
	event1.Readings = append(event1.Readings, event2.Readings...)
	inf.TakeSample(ctx, event1)
	shouldContinue, samples := inf.TakeSample(ctx, buildMetricEvent("WindSpeed", "Float64", "2.010002e+01"))
	t.Logf("Debug check before taking out the sample after waiting for 2 secs->shouldContinue: %v, samples: %v", shouldContinue, samples)

	//After 2 secs, new sampling will start so we get the samples collected so far in the 1st 2 secs
	time.Sleep(2010 * time.Millisecond)

	// The below is a dummy metric ingestion to drain out the previous metric samples, ingesting same data again to handle
	// test failures due to timing issue
	//
	_, samples = inf.TakeSample(ctx, event1)
	//_, samples = inf.TakeSample(ctx, buildMetricEvent("BoolMetric", "Bool", "false"))
	if samples == nil || len(samples.([]*data.TSDataElement)) < 2 {
		t.Errorf("data generation error using TakeSample, abort GroupSamples Test")
	}
	return samples.([]*data.TSDataElement)
}

func TestMLBrokerInferencing_getPredictions(t *testing.T) {
	inf, _, _ := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)
	response := map[string]interface{}{
		"AltaNS12": 3.2,
		"AltaNS14": 14.67,
	}
	mockBody, _ := json.Marshal(response)
	mockResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}

	type args struct {
		data bytes.Buffer
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr assert.ErrorAssertionFunc
	}{
		{"getPredictions - Passed", args{data: *bytes.NewBufferString(`{"input": "test data"}`)}, response, nil},
		{"getPredictions - Failed", args{data: *bytes.NewBufferString(`{"input": "test data"}`)}, nil, assert.Error},
		{"getPredictions - Failed1", args{data: *bytes.NewBufferString(`{"input": "test data"}`)}, nil, assert.Error},
		{"getPredictions - Failed2", args{data: *bytes.NewBufferString(`{"input": "test data"}`)}, nil, assert.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.name, "Failed1") {
				mockedHttpClient1 := &svcmocks.MockHTTPClient{}
				mockedHttpClient1.On("Do", mock.Anything).Return(nil, errors.New("mocked error"))
				inf.Client = mockedHttpClient1
			}
			if strings.Contains(tt.name, "Failed2") {
				mockResponse.StatusCode = http.StatusBadRequest
				mockedHttpClient1 := &svcmocks.MockHTTPClient{}
				mockedHttpClient1.On("Do", mock.Anything).Return(mockResponse, nil)
				inf.Client = mockedHttpClient1
			} else {
				mockedHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
				inf.Client = mockedHttpClient
			}

			got, err := inf.getPredictions(tt.args.data)
			if tt.wantErr != nil {
				if !tt.wantErr(t, err, fmt.Sprintf("getPredictions(%v)", tt.args.data)) {
					return
				}
			} else if err != nil {
				t.Fatalf("expected no error, but got: %v", err)
			}

			assert.Equalf(t, tt.want, got, "getPredictions(%v)", tt.args.data)
		})
	}
}

func TestMLBrokerInferencing_computeSeverityForPrediction(t *testing.T) {
	inf, _, _ := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)

	type args struct {
		thresholdDefinition config.ThresholdDefinition
		prediction          interface{}
	}
	args1 := args{
		thresholdDefinition: config.ThresholdDefinition{
			Operator:       config.BETWEEN_OPERATOR,
			LowerThreshold: 1.0,
			UpperThreshold: 5.0,
		},
		prediction: 3.0,
	}
	args2 := args{
		thresholdDefinition: config.ThresholdDefinition{
			Operator:       config.BETWEEN_OPERATOR,
			LowerThreshold: 1.0,
			UpperThreshold: 5.0,
		},
		prediction: 6.0,
	}
	args3 := args{
		thresholdDefinition: config.ThresholdDefinition{
			Operator:       config.EQUAL_TO_OPERATOR,
			ThresholdValue: 10.0,
		},
		prediction: 10.0,
	}
	args5 := args{
		thresholdDefinition: config.ThresholdDefinition{
			Operator:       config.GREATER_THAN_OPERATOR,
			ThresholdValue: 10.0,
		},
		prediction: 12.0,
	}
	args6 := args{
		thresholdDefinition: config.ThresholdDefinition{
			Operator:       config.LESS_THAN_OPERATOR,
			ThresholdValue: 10.0,
		},
		prediction: 8.0,
	}
	args7 := args{
		thresholdDefinition: config.ThresholdDefinition{
			Operator:       "UNKNOWN",
			ThresholdValue: 10.0,
		},
		prediction: 8.0,
	}

	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr assert.ErrorAssertionFunc
	}{
		{"computeSeverityForPrediction - Between operator with valid prediction in range", args1, true, assert.NoError},
		{"computeSeverityForPrediction - Between operator with prediction out of range", args2, false, assert.NoError},
		{"computeSeverityForPrediction - Equal to operator with float value", args3, true, assert.NoError},
		{"computeSeverityForPrediction - Greater than operator", args5, true, assert.NoError},
		{"computeSeverityForPrediction - Less than operator", args6, true, assert.NoError},
		{"computeSeverityForPrediction - Unknown operator", args7, false, assert.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := inf.computeSeverityForPrediction(tt.args.thresholdDefinition, tt.args.prediction)
			if !tt.wantErr(t, err, fmt.Sprintf("computeSeverityForPrediction(%v, %v)", tt.args.thresholdDefinition, tt.args.prediction)) {
				return
			}
			assert.Equalf(t, tt.want, got, "computeSeverityForPrediction(%v, %v)", tt.args.thresholdDefinition, tt.args.prediction)
		})
	}
}

func TestMLBrokerInferencing_buildAnomalyEvent(t *testing.T) {
	inf, _, _ := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)

	groupName := "Device-1"
	severityData := map[string]*EventBuilderData{
		groupName: {
			Tags: map[string]bool{
				"Overheating": true,
			},
			/*			Predictions: map[float64]bool{
						75.0: true,
						1.2:  false,
					},*/
			Data: [][]interface{}{
				{75.0, 1.2},
			},
		},
	}
	eventStats := EventStats{
		EventName:      "AnomalyDetected",
		EventMessage:   "High Temperature",
		LastSeverity:   models.SEVERITY_CRITICAL,
		RelatedMetrics: []string{"WindSpeed", "TurbinePower"},
		Thresholds:     map[string]interface{}{"Prediction": 1},
		PredLoss:       map[string]interface{}{"Prediction": 0.95},
		Open:           true,
	}

	result := inf.buildEvent(groupName, severityData[groupName], eventStats)

	currentTime := time.Now().Unix() * 1000
	assert.Equal(t, groupName, result.DeviceName)
	assert.Equal(t, "AnomalyDetected", result.Name)
	assert.Equal(t, "Device-1:High Temperature", result.Msg)
	assert.Equal(t, models.SEVERITY_CRITICAL, result.Severity)
	assert.Equal(t, "Open", result.Status)
	assert.WithinDuration(t, time.Unix(0, result.Created*int64(time.Millisecond)), time.Unix(0, currentTime*int64(time.Millisecond)), 100*time.Millisecond)
	assert.Equal(t, "WindTurbineAnomaly_Device-1", result.CorrelationId)

	// Verify the additional data
	assert.Equal(t, "true", result.AdditionalData["Overheating"])
	assert.Equal(t, "75.000000", result.AdditionalData["WindTurbine#WindSpeed"])
	assert.Equal(t, "1.200000", result.AdditionalData["WindTurbine#TurbinePower"])
}

func TestMLBrokerInferencing_checkIfNewAnomalyEvent(t *testing.T) {
	inf, _, _ := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)
	groupName := "device-1"

	// Case 1: New anomaly event (no previous anomaly for this group)
	currentEventStat := &EventStats{
		Open:         true,
		LastSeverity: "High",
	}
	generateEvent := inf.checkIfNewEvent(groupName, currentEventStat)
	assert.True(t, generateEvent, "Expected new anomaly event to be generated")

	// Case 2: No new anomaly, same severity, and status
	previousEventStat := &EventStats{
		Open:         true,
		LastSeverity: "High",
	}
	inf.eventStatByGroup[groupName] = *previousEventStat
	currentEventStat2 := &EventStats{
		Open:         true,
		LastSeverity: "High",
	}
	generateEvent = inf.checkIfNewEvent(groupName, currentEventStat2)
	assert.False(t, generateEvent, "Expected no new anomaly event to be generated")

	// Case 3: Severity changed, so a new event should be generated
	currentEventStat3 := &EventStats{
		Open:         true,
		LastSeverity: "Medium",
	}
	generateEvent = inf.checkIfNewEvent(groupName, currentEventStat3)
	assert.True(t, generateEvent, "Expected new anomaly event to be generated due to severity change")

	// Case 4: Anomaly is now closed, so a new event should be generated
	currentEventStat4 := &EventStats{
		Open:         false,
		LastSeverity: "Medium",
	}
	generateEvent = inf.checkIfNewEvent(groupName, currentEventStat4)
	assert.True(t, generateEvent, "Expected new anomaly event to be generated due to anomaly closure")
}

func TestMLBrokerInferencing_buildAndPublishMLPredictions(t *testing.T) {
	inf, _, _ := BuildMLBrokerPipelineMocks(helpers.CLASSIFICATION_ALGO_TYPE)
	groupName := "TestDevice-1"
	inferences := map[string]interface{}{
		groupName: map[string]interface{}{
			"class":      "2MonthToBreakdown",
			"confidence": 0.75,
		},
	}

	inputData := map[string]data.InferenceData{
		groupName: {
			Devices: []string{"Device1", "Device2"},
			Data: [][]interface{}{
				{75.0, 1.2},
				{80.0, 1.5},
			},
		},
	}

	// Mock the MQTTSend function to simulate successful sending
	mockedMqttSender.On("MQTTSend", mock.Anything, mock.Anything).Return(true, nil)
	inf.mqttSender = mockedMqttSender
	err := inf.buildAndPublishMLPredictions(nil, inferences, inputData)
	assert.NoError(t, err, "Expected no error during execution")

	// Mock the MQTTSend function to simulate failed sending
	mockedMqttSender1 := &svcmocks.MockMqttSender{}
	mockedMqttSender1.On("MQTTSend", mock.Anything, mock.Anything).Return(false, errors.New("mocked mqtt error"))
	inf.mqttSender = mockedMqttSender1
	err = inf.buildAndPublishMLPredictions(nil, inferences, inputData)
	assert.Error(t, err, "Expected no error during execution")
}

func TestMLBrokerInferencing_AccumulateMultipleTimeseriesFeaturesSets(t *testing.T) {
	inf, _, _ := BuildMLBrokerPipelineMocks(helpers.TIMESERIES_ALGO_TYPE)
	u := utils.NewApplicationServiceMock(map[string]string{})
	appCtx = &mocks.AppFunctionContext{}
	appCtx.On("LoggingClient").Return(u.AppService.LoggingClient())

	type args struct {
		ctx    interfaces.AppFunctionContext
		params interface{}
	}
	args1 := args{
		ctx: appCtx,
		params: map[string][]data.InferenceData{
			"Group1": {
				{
					GroupName: "Group1",
					Data: [][]interface{}{
						{75.0, 1.2},
						{80.0, 1.5},
					},
				},
			},
		},
	}
	args2 := args{
		ctx:    appCtx,
		params: map[string][]data.InferenceData{},
	}
	wantResult1 := map[string]data.InferenceData{
		"Group1": {
			GroupName: "Group1",
			Data: [][]interface{}{
				{75.0, 1.2},
				{80.0, 1.5},
			},
		},
	}
	tests := []struct {
		name                 string
		args                 args
		wantContinuePipeline bool
		wantResult           interface{}
	}{
		{"AccumulateMultipleTimeseriesFeaturesSets - TimeseriesAlgoType", args1, true, wantResult1},
		{"AccumulateMultipleTimeseriesFeaturesSets - NonTimeseriesAlgoType", args2, true, map[string][]data.InferenceData{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotContinuePipeline bool
			var gotResult interface{}
			if strings.Contains(tt.name, "NonTimeseriesAlgoType") {
				inf, _, _ := BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)
				gotContinuePipeline, gotResult = inf.AccumulateMultipleTimeseriesFeaturesSets(tt.args.ctx, tt.args.params)
			} else {
				gotContinuePipeline, gotResult = inf.AccumulateMultipleTimeseriesFeaturesSets(tt.args.ctx, tt.args.params)
			}
			assert.Equalf(t, tt.wantContinuePipeline, gotContinuePipeline, "AccumulateMultipleTimeseriesFeaturesSets(%v, %v)", tt.args.ctx, tt.args.params)
			assert.Equalf(t, tt.wantResult, gotResult, "AccumulateMultipleTimeseriesFeaturesSets(%v, %v)", tt.args.ctx, tt.args.params)
		})
	}
}

func TestMLBrokerInferencing_GetAndPublishPredictions(t *testing.T) {
	inf, _, _ = BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)
	u := utils.NewApplicationServiceMock(map[string]string{})
	appCtx = &mocks.AppFunctionContext{}
	appCtx.On("LoggingClient").Return(u.AppService.LoggingClient())
	appCtx.On("PipelineId").Return("12345")

	mockedPreProcessor.On("GetMLAlgorithmDefinition").Return(inf.preprocessor.GetMLAlgorithmDefinition())
	mockedPreProcessor.On("GetMLModelConfig").Return(inf.preprocessor.GetMLModelConfig())
	mockedPreProcessor.On("AccumulateMultipleTimeseriesFeaturesSets", mock.Anything).Return(true, map[string]data.InferenceData{
		"AltaNS14": {
			GroupName: "AltaNS14",
			Data: [][]interface{}{
				{75.0, 1.2},
				{80.0, 1.5},
			},
		},
	})
	inf.preprocessor = mockedPreProcessor

	mockedMqttSender.On("MQTTSend", mock.Anything, mock.Anything).Return(true, nil)
	inf.mqttSender = mockedMqttSender

	response := map[string]interface{}{
		"AltaNS12__0__0": 1.2,
		"AltaNS14__0__0": 4.67,
	}
	mockBody, _ := json.Marshal(response)

	type args struct {
		ctx    interfaces.AppFunctionContext
		params interface{}
	}
	setupMock1 := func() {
		val1 := false
		inf.preprocessor.GetMLAlgorithmDefinition().PublishPredictionsRequired = &val1
		inf.preprocessor.GetMLAlgorithmDefinition().AutoEventGenerationRequired = &val1
		mockResponse = &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
			Header:     make(http.Header),
		}
		mockedHttpClient1 := &svcmocks.MockHTTPClient{}
		mockedHttpClient1.On("Do", mock.Anything).Return(mockResponse, nil)
		inf.Client = mockedHttpClient1
	}
	setupMock2 := func() {
		val1 := true
		val2 := false
		inf.preprocessor.GetMLAlgorithmDefinition().PublishPredictionsRequired = &val1
		inf.preprocessor.GetMLAlgorithmDefinition().AutoEventGenerationRequired = &val2
		mockResponse = &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
			Header:     make(http.Header),
		}
		mockedHttpClient2 := &svcmocks.MockHTTPClient{}
		mockedHttpClient2.On("Do", mock.Anything).Return(mockResponse, nil)
		inf.Client = mockedHttpClient2
	}

	setupMock3 := func() {
		val1 := true
		val2 := false
		inf.preprocessor.GetMLAlgorithmDefinition().PublishPredictionsRequired = &val2
		inf.preprocessor.GetMLAlgorithmDefinition().AutoEventGenerationRequired = &val1
		mockResponse = &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
			Header:     make(http.Header),
		}
		mockedHttpClient3 := &svcmocks.MockHTTPClient{}
		mockedHttpClient3.On("Do", mock.Anything).Return(mockResponse, nil)
		inf.Client = mockedHttpClient3
	}

	args1 := args{
		ctx: appCtx,
		params: map[string]data.InferenceData{
			"AltaNS14__0__0": {
				GroupName: "AltaNS14",
				Data: [][]interface{}{
					{75.0, 1.2},
					{80.0, 1.5},
				},
				Tags: map[string]interface{}{
					"location": "warehouse",
				},
			},
			"AltaNS12__0__0": {
				GroupName: "AltaNS12",
				Data: [][]interface{}{
					{81.0, 1.6},
				},
				Tags: map[string]interface{}{
					"location": "warehouse",
				},
			},
		},
	}
	args2 := args{
		ctx:    appCtx,
		params: nil,
	}
	timestamp := time.Now().Unix() * 1000
	expectedEvents := []models.HedgeEvent{
		{
			Id:         "",
			Class:      "OT_EVENT",
			EventType:  "Anomaly",
			DeviceName: "AltaNS14",
			Name:       "WindTurbineAnomaly:WindTurbineAnomalyEvent",
			Msg:        "AltaNS14:Anomaly detected",
			Severity:   "CRITICAL",
			Priority:   "",
			Profile:    "",
			SourceNode: "",
			Status:     "Open",
			RelatedMetrics: []string{
				"WindTurbine#WindSpeed",
				"WindTurbine#TurbinePower",
			},
			Thresholds: map[string]interface{}{
				"Prediction": config.ThresholdDefinition{
					Label:          "Prediction",
					Operator:       "GREATER_THAN",
					ThresholdValue: 4,
				},
			},
			ActualValues: map[string]interface{}{
				"Prediction": 4.67,
			},
			ActualValueStr: "",
			Unit:           "",
			Location:       "",
			Version:        0,
			AdditionalData: map[string]string{
				"WindTurbine#TurbinePower": "1.200000",
				"WindTurbine#WindSpeed":    "75.000000",
				"warehouse":                "true",
			},
			EventSource:   "HEDGE",
			CorrelationId: "WindTurbineAnomaly_AltaNS14",
			Labels:        []string(nil),
			Remediations:  []models.Remediation(nil),
			Created:       timestamp,
			Modified:      timestamp,
			IsNewEvent:    false,
		},
	}

	tests := []struct {
		name                 string
		args                 args
		setupMock            func()
		wantContinuePipeline bool
		wantResult           interface{}
	}{
		{"GetAndPublishPredictions - Valid predictions with AutoEventGenerationRequired", args1, setupMock1, false, nil},
		{"GetAndPublishPredictions - PublishPredictionsRequired", args1, setupMock2, false, nil},
		{"GetAndPublishPredictions - AutoEventGenerationRequired with Anomalies", args1, setupMock3, true, expectedEvents},
		{"GetAndPublishPredictions - Params is nil", args2, nil, false, errors.New("no event received")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupMock != nil {
				tt.setupMock()
			}
			gotContinuePipeline, gotResult := inf.GetAndPublishPredictions(tt.args.ctx, tt.args.params)
			assert.Equalf(t, tt.wantContinuePipeline, gotContinuePipeline, "GetAndPublishPredictions(%v, %v)", tt.args.ctx, tt.args.params)

			if gotResult != nil {
				gotResultWithFixedTimestamps, ok := gotResult.([]models.HedgeEvent)
				if ok {
					for i, _ := range gotResultWithFixedTimestamps {
						gotResultWithFixedTimestamps[i].Created = timestamp
						gotResultWithFixedTimestamps[i].Modified = timestamp
					}
					gotResult = gotResultWithFixedTimestamps
				}
			}

			if tt.name == "GetAndPublishPredictions - AutoEventGenerationRequired with Anomalies" {
				// Check that there is one open and one closed event
				events, _ := gotResult.([]models.HedgeEvent)
				assert.Equal(t, 2, len(events))
				// Commented out since the order is events is not guaranted for now
				//assert.Equal(t, events[0].Status, "Closed")
				//assert.Equal(t, events[1].Status, "Open")
			} else {
				assert.Equalf(t, tt.wantResult, gotResult, "GetAndPublishPredictions(%v, %v)", tt.args.ctx, tt.args.params)
			}

		})
	}
}

func TestMLBrokerInferencing_getOutputFeatureFromConfig(t *testing.T) {
	mlModelConfig1 := &config.MLModelConfig{
		MLDataSourceConfig: config.MLModelDataConfig{
			FeaturesByProfile: map[string][]config.Feature{
				"WindTurbine": {
					{Type: "METRIC", Name: "WindSpeed", IsInput: true, IsOutput: false},
					{Type: "METRIC", Name: "TurbinePower", IsInput: true, IsOutput: true},
				},
			},
		},
	}
	mlModelConfig2 := &config.MLModelConfig{
		MLDataSourceConfig: config.MLModelDataConfig{
			FeaturesByProfile: map[string][]config.Feature{
				"WindTurbine": {
					{Type: "METRIC", Name: "WindSpeed", IsInput: true, IsOutput: false},
					{Type: "METRIC", Name: "TurbinePower", IsInput: true, IsOutput: false},
				},
			},
		},
	}
	emptyFeatures := &config.MLModelConfig{
		MLDataSourceConfig: config.MLModelDataConfig{
			FeaturesByProfile: map[string][]config.Feature{},
		},
	}

	mockedPreProcessor1 := &helpersmocks.MockPreProcessorInterface{}
	mockedPreProcessor1.On("GetMLModelConfig").Return(mlModelConfig1)
	mockedPreProcessor2 := &helpersmocks.MockPreProcessorInterface{}
	mockedPreProcessor2.On("GetMLModelConfig").Return(mlModelConfig2)
	mockedPreProcessorEmpty := &helpersmocks.MockPreProcessorInterface{}
	mockedPreProcessorEmpty.On("GetMLModelConfig").Return(emptyFeatures)

	tests := []struct {
		name           string
		preprocessor   helpers.PreProcessorInterface
		expectedOutput string
	}{
		{"getOutputFeatureFromConfig - Passed (single output feature)", mockedPreProcessor1, "WindTurbine#TurbinePower"},
		{"getOutputFeatureFromConfig - Passed (no output feature in any profile)", mockedPreProcessor2, ""},
		{"getOutputFeatureFromConfig - Passed (empty FeaturesByProfile)", mockedPreProcessorEmpty, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inf := MLBrokerInferencing{
				preprocessor: tt.preprocessor,
			}
			outputFeature := inf.getOutputFeatureFromConfig()
			assert.Equal(t, tt.expectedOutput, outputFeature)
		})
	}
}

func TestMLBrokerInferencing_getOutputFeatureValue(t *testing.T) {
	testFeatureMap := map[string]int{
		"Feature1": 0,
		"Feature2": 1,
		"Feature3": 2,
	}
	testMLModelConfig := &config.MLModelConfig{
		MLDataSourceConfig: config.MLModelDataConfig{
			FeatureNameToColumnIndex: testFeatureMap,
		},
	}
	mockPreprocessor := &helpersmocks.MockPreProcessorInterface{}
	mockPreprocessor.On("GetMLModelConfig").Return(testMLModelConfig)
	t.Run("getOutputFeatureValue - Passed (valid feature and input)", func(t *testing.T) {
		inf := MLBrokerInferencing{
			preprocessor: mockPreprocessor,
		}
		inputData := []interface{}{10.5, 20.3, 30.7}
		expected := 10.5
		value, err := inf.getOutputFeatureValue("Feature1", inputData)

		assert.NoError(t, err)
		assert.Equal(t, expected, value)
	})
	t.Run("getOutputFeatureValue - Passed (valid feature at different index)", func(t *testing.T) {
		inf := MLBrokerInferencing{
			preprocessor: mockPreprocessor,
		}
		inputData := []interface{}{10.5, 20.3, 30.7}
		expected := 30.7
		value, err := inf.getOutputFeatureValue("Feature3", inputData)

		assert.NoError(t, err)
		assert.Equal(t, expected, value)
	})
	t.Run("getOutputFeatureValue - Failed (feature not found)", func(t *testing.T) {
		inf := MLBrokerInferencing{
			preprocessor: mockPreprocessor,
		}
		inputData := []interface{}{10.5, 20.3, 30.7}
		expectedErr := "feature name 'FeatureX' not found in map"
		_, err := inf.getOutputFeatureValue("FeatureX", inputData)

		assert.Error(t, err)
		assert.EqualError(t, err, expectedErr)
	})
	t.Run("getOutputFeatureValue - Failed (index out of bounds)", func(t *testing.T) {
		inf := MLBrokerInferencing{
			preprocessor: mockPreprocessor,
		}
		inputData := []interface{}{10.5}
		expectedErr := "index 1 for feature 'Feature2' is out of bounds"
		_, err := inf.getOutputFeatureValue("Feature2", inputData)

		assert.Error(t, err)
		assert.EqualError(t, err, expectedErr)
	})
	t.Run("getOutputFeatureValue - Failed (invalid inputData type)", func(t *testing.T) {
		inf := MLBrokerInferencing{
			preprocessor: mockPreprocessor,
		}
		inputData := []int{10, 20, 30} // invalid type
		expectedErr := "inputData is not of type []interface{}, got []int"
		_, err := inf.getOutputFeatureValue("Feature1", inputData)

		assert.Error(t, err)
		assert.EqualError(t, err, expectedErr)
	})
	t.Run("getOutputFeatureValue - Failed (output feature not float64)", func(t *testing.T) {
		inf := MLBrokerInferencing{
			preprocessor: mockPreprocessor,
		}
		inputData := []interface{}{"stringValue", 20.3, 30.7} // invalid value at index 0
		expectedErr := "output feature value is not of type float64, got string"
		_, err := inf.getOutputFeatureValue("Feature1", inputData)

		assert.Error(t, err)
		assert.EqualError(t, err, expectedErr)
	})
}
