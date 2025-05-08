/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"fmt"
	"strings"
)

const (
	BETWEEN_OPERATOR           = "BETWEEN"
	EQUAL_TO_OPERATOR          = "EQUAL_TO"
	GREATER_THAN_OPERATOR      = "GREATER_THAN"
	LESS_THAN_OPERATOR         = "LESS_THAN"
	TIMESTAMP_FEATURE_COL_NAME = "Timestamp"
)

type MLAlgorithmDefinition struct {
	Name                     string `json:"name" validate:"required,max=200,matchRegex=^[a-zA-Z0-9][a-zA-Z0-9_-]*$"` // Algo name
	Description              string `json:"description"`
	Type                     string `json:"type"`
	Enabled                  bool   `json:"enabled"`                  // true, false
	OutputFeaturesPredefined bool   `json:"outputFeaturesPredefined"` // true, false
	IsTrainingExternal       bool   `json:"isTrainingExternal,omitempty"`
	IsDeploymentExternal     bool   `json:"isDeploymentExternal,omitempty"`
	AllowDeploymentAtEdge    bool   `json:"allowDeploymentAtEdge" default:"true"`
	RequiresDataPipeline     bool   `json:"requiresDataPipeline"`
	PredictionName           string `json:"predictionName,omitempty"`
	// Whether prediction is real time or is it scheduled, we might need scheduled in case of scheduled to handle potential compute optimization
	IsPredictionRealTime bool `json:"isPredictionRealTime"`
	// Optional and can be overridden at the time of deployment in MLModelDeployment
	DefaultPredictionEndpointURL string `json:"defaultPredictionEndpointUrl"`
	// DefaultPredictionPort is allocated at the time of creating the algo name
	// Allocation is done after scanning the existing algos and providing the next available number
	DefaultPredictionPort        int64  `json:"defaultPredictionPort"`
	TrainerImagePath             string `json:"trainerImagePath"`
	TrainerImageDigest           string `json:"trainerImageDigest,omitempty"`
	DeprecatedTrainerImageDigest string `json:"deprecatedTrainerImageDigest,omitempty"`
	// Prediction image path & Digest
	PredictionImagePath             string                    `json:"predictionImagePath"`
	PredictionImageDigest           string                    `json:"predictionImageDigest,omitempty"`
	DeprecatedPredictionImageDigest string                    `json:"deprecatedPredictionImageDigest,omitempty"`
	PredictionPayloadTemplate       PredictionPayloadTemplate `json:"predictionPayloadTemplate"`
	// We assume that the output of prediction will be a map, so we can scan the keys and get the values from prediction
	PredictionOutputKeys []string `json:"predictionOutputKeys"`
	// User defined parameter map ( eg HighSeverityEventLowerBound)
	ThresholdUserParameterMap map[string]interface{} `json:"thresholdUserParameters,omitempty"`
	// For AutoEncoder running at edge we need AutoEventGeneration while for simulation we might not need it
	AutoEventGenerationRequired *bool `json:"autoEventGenerationRequired"`
	PublishPredictionsRequired  *bool `json:"publishPredictionsRequired"`
	// Custom parameters that could be provided for using in the input payload for prediction endpoint
	HyperParameters map[string]interface{} `json:"hyperParameters"`
	// The below is required to support time-series prediction for simulation, possibly the Algo (GluonTS uses this somewhere)
	TimeSeriesAttributeRequired bool `json:"timeStampAttributeRequired,omitempty"`
	// The below is required to support time-series prediction
	GroupByAttributesRequired bool `json:"groupByAttributesRequired,omitempty"`
	// Is the algorithm out of the box (builtin within hedge product package)
	IsOotb         bool   `json:"isOotb" default:"false"`
	LastApprovedBy string `json:"lastApprovedBy,omitempty"`
}

type PredictionPayloadTemplate struct {
	Name          string `json:"name,omitempty"`
	PayloadSample string `json:"payloadSample,omitempty"`
	Template      string `json:"template"`
}

func (algo MLAlgorithmDefinition) BuildDefaultPredictionEndPointUrl() string {
	if strings.HasPrefix(algo.DefaultPredictionEndpointURL, "http") {
		return algo.DefaultPredictionEndpointURL
	}
	containerName := strings.ToLower(algo.Name)
	if !strings.HasPrefix(algo.DefaultPredictionEndpointURL, "/") {
		algo.DefaultPredictionEndpointURL = fmt.Sprintf("/%s", algo.DefaultPredictionEndpointURL)
	}
	if algo.DefaultPredictionPort != 0 {
		return fmt.Sprintf("http://%s:%d%s", containerName, algo.DefaultPredictionPort, algo.DefaultPredictionEndpointURL)
	}
	return fmt.Sprintf("http://%s%s", containerName, algo.DefaultPredictionEndpointURL)
}

// deviceSelectionMethod // Auto:-> Random, every nth, Stratification , Future

/*
If Grouping by Profile/DeviceName,

  GroupBy: DeviceName
  GroupByFilter :
            metaData[]
			Profiles[profile1]

  InputFeature_Metrics: [Profile1(m11,m12,m13)]
  InputFeatures_Metadata: [profile1(meta1,meta2)]


  TrainingDataDeviceScope:
  [
  	[device1, device2, device4] // Random sample of devices,
  ]

If Grouping by MetaData => List of MetaData ( PlantName, Area)

   GroupBy: MetaData
   GroupByFilter: metaData[metaData1, metadata2],
                  Profiles[profile1, profile2, profile3]

  InputFeature_Metrics: [Profile1(m11,m12,m13), Profile2(m21,m22), Profile3(m31,m32)]
  InputFeatures_Metadata: [profile1(meta11,meta12), Profile2(meta21, meta22)]

 TrainingDataDeviceScope: Manual selection not applicable since it is lot of manual effort and discipline


 Example:
   plant1, area1, d11(m11), d21(m21), d31(m31)
   plant2, area2, d12(m12), d22(m22), d32(m32)

*/

const (
	ENTITY_TYPE_PROFILE      = "PROFILE"
	ENTITY_TYPE_DEVICELABELS = "DEVICE_LABELS"
	ENTITY_TYPE_DIGITAL_TWIN = "DIGITAL_TWIN"
)

type MLModelConfig struct {
	Name        string `json:"name" validate:"required,max=200,matchRegex=^[a-zA-Z0-9][a-zA-Z0-9_-]*$"`
	Version     string `json:"version,omitempty"` // Only for readability
	Description string `json:"description,omitempty,excludesall=!@#?,max=300"`
	// Refers to name in MLAlgorithmDefinition
	MLAlgorithm        string            `json:"mlAlgorithm" validate:"excludesall=!@#?,max=300"`
	MLDataSourceConfig MLModelDataConfig `json:"mlDataSourceConfig" validate:"required"`
	// default message that can have placeholders for runtime parameters from device labels/tags {{.deviceName}}
	Message string `json:"message"`
	Enabled bool   `json:"enabled,omitempty" default:"true"` //true, false

	// ModelConfigVersion is the running counter that should get incremented after feature definitions are changed with trained ml_model even if the mlModel config is changed later
	ModelConfigVersion int64 `json:"modelConfigVersion,omitempty"`

	// LocalModelStorageDir and TrainingImagePath are part of MLModel that is created after training,
	// however, we still keep here since the image that is used for training might get updated after the training
	// We can store MLAlgoConfig also in mlModel, but we will need to take special care so we don't store in redis
	LocalModelStorageDir string `json:"localModelStorageDir,omitempty"` // No need to expose outside, Will be set after ml_model downloaded
	TrainedModelCount    int    `json:"trainedModelCount,omitempty"`
	Modified             int64  `json:"modified,omitempty"`
	MLAlgorithmType      string `json:"mlAlgorithmType,omitempty"`
}

type MLModelConfigSummary struct {
	Name              string         `json:"name"`
	Description       string         `json:"description,omitempty"`
	MLAlgorithm       string         `json:"mlAlgorithm"`
	Enabled           bool           `json:"enabled,omitempty" default:"true"` //true, false
	TrainingJobStatus string         `json:"trainingJobStatus"`
	Deployments       map[string]int `json:"deployments"`
	MLAlgorithmType   string         `json:"mlAlgorithmType"`
}

// We will assume only timeseries data as input for prediction.
// We have allowed ExternalFeatures for now assuming this will be output feature eg labelled output to train the mode
// Adding other data sources were becoming very complicated since we need to account for querying, joining during
// training and then generating dynamic data pipeline
type MLModelDataConfig struct {
	// Feature list in here while datasource to pull from in MLDataSourceConfig
	// Helps with filtering and in UI to select the metrics from
	// Starting entity to help users to start configuring the features for training or to use to filter our the topic during prediction
	// EntityType can be deviceName, labels or DigitalTwin
	EntityType string `json:"entityType,omitempty" default:"Profile"`
	// If EntityType=digitalTwin, then GroupOrJoinKeys=twinId/NA since we get explicit deviceNames
	// if EntityType=deviceLabels, GroupOrJoinKeys = [PlantArea]
	// if EntityType=deviceName, GroupOrJoinKeys=[deviceName]
	GroupOrJoinKeys []string `json:"groupOrJoinKeys"`
	// If EntityType=digitalTwin, then EntityName=WindturbineTwin,
	// if EntityType=deviceLabels, EntityName = PlantArea, ( to be derived from GroupOrJoinKeys =>Concate if multiple
	// if EntityType=deviceName, EntityName=profileName => derive when only one profile
	EntityName string `json:"entityName,omitempty"`
	// Experimental start, necessitated after checking NG data
	SupportSlidingWindow bool `json:"supportSlidingWindow" default:"true"`

	PrimaryProfile                 string                      `json:"primaryProfile"`
	FeaturesByProfile              map[string][]Feature        `json:"featuresByProfile" validate:"required"`
	DeviceSelectionFilterByProfile map[string]FilterDefinition `json:"deviceSelectionFilterByProfile,omitempty"`
	// Below is placeholder to get labelled/derived data, so need to also add transform function if it is input feature
	ExternalFeatures         []Feature      `json:"externalFeatures,omitempty"`
	FeatureNameToColumnIndex map[string]int `json:"featureNameToColumnIndex,omitempty"`
	// The below 2 OutputPredictionCount and InputContextCount are required for timeseries
	OutputPredictionCount      int                        `json:"outputPredictionCount,omitempty"`
	InputContextCount          int                        `json:"inputContextCount,omitempty"`
	TrainingDataSourceConfig   TrainingDataSourceConfig   `json:"trainingDataSourceConfig"`
	PredictionDataSourceConfig PredictionDataSourceConfig `json:"predictionDataSourceConfig"`
}

type DigitalTwinParameterType int

const (
	OperatingMetric DigitalTwinParameterType = iota // 0
	Input                                           // 1
	ExternalInput
	Output // 2
)

type Feature struct {
	Type string `json:"type" default:"METRIC"` // METRIC, CATEGORICAL, TIMESTAMP
	Name string `json:"name"`
	//Units              string `json:"units,omitempty"`
	IsInput bool `json:"isInput"`
	// In case of timeseries forcasting the same metric can be input as well as output
	IsOutput bool `json:"isOutput" default:"false"`
	// if external or user configurable, needs to be provided by user after downloading and uploading in case of training
	FromExternalDataSource bool `json:"fromExternalDataSource" default:"false"`
	// In case there are multiple devices for the same profile, we need a way to decide which device to consider
	// This applies when EntityType=deviceLabels or EntityType=digitalTwin
	// The value part of the FilterDefinition will be used to name the column to make it unique
	// FilterDefinition is to be provided by digital twin author or ML user when we create MLModel Configuration
	// Not an array right now to make it simple
	DeviceSelector *FilterDefinition `json:"deviceSelector,omitempty"`
	// INPUT, OUTPUT, OPERATING_METRICS
	DigitalTwinParameterType DigitalTwinParameterType `json:"digitalTwinParameterType,omitempty"`
}

type PredictionDataSourceConfig struct {
	// Valid streamType is redis or MQTT
	StreamType string             `json:"streamType" default:"redis"`
	Filters    []FilterDefinition `json:"filters,omitempty"`
	// Need to keep one of PredictionRateSecs or SamplingIntervalSecs
	//PredictionRateSecs          int64              `json:"predictionRateSecs" validate:"gt=0,required" default:"30"`
	SamplingIntervalSecs  int64  `json:"samplingIntervalSecs,omitempty"`
	TopicName             string `json:"topicName"`
	PredictionEndPointURL string `json:"predictionEndPointURL"`
	//TimeseriesFeaturesSetsCount int    `json:"timeseriesFeaturesSetsCount"`
}

type TrainingDataSourceConfig struct {
	Filters                        []FilterDefinition `json:"filters,omitempty"`
	DataCollectionTotalDurationSec int64              `json:"dataCollectionTotalDurationSec,omitempty" validate:"gt=0,required"`
	SamplingIntervalSecs           int64              `json:"samplingIntervalSecs" validate:"gt=0,required"`
}

type FilterDefinition struct {
	Label    string `json:"label" validate:"required"`
	Operator string `json:"operator" validate:"required"`
	Value    string `json:"value" validate:"required"` // profile1: [meta1, meta2], profile2: [meta21, meta22]
}

type MLModelThresholdConfig struct {
	Name string `json:"name"`
	// what is the type and how do we use it?
	ThresholdParamValue map[string]interface{} `json:"threshold"`
}

type MLConfig struct {
	MLModelConfig    MLModelConfig         `json:"mlModelConfig"`
	MLAlgoDefinition MLAlgorithmDefinition `json:"mlAlgoDefinition"`
}

type MLConfigValidation struct {
	ExistingMLConfig MLConfig `json:"existingMLConfig"`
	NewMLConfig      MLConfig `json:"newMLConfig"`
}

// The below will get replaced by the MLEventTemplate
type MLEventConfig struct {
	MLAlgorithm                string      `json:"mlAlgorithm" validate:"required,max=200,matchRegex=^[a-zA-Z0-9-]+$"`
	MlModelConfigName          string      `json:"mlModelConfigName" validate:"required,max=200,matchRegex=^[a-zA-Z0-9_-]+$"` // foreign key refers to trainingConfig against which the training is being performed
	EventName                  string      `json:"eventName" validate:"required,max=200,matchRegex=^[a-zA-Z0-9_-]+$"`
	Description                string      `json:"description" validate:"required,max=200,eventMessageRegex"`
	Conditions                 []Condition `json:"conditions"`
	StabilizationPeriodByCount int         `json:"stabilizationPeriodByCount" validate:"gt=0,required"`
	MLAlgorithmType            string      `json:"mlAlgorithmType,omitempty"`
}

type Condition struct {
	ThresholdsDefinitions []ThresholdDefinition `json:"thresholdsDefinitions" validate:"required"` // All the parts of condition
	SeverityLevel         string                `json:"severityLevel" validate:"required"`         // CRITICAL, MAJOR, MINOR
}

type ThresholdDefinition struct {
	Label          string      `json:"label,omitempty"`              // Anomaly: "Prediction", Classification: class name (categorical value), Timeseries: metric name
	Operator       string      `json:"operator" validate:"required"` // BETWEEN, EQUAL_TO, GREATER_THAN, LESS_THAN
	ThresholdValue interface{} `json:"thresholdValue,omitempty"`     // string for EQUAL_TO, float64 for GREATER_THAN, LESS_THAN
	LowerThreshold float64     `json:"lowerThreshold,omitempty"`     // for BETWEEN operator: >=
	UpperThreshold float64     `json:"upperThreshold,omitempty"`     // for BETWEEN operator: <
}

// NewDefaultEventConfig A default EventConfig is created when a new model is generated first time (v1), so the below constructor
// We set it for "Anomaly" algorithm type only; for the rest, we expect the user to configure using UI.
func NewDefaultAnomalyEventConfig(mlAlgorithm string, mlModelConfigName string) MLEventConfig {
	var conditions []Condition
	conditions = []Condition{
		{
			ThresholdsDefinitions: []ThresholdDefinition{
				{
					Label:          "Prediction",
					Operator:       GREATER_THAN_OPERATOR,
					ThresholdValue: 3.0,
				},
			},
			SeverityLevel: "MAJOR",
		},
	}

	eventCfg := MLEventConfig{
		MLAlgorithm:                mlAlgorithm,
		MlModelConfigName:          mlModelConfigName,
		EventName:                  mlModelConfigName + "Event",
		Description:                "Anomaly detected",
		Conditions:                 conditions,
		StabilizationPeriodByCount: 2,
		MLAlgorithmType:            "Anomaly",
	}

	return eventCfg
}
