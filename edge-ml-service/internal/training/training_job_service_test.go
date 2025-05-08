/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redsync/redsync/v4"
	"io"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"hedge/common/client"
	db2 "hedge/common/db"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	trgConfig "hedge/edge-ml-service/internal/config"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"hedge/edge-ml-service/pkg/helpers"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	redisMock "hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	trainingmocks "hedge/mocks/hedge/edge-ml-service/pkg/training-data"
)

var (
	modelDeployCommand          ml_model.ModelDeployCommand
	mlEventConfigs              []config.MLEventConfig
	dBMockClient, dBMockClient1 redisMock.MockMLDbInterface
	flds2                       fields1
	mockJob                     job.TrainingJobDetails
	mockAlgorithmConfig         config.MLAlgorithmDefinition
	connectionConfig            *config.MLMgmtConfig
	dataSourceProvider          *svcmocks.MockDataStoreProvider
	mockMlModels                []ml_model.MLModel
	mockModelDeploymentStatus   ml_model.ModelDeploymentStatus
	mockModelDeploymentStatus1  ml_model.ModelDeploymentStatus
	mockMqttSender              *svcmocks.MockMqttSender
	nodes                       []models.Node
	mockmlModelConfig           *config.MLModelConfig
	mockedTrainingDataService   trainingmocks.MockTrainingDataServiceInterface
)

type fields1 struct {
	appConfig                            *config.MLMgmtConfig
	service                              interfaces.ApplicationService
	trainingConfigDataService            *trgConfig.MLModelConfigService
	trainingDataService                  TrainingDataServiceInterface
	dbClient                             redis.MLDbInterface
	currentConfigurationsWithRunningJobs map[string]struct{}
	lastProcessedTime                    int64
	mqttSender                           MqttSender
	telemetry                            *helpers.Telemetry
	client                               client.HTTPClient
	connectionHandler                    interface{}
}

func Init() {
	u = utils.NewApplicationServiceMock(map[string]string{"DataStore_Provider": "ADE", "MetaDataServiceUrl": "http://localhost:59881", "RedisHost": "edgex-redis",
		"ModelDeployTopic": "ModelDeployTopic", "Training_Provider": "ADE", "EstimatedJobDuration": "100",
		"HedgeAdminURL": "http://localhost:48098"})
	connectionConfig = config.NewMLMgmtConfig()
	connectionConfig.LoadCoreAppConfigurations(u.AppService)
	dataSourceProvider = &svcmocks.MockDataStoreProvider{}
	appConfig = &config.MLMgmtConfig{}
	appConfig.LoadCoreAppConfigurations(u.AppService)

	mockMqttSender = &svcmocks.MockMqttSender{}
	mockHttpClient = svcmocks.MockHTTPClient{}
	mockedTrainingDataService = trainingmocks.MockTrainingDataServiceInterface{}

	flds2 = fields1{
		appConfig:                            appConfig,
		service:                              u.AppService,
		trainingConfigDataService:            trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient, dataSourceProvider, nil, nil),
		trainingDataService:                  &mockedTrainingDataService,
		dbClient:                             &dBMockClient,
		currentConfigurationsWithRunningJobs: make(map[string]struct{}),
		lastProcessedTime:                    0,
		mqttSender:                           mockMqttSender,
		telemetry:                            &helpers.Telemetry{},
		client:                               &mockHttpClient,
	}
	modelDeployCommand = ml_model.ModelDeployCommand{
		MLAlgorithm:          "RandomForest",
		MLModelConfigName:    "exampleConfig",
		ModelVersion:         1,
		ModelName:            "ExampleModel",
		TargetNodes:          []string{"Node1", "Node2"},
		CommandName:          "Deploy",
		ModelAvailableDate:   1638300000,
		MLEventConfigsToSync: []ml_model.SyncMLEventConfig{{SyncCommand: ml_model.UPDATE, MLEventConfig: config.MLEventConfig{}, OldMLEventConfig: config.MLEventConfig{}}},
	}
	conditions := []config.Condition{
		{
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:          "Prediction",
					Operator:       config.BETWEEN_OPERATOR,
					LowerThreshold: 2,
					UpperThreshold: 3,
				},
			},
			SeverityLevel: "MINOR",
		},
		{
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:          "Prediction",
					Operator:       config.BETWEEN_OPERATOR,
					LowerThreshold: 3,
					UpperThreshold: 4,
				},
			},
			SeverityLevel: "MAJOR",
		},
		{
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:          "Prediction",
					Operator:       config.GREATER_THAN_OPERATOR,
					ThresholdValue: 4,
				},
			},
			SeverityLevel: "CRITICAL",
		},
	}

	mlEventConfig := config.MLEventConfig{
		MLAlgorithm:       "HedgeAnomaly",
		MlModelConfigName: "WindTurbineAnomaly",
		EventName:         "WindTurbineAnomaly" + "Event",
		Description:       "Anomaly detected",
		Conditions:        conditions, StabilizationPeriodByCount: 3,
	}
	mlEventConfigs = append(mlEventConfigs, mlEventConfig)

	dBMockClient = redisMock.MockMLDbInterface{}
	dBMockClient1 = redisMock.MockMLDbInterface{}

	mockmlModelConfig = &config.MLModelConfig{
		Name:                 "TestTrainingConfig",
		Version:              "1.0",
		Description:          "Test description",
		MLAlgorithm:          "ADE",
		Enabled:              true,
		ModelConfigVersion:   1,
		LocalModelStorageDir: "local/models/TestAlgorithm/TestProfile",
		TrainedModelCount:    0,
	}
	mockJob = job.TrainingJobDetails{
		Name:              "AIF Job",
		MLAlgorithm:       "ADE",
		MLModelConfigName: "TestTrainingConfig",
		MLModelConfig:     mockmlModelConfig,
		Enabled:           false,
		StatusCode:        job.New,
		Status:            "",
		StartTime:         time.Now().Unix(),
		EndTime:           0,
		Msg:               "",
		ModelNameVersion:  "",
		BucketName:        "",
		JobDir:            "",
		JobRegion:         "",
		PackageUris:       nil,
		ModelName:         "AIF Job",
		TrainingFramework: "",
		RuntimeVersion:    "",
		PythonModule:      "",
		PythonVersion:     "",
		GCPProjectName:    "",
		ModelDir:          "hedge_export",
		ModelDownloadDir:  "/hedge_export",
	}
	mockAlgorithmConfig = config.MLAlgorithmDefinition{
		Name:        "AIF",
		Description: "Test description",
		Type:        "Anomaly",
		Enabled:     true,
		//Supervised:  true,
		//InputFeaturesPredefined:  true,
		OutputFeaturesPredefined: true,
		//InputFeatures:            []string{"InputFeature1", "InputFeature2"},
		//OutputFeatures:           []string{"OutputFeature1", "OutputFeature2"},
		//TrainingComputeProvider:      "TestComputeProvider",
		DefaultPredictionEndpointURL: "/api/v3/predict",
		DefaultPredictionPort:        48096,
	}
	mockMlModels = []ml_model.MLModel{
		{
			MLAlgorithm:          "RandomForest",
			MLModelConfigName:    "exampleConfig",
			ModelName:            "RandomForest_Config1_1",
			ModelVersion:         1,
			ModelCreatedTimeSecs: 1638220800,
			IsModelDeprecated:    false,
		},
		{
			MLAlgorithm:          "RandomForest",
			MLModelConfigName:    "exampleConfig",
			ModelName:            "DecisionTree_Config2_2",
			ModelVersion:         2,
			ModelCreatedTimeSecs: 1638320803,
			IsModelDeprecated:    true,
		},
	}
	mockModelDeploymentStatus = ml_model.ModelDeploymentStatus{
		MLAlgorithm:             "RandomForest",
		MLModelConfigName:       "exampleConfig",
		ModelName:               "RandomForest_Config1_1",
		NodeId:                  "Node-1",
		NodeDisplayName:         "DisplayGroup1",
		DeploymentStatusCode:    ml_model.ReadyToDeploy,
		DeploymentStatus:        "",
		ModelVersion:            1,
		ModelDeploymentTimeSecs: time.Now().Unix(),
		IsModelDeprecated:       false,
		PermittedOption:         "",
		Message:                 "",
	}
	mockModelDeploymentStatus1 = ml_model.ModelDeploymentStatus{
		MLAlgorithm:             "RandomForest",
		MLModelConfigName:       "exampleConfig",
		ModelName:               "DecisionTree_Config2_2",
		NodeId:                  "Node-2",
		NodeDisplayName:         "DisplayGroup1",
		DeploymentStatusCode:    ml_model.ReadyToDeploy,
		DeploymentStatus:        "",
		ModelVersion:            2,
		ModelDeploymentTimeSecs: 1638320803,
		IsModelDeprecated:       false,
		PermittedOption:         "",
		Message:                 "",
	}

	nodes = []models.Node{
		{
			NodeId:           "Node-1",
			HostName:         "host1",
			IsRemoteHost:     false,
			Name:             "Node1",
			RuleEndPoint:     "/hedge/api/v3/rules/host1",
			WorkFlowEndPoint: "/hedge/hedge-node-red/host1",
		},
		{
			NodeId:           "Node-2",
			HostName:         "host2",
			IsRemoteHost:     true,
			Name:             "Node2",
			RuleEndPoint:     "/hedge/api/v3/rules/host2",
			WorkFlowEndPoint: "/hedge/hedge-node-red/host2",
		},
	}

	mockBody, _ := json.Marshal(nodes)
	mockResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}

}

func TestBuildTopicNameForNode(t *testing.T) {
	Init()
	type args struct {
		topic        string
		ctx          interfaces.AppFunctionContext
		modelDeplCmd interface{}
	}
	args1 := args{
		topic:        "sampleTopic",
		ctx:          u.AppFunctionContext,
		modelDeplCmd: &modelDeployCommand,
	}
	args2 := args{
		topic:        "sampleTopic",
		ctx:          u.AppFunctionContext,
		modelDeplCmd: &ml_model.ModelDeployCommand{TargetNodes: []string{""}},
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"BuildTopicNameForNode - Passed", args1, "Node1/sampleTopic", false},
		{"BuildTopicNameForNode - Failed", args2, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildTopicNameForNode(tt.args.topic, tt.args.ctx, tt.args.modelDeplCmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildTopicNameForNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BuildTopicNameForNode() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTrainingJobService(t *testing.T) {
	Init()
	type args struct {
		service                   interfaces.ApplicationService
		appConfig                 *config.MLMgmtConfig
		trainingConfigDataService *trgConfig.MLModelConfigService
		dbClient                  redis.MLDbInterface
		trainingDataService       *TrainingDataService
		telemetry                 *helpers.Telemetry
		mqttSender                MqttSender
	}
	mqttSender := &transforms.MQTTSecretSender{}
	args1 := args{
		service:                   u.AppService,
		appConfig:                 appConfig,
		trainingConfigDataService: &trgConfig.MLModelConfigService{},
		dbClient:                  &dBMockClient,
		trainingDataService:       &TrainingDataService{},
		telemetry:                 &helpers.Telemetry{},
		mqttSender:                mqttSender,
	}
	args2 := args{
		service:                   u.AppService,
		appConfig:                 appConfig,
		trainingConfigDataService: &trgConfig.MLModelConfigService{},
		dbClient:                  &dBMockClient,
		trainingDataService:       &TrainingDataService{},
		telemetry:                 &helpers.Telemetry{},
	}
	want := &TrainingJobService{
		appConfig:                            appConfig,
		service:                              u.AppService,
		trainingConfigDataService:            &trgConfig.MLModelConfigService{},
		trainingDataService:                  &TrainingDataService{},
		dbClient:                             &dBMockClient,
		currentConfigurationsWithRunningJobs: make(map[string]struct{}),
		lastProcessedTime:                    0,
		mqttSender:                           mqttSender,
		telemetry:                            &helpers.Telemetry{},
		client:                               nil,
	}
	tests := []struct {
		name string
		args args
		want *TrainingJobService
	}{
		{"NewTrainingJobService - Passed", args1, want},
		{"NewTrainingJobService - Passed1", args2, want},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTrainingJobService(tt.args.service, tt.args.appConfig, tt.args.trainingConfigDataService, tt.args.dbClient, tt.args.trainingDataService, tt.args.telemetry, tt.args.mqttSender, nil)
			assert.NotNil(t, got.mqttSender, "mqttSender should not be nil")
			assert.IsType(t, &transforms.MQTTSecretSender{}, got.mqttSender, "mqttSender should be of type transforms.MQTTSecretSender")
			assert.Equal(t, tt.want.appConfig, got.appConfig, "Unexpected appConfig")

			if tt.name == "NewTrainingJobService - Passed" && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTrainingJobService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_AddDefaultMLEventConfig(t *testing.T) {
	Init()
	dBMockClient.On("GetAlgorithm", mock.Anything).Return(&mockAlgorithmConfig, nil)
	dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything, mock.Anything).Return([]config.MLEventConfig{}, nil)
	dBMockClient.On("AddMLEventConfig", mock.Anything).Return(mlEventConfigs[0], nil)

	dBMockClient1.On("GetAlgorithm", mock.Anything).Return(&config.MLAlgorithmDefinition{Type: "Classification"}, nil)
	dBMockClient1.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything, mock.Anything).Return(mlEventConfigs, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	dBMockClient2.On("GetAlgorithm", mock.Anything).Return(&mockAlgorithmConfig, nil)
	dBMockClient2.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything, mock.Anything).Return(mlEventConfigs, nil)
	dBMockClient2.On("UpdateMLEventConfig", mock.Anything, mock.Anything).Return(config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	type args struct {
		mlModelConfig *config.MLModelConfig
		modelVersion  int64
		modelName     string
	}
	args1 := args{
		mlModelConfig: mockmlModelConfig,
		modelVersion:  1,
		modelName:     "TestModel",
	}
	flds3 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		dbClient:  &dBMockClient1,
	}
	flds4 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		dbClient:  &dBMockClient2,
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		{"AddDefaultMLEventConfig - Passed", flds2, args1, false},
		{"AddDefaultMLEventConfig - Failed", flds3, args1, true},
		{"AddDefaultMLEventConfig - Failed1", flds4, args1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			if err := js.AddDefaultMLEventConfig(tt.args.mlModelConfig, tt.args.modelName); (err != nil) != tt.wantErr {
				t.Errorf("AddDefaultMLEventConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTrainingJobService_BuildJob(t *testing.T) {
	Init()
	dBMockClient.On("GetMlModelConfig", mock.Anything, mock.Anything).Return(*mockmlModelConfig, nil)
	dBMockClient.On("GetAlgorithm", mock.Anything).Return(&mockAlgorithmConfig, nil)
	appConfig1 := &config.MLMgmtConfig{}
	appConfig1.LoadCoreAppConfigurations(u.AppService)
	dBMockClient1.On("GetMlModelConfig", mock.Anything, mock.Anything).Return(config.MLModelConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	appConfig3 := &config.MLMgmtConfig{}
	appConfig3.LoadCoreAppConfigurations(u.AppService)
	appConfig3.TrainingProvider = "nonexistent"

	flds3 := fields1{
		appConfig:                 appConfig1,
		service:                   u.AppService,
		trainingConfigDataService: trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient, dataSourceProvider, nil, nil),
	}
	flds4 := fields1{
		appConfig:                 appConfig1,
		service:                   u.AppService,
		trainingConfigDataService: trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient1, dataSourceProvider, nil, nil),
	}

	flds6 := fields1{
		appConfig:                 appConfig3,
		service:                   u.AppService,
		trainingConfigDataService: trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient, dataSourceProvider, nil, nil),
	}
	type args struct {
		jobSubmissionDetails job.JobSubmissionDetails
	}

	args2 := args{
		jobSubmissionDetails: job.JobSubmissionDetails{
			MLAlgorithm:       "InvalidProvider",
			MLModelConfigName: "SampleConfig",
		},
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    *job.TrainingJobDetails
		wantErr bool
	}{
		{"BuildJobConfig - Failed", flds3, args2, nil, true},
		{"BuildJobConfig - Failed1", flds4, args2, nil, true},
		{"BuildJobConfig - Failed2", flds6, args2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			_, err := js.BuildJobConfig(tt.args.jobSubmissionDetails)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildJobConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			//if !reflect.DeepEqual(got, tt.want) {
			//	t.Errorf("BuildJobConfig() got = %v, want %v", got, tt.want)
			//}
		})
	}
}

func TestTrainingJobService_DeleteMLTrainingJob(t *testing.T) {
	Init()
	dBMockClient.On("GetMLTrainingJob", mockJob.Name).Return(mockJob, nil)
	dBMockClient.On("DeleteMLTrainingJobs", mockJob.Name).Return(nil)
	dBMockClient.On("GetMLTrainingJob", "Non-Existent Job").Return(job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "no such job"))

	flds3 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		dbClient:  &dBMockClient1,
	}
	mockJob.StatusCode = job.TrainingCompleted
	sampleJob := job.TrainingJobDetails{
		Name:       "SampleJob",
		StatusCode: 5,
	}

	dBMockClient1.On("GetMLTrainingJob", mockJob.Name).Return(mockJob, nil)
	dBMockClient1.On("GetMLTrainingJob", sampleJob.Name).Return(sampleJob, nil)
	dBMockClient1.On("DeleteMLTrainingJobs", mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "error deleting the job"))

	type args struct {
		jobName string
	}
	args1 := args{
		jobName: mockJob.Name,
	}
	args2 := args{
		jobName: "Non-Existent Job",
	}
	args3 := args{
		jobName: sampleJob.Name,
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		{"DeleteMLTrainingJob - Passed", flds2, args1, false},
		{"DeleteMLTrainingJob - Passed1", flds3, args3, false},
		{"DeleteMLTrainingJob - Failed1", flds2, args2, true},
		{"DeleteMLTrainingJob - Failed2", flds3, args1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			if err := js.DeleteMLTrainingJob(tt.args.jobName); (err != nil) != tt.wantErr {
				t.Errorf("DeleteMLTrainingJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTrainingJobService_DeployUndeployMLModel(t *testing.T) {
	Init()
	dBMockClient.On("GetLatestModelsByConfig", modelDeployCommand.MLAlgorithm, modelDeployCommand.MLModelConfigName).Return(mockMlModels, nil)
	dBMockClient.On("GetAllMLEventConfigsByConfig", modelDeployCommand.MLAlgorithm, modelDeployCommand.MLModelConfigName, mock.Anything).Return(mlEventConfigs, nil)
	dBMockClient.On("UpdateModelDeployment", mock.Anything).Return(nil)
	dBMockClient.On("GetDeploymentsByNode", mock.Anything).Return(mockModelDeploymentStatus)

	u.AppService.On("BuildContext", "gg", "ml").Return(u.AppFunctionContext)
	mockMqttSender.On("MQTTSend", u.AppFunctionContext, mock.Anything).Return(true, nil)
	mockMqttSender1 := &svcmocks.MockMqttSender{}
	mockMqttSender1.On("MQTTSend", u.AppFunctionContext, mock.Anything).Return(false, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	dBMockClient1.On("GetLatestModelsByConfig", modelDeployCommand.MLAlgorithm, modelDeployCommand.MLModelConfigName).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient1.On("GetAlgorithm", mock.Anything).Return(&config.MLAlgorithmDefinition{}, nil)

	dBMockClient.On("GetLatestModelsByConfig", "AnotherAlgo", "trainingDataConfig.json").Return(mockMlModels, nil)
	dBMockClient.On("GetAllMLEventConfigsByConfig", "AnotherAlgo", "trainingDataConfig.json", mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("GetAlgorithm", mock.Anything).Return(&config.MLAlgorithmDefinition{}, nil)

	dBMockClient2 = redisMock.MockMLDbInterface{}
	dBMockClient2.On("GetLatestModelsByConfig", modelDeployCommand.MLAlgorithm, modelDeployCommand.MLModelConfigName).Return(mockMlModels, nil)
	dBMockClient2.On("GetAllMLEventConfigsByConfig", modelDeployCommand.MLAlgorithm, modelDeployCommand.MLModelConfigName, mock.Anything).Return(mlEventConfigs, nil)
	dBMockClient2.On("UpdateModelDeployment", mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient2.On("GetDeploymentsByNode", mock.Anything).Return(mockModelDeploymentStatus)
	dBMockClient2.On("GetAlgorithm", mock.Anything).Return(&config.MLAlgorithmDefinition{}, nil)

	flds3 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		dbClient:  &dBMockClient1,
	}
	flds4 := fields1{
		appConfig:  appConfig,
		service:    u.AppService,
		dbClient:   &dBMockClient2,
		mqttSender: mockMqttSender1,
	}
	modelDeployCommand1 := ml_model.ModelDeployCommand{
		TargetNodes: nil,
	}
	modelDeployCommand2 := ml_model.ModelDeployCommand{
		MLAlgorithm:       "AnotherAlgo",
		MLModelConfigName: "trainingDataConfig.json",
		TargetNodes:       modelDeployCommand.TargetNodes,
	}
	modelDeployCommand3 := ml_model.ModelDeployCommand{
		MLAlgorithm:       modelDeployCommand.MLAlgorithm,
		MLModelConfigName: modelDeployCommand.MLModelConfigName,
		TargetNodes:       modelDeployCommand.TargetNodes,
		CommandName:       "UnDeploy",
	}
	type args struct {
		modelDeployCommand *ml_model.ModelDeployCommand
	}
	args1 := args{
		modelDeployCommand: &modelDeployCommand,
	}
	args2 := args{
		modelDeployCommand: &modelDeployCommand1,
	}
	args3 := args{
		modelDeployCommand: &modelDeployCommand2,
	}
	args4 := args{
		modelDeployCommand: &modelDeployCommand3,
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		{"DeployUndeployMLModel - Passed", flds2, args1, false},
		{"DeployUndeployMLModel - Passed1", flds2, args4, false},
		{"DeployUndeployMLModel - Failed", flds3, args1, true},
		{"DeployUndeployMLModel - Failed2", flds3, args2, true},
		{"DeployUndeployMLModel - Failed3", flds2, args3, true},
		{"DeployUndeployMLModel - Failed4", flds4, args4, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			if err := js.DeployUndeployMLModel(tt.args.modelDeployCommand, &config.MLAlgorithmDefinition{}); (err != nil) != tt.wantErr {
				t.Errorf("DeployUndeployMLModel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTrainingJobService_FilterJobs(t *testing.T) {
	Init()
	lastProcessedTime := int64(170151604)
	currentTime := int64(170151684)

	flds3 := fields1{
		appConfig:         appConfig,
		service:           u.AppService,
		dbClient:          &dBMockClient1,
		lastProcessedTime: lastProcessedTime,
	}
	type args struct {
		jobs        []job.TrainingJobDetails
		currentTime int64
	}
	args1 := args{
		jobs: []job.TrainingJobDetails{
			{Name: "job-1", StartTime: lastProcessedTime + 100}, // Adjusted concrete timestamp
			{Name: "job-2", StartTime: lastProcessedTime - 100}, // Should not be included
		},
		currentTime: currentTime,
	}
	args2 := args{
		jobs: []job.TrainingJobDetails{
			{Name: "job-1", StartTime: int64(lastProcessedTime) - 100}, // StartTime less than lastProcessedTime
			{Name: "job-2", StartTime: int64(lastProcessedTime) - 50},  // StartTime equal to currentTime
			{Name: "job-3", StartTime: int64(lastProcessedTime) + 100}, // StartTime greater than currentTime
		},
		currentTime: lastProcessedTime - 50,
	}
	want := []job.TrainingJobDetails{
		{
			Name:      "job-1",
			StartTime: lastProcessedTime + 100,
		},
	}
	tests := []struct {
		name   string
		fields fields1
		args   args
		want   []job.TrainingJobDetails
	}{
		{"FilterJobs - Passed", flds3, args1, want},
		{"FilterJobs - Failed", flds3, args2, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			if got := js.FilterJobs(tt.args.jobs, tt.args.currentTime); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterJobs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_GetEstimatedDuration(t *testing.T) {
	Init()
	jobs := []job.TrainingJobDetails{
		{Name: "job-1", StartTime: 170150604, EndTime: 170151654, StatusCode: job.TrainingCompleted},
		{Name: "job-2", StartTime: 170150605, EndTime: 170151655, StatusCode: job.TrainingCompleted},
	}
	jobs1 := []job.TrainingJobDetails{
		{Name: "job-1", StartTime: 170150604, EndTime: 170151654, StatusCode: job.New},
		{Name: "job-2", StartTime: 170150605, EndTime: 170151655, StatusCode: job.UploadingTrainingData},
	}
	dBMockClient.On("GetMLTrainingJobs", mock.Anything).Return(jobs, nil)
	dBMockClient1.On("GetMLTrainingJobs", mock.Anything).Return(jobs1, nil)

	flds3 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		dbClient:  &dBMockClient1,
	}
	type args struct {
		configJobsKey string
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    int64
		wantErr bool
	}{
		{"GetEstimatedDuration - Passed-1", flds2, args{configJobsKey: "someConfigKey"}, 1050, false},
		{"GetEstimatedDuration - Passed-2", flds3, args{configJobsKey: "someConfigKey"}, 100, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.GetEstimatedDuration(tt.args.configJobsKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEstimatedDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetEstimatedDuration() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_GetJob(t *testing.T) {
	Init()
	dBMockClient.On("GetMLTrainingJob", mockJob.Name).Return(mockJob, nil)
	dBMockClient.On("GetMLTrainingJob", "non-existent job name").Return(job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	type args struct {
		jobName string
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    job.TrainingJobDetails
		wantErr bool
	}{
		{"GetJob - Passed", flds2, args{jobName: mockJob.Name}, mockJob, false},
		{"GetJob - Failed", flds2, args{jobName: "non-existent job name"}, job.TrainingJobDetails{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.GetJob(tt.args.jobName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetJob() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_GetJobsInProgress(t *testing.T) {
	Init()
	algoName := "TestAlgorithm"
	modelConfigName := "TestModelConfig"
	errorName := "error"
	jobs := []job.TrainingJobDetails{
		{Name: "job", MLAlgorithm: algoName, MLModelConfigName: modelConfigName, StatusCode: job.TrainingInProgress},
	}

	dBMockClient.On("GetMLTrainingJobsByConfig", modelConfigName, mock.Anything).Return(jobs, nil)
	dBMockClient.On("GetMLTrainingJobsByConfig", errorName, mock.Anything).Return([]job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	flds2.dbClient = &dBMockClient
	type args struct {
		mlModelConfigName string
	}
	args1 := args{
		mlModelConfigName: modelConfigName,
	}
	args2 := args{
		mlModelConfigName: errorName,
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    []job.TrainingJobDetails
		wantErr bool
	}{
		{"GetJobsInProgress - Passed", flds2, args1, jobs, false},
		{"GetJobsInProgress - Failed", flds2, args2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.GetJobsInProgress(algoName, tt.args.mlModelConfigName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobsInProgress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetJobsInProgress() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_GetJobSummary(t *testing.T) {
	Init()
	jobs := []job.TrainingJobDetails{
		{Name: "job-1", MLAlgorithm: "algorithm-1", StartTime: 170516780, MLModelConfigName: mockmlModelConfig.Name, Status: "InProgress"},
		{Name: "job-2", MLAlgorithm: "algorithm-1", StartTime: 170150605, MLModelConfigName: mockmlModelConfig.Name, Status: "Created"},
	}

	dBMockClient.On("GetMLTrainingJobsByConfig", mockmlModelConfig.Name, mock.Anything).Return(jobs, nil)
	dBMockClient.On("GetMLTrainingJobsByConfig", "config-1", mock.Anything).Return([]job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	jobSummary := []job.TrainingJobSummary{
		{
			Name:              "job-1",
			MLAlgorithm:       "algorithm-1",
			MLModelConfigName: mockmlModelConfig.Name,
			Status:            "InProgress",
			StartTime:         170516780,
		},
	}
	type args struct {
		trainingConfigName string
		jobStatus          string
	}
	args1 := args{
		trainingConfigName: mockmlModelConfig.Name,
		jobStatus:          "InProgress",
	}
	args2 := args{
		trainingConfigName: "config-1",
		jobStatus:          "InProgress",
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    []job.TrainingJobSummary
		wantErr bool
	}{
		{"GetJobSummary - Passed", flds2, args1, jobSummary, false},
		{"GetJobSummary - Failed", flds2, args2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.GetJobSummary(tt.args.trainingConfigName, tt.args.jobStatus)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobSummary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetJobSummary() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_GetJobs(t *testing.T) {
	Init()
	jobs := []job.TrainingJobDetails{
		{Name: "job-1", MLAlgorithm: "algorithm-1", StartTime: 170516780, MLModelConfigName: mockmlModelConfig.Name, Status: "InProgress"},
		{Name: "job-2", MLAlgorithm: "algorithm-1", StartTime: 170150605, MLModelConfigName: mockmlModelConfig.Name, Status: "Created"},
	}

	dBMockClient.On("GetMLTrainingJobs", "job-key-1").Return(jobs, nil)
	dBMockClient.On("GetMLTrainingJobs", "job-key-2").Return([]job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	type args struct {
		jobKey string
	}
	args1 := args{
		jobKey: "job-key-1",
	}
	args2 := args{
		jobKey: "job-key-2",
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    []job.TrainingJobDetails
		wantErr bool
	}{
		{"GetJobs - Passed", flds2, args1, jobs, false},
		{"GetJobs - Failed", flds2, args2, []job.TrainingJobDetails{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.GetJobs(tt.args.jobKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetJobs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetJobs() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_GetMLEventConfig(t *testing.T) {
	Init()
	dBMockClient.On("GetMLEventConfigByName", "RandomForest", mock.Anything, mock.Anything, mock.Anything).Return(mlEventConfigs[0], nil)
	dBMockClient.On("GetAllMLEventConfigsByConfig", "RandomForest", mock.Anything, mock.Anything).Return(mlEventConfigs, nil)

	dBMockClient1.On("GetMLEventConfigByName", "RandomForest", mock.Anything, mock.Anything, mock.Anything).Return(config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient1.On("GetAllMLEventConfigsByConfig", "RandomForest", mock.Anything, mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	flds := fields1{
		service:  u.AppService,
		dbClient: &dBMockClient,
	}
	flds1 := fields1{
		service:  u.AppService,
		dbClient: &dBMockClient1,
	}

	type args struct {
		mlAlgorithm        string
		trainingConfigName string
		modelVersion       int64
		eventName          string
	}
	args1 := args{
		mlAlgorithm:        "RandomForest",
		trainingConfigName: "exampleConfig",
		modelVersion:       1,
		eventName:          "all",
	}
	args2 := args{
		mlAlgorithm:        "RandomForest",
		trainingConfigName: "exampleConfig",
		modelVersion:       1,
		eventName:          "TestEvent",
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    interface{}
		wantErr bool
	}{
		{"GetMLEventConfig - PassedAll", flds, args1, mlEventConfigs, false},
		{"GetMLEventConfig - PassedTestEvent", flds, args2, mlEventConfigs[0], false},
		{"GetMLEventConfig - FailedAll", flds1, args1, nil, true},
		{"GetMLEventConfig - FailedTestEvent", flds1, args2, config.MLEventConfig{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			got, err := js.GetMLEventConfig(tt.args.mlAlgorithm, tt.args.trainingConfigName, tt.args.eventName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMLEventConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if isNil(got) && isNil(tt.want) {
				return
			} else {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("GetMLEventConfig() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func isNil(i interface{}) bool {
	// Check if the value is nil or if the underlying value is a nil pointer
	if i == nil {
		return true
	}
	// Use reflection to check if i is a pointer or interface that is nil
	v := reflect.ValueOf(i)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

func TestTrainingJobService_GetModelDeploymentsByConfig(t *testing.T) {
	Init()
	dBMockClient.On("GetModels", "RandomForest", "exampleConfig").Return(mockMlModels, nil)
	dBMockClient.On("GetModels", "RandomForest", "anotherConfig").Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("GetDeploymentsByConfig", "RandomForest", "exampleConfig").Return([]ml_model.ModelDeploymentStatus{}, nil)
	mockModelDeploymentStatus.NodeId = ""

	mockDeplStatus := ml_model.ModelDeploymentStatus{
		MLAlgorithm:             "RandomForest",
		MLModelConfigName:       "exampleConfig",
		ModelName:               "DecisionTree_Config2_2",
		NodeId:                  "Node-1",
		NodeDisplayName:         "Node1",
		DeploymentStatusCode:    ml_model.ModelEndOfLife,
		DeploymentStatus:        "ModelEndOfLife",
		ModelVersion:            2,
		ModelDeploymentTimeSecs: 0,
		IsModelDeprecated:       true,
		PermittedOption:         "none",
		Message:                 "",
	}
	mockDeplStatus1 := ml_model.ModelDeploymentStatus{
		MLAlgorithm:          "RandomForest",
		MLModelConfigName:    "exampleConfig",
		NodeId:               "Node-2",
		NodeDisplayName:      "",
		DeploymentStatusCode: ml_model.ModelEndOfLife,
		ModelVersion:         1,
	}

	deployments := []ml_model.ModelDeploymentStatus{
		mockDeplStatus,
		mockDeplStatus,
		mockDeplStatus1,
		mockDeplStatus1,
	}

	dBMockClient1.On("GetModels", "RandomForest", "exampleConfig").Return(mockMlModels, nil)
	dBMockClient1.On("GetDeploymentsByConfig", "RandomForest", "exampleConfig").Return(deployments, nil)

	dBMockClient2 := redisMock.MockMLDbInterface{}
	dBMockClient2.On("GetModels", "RandomForest", "exampleConfig").Return(mockMlModels, nil)
	dBMockClient2.On("GetDeploymentsByConfig", "RandomForest", "exampleConfig").Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	nodes := []models.Node{
		{
			NodeId: "Node-1, Node-1",
		},
		{
			NodeId: "Node-2, Node-2",
			Name:   "Node2",
		},
	}

	mockBody, _ := json.Marshal(nodes)
	mockResponse := &http.Response{
		StatusCode: http.StatusOK,
		Body:       nil,
	}
	runFunc := func(args mock.Arguments) {
		mockResponse.Body = io.NopCloser(bytes.NewBuffer(mockBody))
	}
	mockHttpClient.On("Do", mock.Anything).Run(runFunc).Return(mockResponse, nil)

	type args struct {
		mlAlgorithm       string
		mlModelConfigName string
		groupByNodes      bool
	}
	args1 := args{
		mlAlgorithm:       "RandomForest",
		mlModelConfigName: "exampleConfig",
		groupByNodes:      true,
	}
	args2 := args{
		mlAlgorithm:       "RandomForest",
		mlModelConfigName: "anotherConfig",
		groupByNodes:      true,
	}
	args3 := args{
		mlAlgorithm:       "RandomForest",
		mlModelConfigName: "exampleConfig",
		groupByNodes:      false,
	}
	flds3 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		dbClient:  &dBMockClient1,
		client:    &mockHttpClient,
	}
	flds4 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		dbClient:  &dBMockClient2,
		client:    &mockHttpClient,
	}
	want := []ml_model.ModelDeploymentStatus{
		{
			MLAlgorithm:             "RandomForest",
			MLModelConfigName:       "exampleConfig",
			ModelName:               "DecisionTree_Config2_2",
			NodeId:                  "Node-1, Node-2",
			NodeDisplayName:         "Node-1, host2",
			DeploymentStatusCode:    ml_model.ModelEndOfLife,
			DeploymentStatus:        "ModelEndOfLife",
			ModelVersion:            2,
			ModelDeploymentTimeSecs: time.Now().Unix(),
			IsModelDeprecated:       true,
			PermittedOption:         "none",
			Message:                 "",
		},
	}

	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    []ml_model.ModelDeploymentStatus
		wantErr bool
	}{
		{"GetModelDeploymentsByConfig - Passed", flds2, args1, want, false},
		{"GetModelDeploymentsByConfig - Passed1", flds3, args1, want, false},
		{"GetModelDeploymentsByConfig - Passed2", flds3, args3, want, false},
		{"GetModelDeploymentsByConfig - Failed", flds2, args2, []ml_model.ModelDeploymentStatus{}, true},
		{"GetModelDeploymentsByConfig - Failed1", flds4, args1, []ml_model.ModelDeploymentStatus{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.GetModelDeploymentsByConfig(tt.args.mlAlgorithm, tt.args.mlModelConfigName, tt.args.groupByNodes)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetModelDeploymentsByConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !(got != nil && want != nil) {
				t.Errorf("GetModelDeploymentsByConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_GetModels(t *testing.T) {
	Init()
	dBMockClient.On("GetModels", "RandomForest", "exampleConfig").Return(mockMlModels, nil)
	dBMockClient.On("GetModels", "DummyAlgorithm", "exampleConfig").Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("GetLatestModelsByConfig", modelDeployCommand.MLAlgorithm, modelDeployCommand.MLModelConfigName).Return(mockMlModels, nil)

	type args struct {
		mlAlgorithm        string
		mlModelConfigName  string
		latestVersionsOnly bool
	}
	args1 := args{
		mlAlgorithm:        "RandomForest",
		mlModelConfigName:  "exampleConfig",
		latestVersionsOnly: false,
	}
	args2 := args{
		mlAlgorithm:        "RandomForest",
		mlModelConfigName:  "exampleConfig",
		latestVersionsOnly: true,
	}
	args3 := args{
		mlAlgorithm:        "DummyAlgorithm",
		mlModelConfigName:  "exampleConfig",
		latestVersionsOnly: false,
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    []ml_model.MLModel
		wantErr bool
	}{
		{"GetModels - Passed-1", flds2, args1, mockMlModels, false},
		{"GetModels - Passed-2", flds2, args2, mockMlModels, false},
		{"GetModels - Failed", flds2, args3, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.GetModels(tt.args.mlAlgorithm, tt.args.mlModelConfigName, tt.args.latestVersionsOnly)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetModels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetModels() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_PublishModelDeploymentCommand(t *testing.T) {
	Init()
	mockMqttSender.On("MQTTSend", u.AppFunctionContext, mock.Anything).Return(true, nil)
	mockMqttSender1 := &svcmocks.MockMqttSender{}
	mockMqttSender1.On("MQTTSend", u.AppFunctionContext, mock.Anything).Return(false, nil)
	u.AppService.On("BuildContext", "gg", "ml").Return(u.AppFunctionContext)

	type args struct {
		modelDeployCommand *ml_model.ModelDeployCommand
	}
	flds3 := fields1{
		service:    u.AppService,
		mqttSender: mockMqttSender1,
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		{"PublishModelDeploymentCommand - Passed", flds2, args{&modelDeployCommand}, false},
		{"PublishModelDeploymentCommand - Failed", flds3, args{&modelDeployCommand}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			if err := js.PublishModelDeploymentCommand(tt.args.modelDeployCommand); (err != nil) != tt.wantErr {
				t.Errorf("PublishModelDeploymentCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func readHedgeAlgoTemplateFile(string) ([]byte, error) {
	return []byte(`{"name": "Template1", "otherKey": "otherValue"}`), nil
}

func TestTrainingJobService_SubmitJob(t *testing.T) {
	Init()

	//mockResponseBody, _ := json.Marshal(algoInstance)

	mockTrainingConfig1 := &config.MLModelConfig{
		Enabled: false,
	}
	job1 := job.TrainingJobDetails{Name: "job-1", StatusCode: job.New, MLModelConfigName: "exampleConf", MLModelConfig: mockTrainingConfig1}
	dBMockClient.On("GetMLTrainingJobs", db2.MLTrainingJob+":config:"+mockJob.MLModelConfigName).Return([]job.TrainingJobDetails{mockJob}, nil)
	dBMockClient.On("GetMLTrainingJobs", db2.MLTrainingJob+":config:"+job1.MLModelConfigName).Return([]job.TrainingJobDetails{job1}, nil)
	dBMockClient.On("GetMLTrainingJob", mockJob.Name).Return(mockJob, nil)
	dBMockClient.On("GetMLTrainingJob", job1.Name).Return(job1, nil)
	dBMockClient.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)

	mockedTrainingDataService.On("CreateTrainingData", mock.Anything, mock.Anything).Return("", nil)
	mockedMlStorage.On("FileExists", mock.Anything).Return(true)
	mockedMlStorage.On("GetMLModelConfigFileName", true).Return("TestTrainingConfig")

	type args struct {
		inputJob *job.TrainingJobDetails
	}
	flds3 := fields1{
		currentConfigurationsWithRunningJobs: map[string]struct{}{mockJob.MLAlgorithm + "_" + mockJob.MLModelConfigName: {}},
	}
	appConfig1 := &config.MLMgmtConfig{}
	appConfig1.LoadCoreAppConfigurations(u.AppService)

	flds4 := fields1{
		appConfig:                            appConfig1,
		service:                              u.AppService,
		trainingConfigDataService:            trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient, dataSourceProvider, nil, nil),
		trainingDataService:                  &mockedTrainingDataService,
		dbClient:                             &dBMockClient,
		currentConfigurationsWithRunningJobs: make(map[string]struct{}),
		lastProcessedTime:                    0,
		mqttSender:                           mockMqttSender,
		telemetry:                            &helpers.Telemetry{},
		client:                               &mockHttpClient,
	}

	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		//{"SubmitJob - Passed", flds2, args{&mockJob}, false},
		{"SubmitJob - Failed", flds4, args{&mockJob}, true},
		{"SubmitJob - Failed-1", flds3, args{&mockJob}, true},
		{"SubmitJob - Failed-2", flds2, args{&job1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			if err := js.SubmitJob(tt.args.inputJob); (err != nil) != tt.wantErr {
				t.Errorf("SubmitJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// fileType:
// 0 - valid file
// 1 - without config.json
// 2 - without model
// 3 - invalid zip file
// 4 - invalid file descriptor
func createRegisterUploadedModelZipFile(path string, fileType int8) {
	if _, err := os.Stat(path); err == nil {
		// File exists, so remove it
		os.Remove(path)
	}

	if fileType == 4 {
		return
	}

	zipFile, _ := os.Create(path)
	defer zipFile.Close()

	if fileType == 3 {
		return
	}

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	if fileType != 1 {
		zipWriter.Create("config.json")
	}
	if fileType != 2 {
		zipWriter.Create("something.pickle")
	}
}

func TestTrainingJobService_RegisterUploadedModel(t *testing.T) {
	Init()
	mockTrainingConfig1 := &config.MLModelConfig{
		Name: "name",
	}
	job1 := job.TrainingJobDetails{Name: "job1", MLAlgorithm: "MockMLAlgorithm", MLModelConfigName: "MockMLModelConfigName", MLModelConfig: mockTrainingConfig1}
	dBMockClient.On("GetMLTrainingJobs", db2.MLTrainingJob+":config:"+job1.MLModelConfigName).Return([]job.TrainingJobDetails{job1}, nil)
	dBMockClient.On("GetMLTrainingJob", job1.Name).Return(job1, nil)
	dBMockClient.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)

	dBMockClient.On("GetLatestModelVersion", mock.Anything, mock.Anything).Return(int64(1), nil)
	dBMockClient.On("SaveMLModel", mock.Anything).Return(nil)
	dBMockClient.On("GetMLEventConfig", mock.Anything, mock.Anything, mock.Anything).Return(&config.MLEventConfig{}, nil)
	dBMockClient.On("UpdateMLEventConfig", mock.Anything, mock.Anything).Return(config.MLEventConfig{}, nil)

	dBMockClient.On("GetMetricCounter", mock.Anything).Return(int64(1), nil)
	dBMockClient.On("IncrMetricCounterBy", mock.Anything, mock.Anything).Return(int64(1), nil)

	dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything, mock.Anything).Return([]config.MLEventConfig{config.MLEventConfig{}}, nil)
	dBMockClient.On("GetAlgorithm", mock.Anything).Return(&config.MLAlgorithmDefinition{}, nil)
	dBMockClient.On("AcquireRedisLock", mock.Anything).Return(&redsync.Mutex{}, nil)

	dBMockClient1.On("GetMLTrainingJobs", db2.MLTrainingJob+":config:"+job1.MLModelConfigName).Return([]job.TrainingJobDetails{job1}, nil)
	dBMockClient1.On("GetMLTrainingJob", mock.Anything).Return(job1, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient1.On("AddMLTrainingJob", mock.Anything).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient1.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)
	dBMockClient1.On("GetLatestModelVersion", mock.Anything, mock.Anything).Return(int64(1), nil)
	dBMockClient1.On("SaveMLModel", mock.Anything).Return(nil)
	dBMockClient1.On("GetMLEventConfig", mock.Anything, mock.Anything, mock.Anything).Return(&config.MLEventConfig{}, nil)
	dBMockClient1.On("GetMetricCounter", mock.Anything).Return(int64(1), nil)
	dBMockClient1.On("IncrMetricCounterBy", mock.Anything, mock.Anything).Return(int64(1), nil)

	dBMockClient2.On("GetMLTrainingJobs", db2.MLTrainingJob+":config:"+job1.MLModelConfigName).Return([]job.TrainingJobDetails{job1}, nil)
	dBMockClient2.On("GetMLTrainingJob", mock.Anything).Return(job1, nil)
	dBMockClient2.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)
	dBMockClient2.On("GetLatestModelVersion", mock.Anything, mock.Anything).Return(int64(1), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient2.On("SaveMLModel", mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient2.On("GetMLEventConfig", mock.Anything, mock.Anything, mock.Anything).Return(&config.MLEventConfig{}, nil)
	dBMockClient2.On("GetMetricCounter", mock.Anything).Return(int64(1), nil)
	dBMockClient2.On("IncrMetricCounterBy", mock.Anything, mock.Anything).Return(int64(1), nil)

	dBMockClient3 := redisMock.MockMLDbInterface{}
	dBMockClient3.On("GetMetricCounter", mock.Anything).Return(int64(1), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""))
	dBMockClient3.On("AcquireRedisLock", mock.Anything).Return(&redsync.Mutex{}, nil)

	mockedTrainingDataService.On("CreateTrainingData", mock.Anything, mock.Anything).Return("", nil)
	mockedMlStorage.On("FileExists", mock.Anything).Return(true)
	mockedMlStorage.On("GetMLModelConfigFileName", true).Return("TestTrainingConfig")

	appConfig1 := &config.MLMgmtConfig{}
	appConfig1.LoadCoreAppConfigurations(u.AppService)
	appConfig1.BaseTrainingDataLocalDir = "/tmp"

	mockedMetricMngr := &mocks.MetricsManager{}
	mockedMetricMngr.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
	client.Client = &mockHttpClient

	telemetry := helpers.NewTelemetry(u.AppService, "serviceName", mockedMetricMngr, &dBMockClient)
	telemetry1 := helpers.NewTelemetry(u.AppService, "serviceName", mockedMetricMngr, &dBMockClient3)

	flds := fields1{
		appConfig:                            appConfig1,
		service:                              u.AppService,
		trainingConfigDataService:            trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient, dataSourceProvider, nil, nil),
		trainingDataService:                  &mockedTrainingDataService,
		dbClient:                             &dBMockClient,
		currentConfigurationsWithRunningJobs: make(map[string]struct{}),
		lastProcessedTime:                    0,
		mqttSender:                           mockMqttSender,
		telemetry:                            telemetry,
		client:                               &mockHttpClient,
	}
	flds1 := fields1{
		currentConfigurationsWithRunningJobs: map[string]struct{}{job1.MLAlgorithm + "_" + job1.MLModelConfigName: {}},
	}
	flds2 := fields1{
		appConfig:                            appConfig1,
		service:                              u.AppService,
		trainingConfigDataService:            trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient1, dataSourceProvider, nil, nil),
		trainingDataService:                  &mockedTrainingDataService,
		dbClient:                             &dBMockClient1,
		currentConfigurationsWithRunningJobs: make(map[string]struct{}),
		lastProcessedTime:                    0,
		mqttSender:                           mockMqttSender,
		telemetry:                            telemetry,
		client:                               &mockHttpClient,
	}
	flds3 := fields1{
		appConfig:                            appConfig1,
		service:                              u.AppService,
		trainingConfigDataService:            trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient2, dataSourceProvider, nil, nil),
		trainingDataService:                  &mockedTrainingDataService,
		dbClient:                             &dBMockClient2,
		currentConfigurationsWithRunningJobs: make(map[string]struct{}),
		lastProcessedTime:                    0,
		mqttSender:                           mockMqttSender,
		telemetry:                            telemetry1,
		client:                               &mockHttpClient,
	}

	finalDirPath := "/tmp/MockMLAlgorithm/MockMLModelConfigName"
	os.MkdirAll(finalDirPath, os.ModePerm)

	srcZipFilePath := "/tmp/TestTrainingJobService_RegisterUploadedModelSrc.zip"
	dstZipFilePath := "/tmp/TestTrainingJobService_RegisterUploadedModelDst.zip"

	type args struct {
		inputJob *job.TrainingJobDetails
		//srcZipFilePath string
		dstZipFilePath string
		zipFileType    int8
	}
	arg := args{
		inputJob:       &job1,
		dstZipFilePath: dstZipFilePath,
		zipFileType:    0,
	}
	arg2 := args{
		inputJob:       &job1,
		dstZipFilePath: "/nonexistent_folder/nonexistent_file",
		zipFileType:    0,
	}
	arg3 := args{
		inputJob:       &job1,
		dstZipFilePath: dstZipFilePath,
		zipFileType:    4,
	}
	arg4 := args{
		inputJob:       &job1,
		dstZipFilePath: dstZipFilePath,
		zipFileType:    3,
	}
	arg5 := args{
		inputJob:       &job1,
		dstZipFilePath: dstZipFilePath,
		zipFileType:    1,
	}
	arg6 := args{
		inputJob:       &job1,
		dstZipFilePath: dstZipFilePath,
		zipFileType:    2,
	}

	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{

		{"RegisterUploadedModel - Passed-1", flds, arg, false},
		{"RegisterUploadedModel - Passed-2", flds3, arg, false},
		{"RegisterUploadedModel - Failed-1", flds1, arg, true},
		{"RegisterUploadedModel - Failed-2", flds, arg2, true},
		{"RegisterUploadedModel - Failed-3", flds, arg3, true},
		{"RegisterUploadedModel - Failed-4", flds, arg4, true},
		{"RegisterUploadedModel - Failed-5", flds, arg5, true},
		{"RegisterUploadedModel - Failed-6", flds, arg6, true},
		{"RegisterUploadedModel - Failed-7", flds2, arg, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}

			createRegisterUploadedModelZipFile(srcZipFilePath, tt.args.zipFileType)
			tmpZipFile, _ := os.Open(srcZipFilePath)
			if err := js.RegisterUploadedModel(tt.args.dstZipFilePath, tmpZipFile, tt.args.inputJob); (err != nil) != tt.wantErr {
				t.Errorf("SubmitJob() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tmpZipFile != nil {
				tmpZipFile.Close()
			}
			os.Remove(tt.args.dstZipFilePath)
			os.Remove(srcZipFilePath)
		})
	}
	os.RemoveAll(finalDirPath)
	os.Remove("/tmp/MockMLAlgorithm")
}

func TestTrainingJobService_TerminateJobs(t *testing.T) {
	Init()
	dBMockClient.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)
	js := &TrainingJobService{
		dbClient: &dBMockClient,
		service:  u.AppService,
	}
	currentTime := time.Now().Unix()
	sampleJob := job.TrainingJobDetails{
		StartTime: currentTime - 21600,
	}

	updatedJobs := js.TerminateJobs([]job.TrainingJobDetails{sampleJob}, currentTime)

	if updatedJobs[0].StatusCode != 7 {
		t.Errorf("Expected StatusCode to be 7, got %d", updatedJobs[0].StatusCode)
	}
	if updatedJobs[0].EndTime != currentTime {
		t.Errorf("Expected EndTime to be %d, got %d", currentTime, updatedJobs[0].EndTime)
	}
	if updatedJobs[0].Msg != "Job Terminated Abnormally" {
		t.Errorf("Expected Msg to be 'Job Terminated Abnormally', got %s", updatedJobs[0].Msg)
	}
	if !dBMockClient.AssertCalled(t, "UpdateMLTrainingJob", mock.Anything) {
		t.Error("Expected UpdateMLTrainingJob to be called, but it wasn't")
	}
}

func TestTrainingJobService_TerminateLongRunningJobs(t *testing.T) {
	Init()
	terminationJobInterval := 2 * time.Second
	dBMockClient.On("GetMLTrainingJobs", mock.Anything).Return([]job.TrainingJobDetails{}, nil)
	tests := []struct {
		name   string
		fields fields1
	}{
		{"TerminateLongRunningJobs - Passed", flds2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}

			// Use a channel to signal termination
			stopCh := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				defer wg.Done()
				js.TerminateLongRunningJobs(stopCh, terminationJobInterval)
			}()

			time.Sleep(5 * time.Second)
			close(stopCh)
		})
	}
}

func TestTrainingJobService_UpdateMLEventConfig(t *testing.T) {
	Init()

	conditions := []config.Condition{
		{
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:          "Prediction",
					Operator:       config.BETWEEN_OPERATOR,
					LowerThreshold: 2,
					UpperThreshold: 3,
				},
			},
			SeverityLevel: "MINOR",
		},
	}

	mlEventConfig := config.MLEventConfig{
		MLAlgorithm:                "HedgeAnomaly",
		MlModelConfigName:          "WindTurbineAnomaly",
		EventName:                  "WindTurbineAnomaly" + "Event",
		Description:                "Anomaly detected",
		Conditions:                 conditions,
		StabilizationPeriodByCount: 3,
		MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
	}
	mlEventConfig1 := config.MLEventConfig{
		MLAlgorithm:                "HedgeAnomaly",
		MlModelConfigName:          "WindTurbineAnomaly",
		EventName:                  "WindTurbineAnomaly" + "Event",
		Description:                "Anomaly detected",
		Conditions:                 conditions,
		StabilizationPeriodByCount: 3,
		MLAlgorithmType:            "InvalidAlgoType",
	}
	mockBody, _ := json.Marshal([]models.Node{{NodeId: "nodeId-1"}, {NodeId: "nodeId-2"}})
	mockResponse1 := &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}
	mockResponse2 := &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}
	mockResponse3 := &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}

	dBMockClient.On("UpdateMLEventConfig", mock.Anything, mock.Anything).Return(mlEventConfig, nil)
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse1, nil)

	dBMockClient1.On("UpdateMLEventConfig", mock.Anything, mock.Anything).Return(mlEventConfig, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	mockHttpClient1.On("Do", mock.Anything).Return(mockResponse2, nil)

	dBMockClient2.On("UpdateMLEventConfig", mock.Anything, mock.Anything).Return(mlEventConfig, nil)
	mockHttpClient2 := svcmocks.MockHTTPClient{}
	mockHttpClient2.On("Do", mock.Anything).Return(mockResponse3, nil)

	u.AppService.On("BuildContext", "gg", "ml").Return(u.AppFunctionContext)
	mockMqttSender.On("MQTTSend", u.AppFunctionContext, mock.Anything).Return(true, nil)
	mockMqttSender1 := &svcmocks.MockMqttSender{}
	mockMqttSender1.On("MQTTSend", u.AppFunctionContext, mock.Anything).Return(false, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	type args struct {
		config *config.MLEventConfig
	}
	args1 := args{
		config: &config.MLEventConfig{
			MLAlgorithm:       "InvalidAlgorithm",
			MlModelConfigName: "yourTrainingConfig",
		},
	}
	flds3 := fields1{
		appConfig:  appConfig,
		service:    u.AppService,
		dbClient:   &dBMockClient1,
		client:     &mockHttpClient1,
		mqttSender: mockMqttSender,
	}
	flds4 := fields1{
		appConfig:  appConfig,
		service:    u.AppService,
		dbClient:   &dBMockClient2,
		client:     &mockHttpClient2,
		mqttSender: mockMqttSender1,
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		{"UpdateMLEventConfig - Passed", flds2, args{config: &mlEventConfig}, false},
		{"UpdateMLEventConfig - Failed", flds2, args1, true},
		{"UpdateMLEventConfig - Failed1", flds3, args{config: &mlEventConfig}, true},
		{"UpdateMLEventConfig - Failed2 (MQTTSend failed, err skipped)", flds4, args{config: &mlEventConfig}, false},
		{"UpdateMLEventConfig - Failed3", flds2, args{config: &mlEventConfig1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			if err := js.UpdateMLEventConfig(*tt.args.config, *tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("UpdateMLEventConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTrainingJobService_ValidateModelVersionExist_Pass(t *testing.T) {
	Init()
	modelVersion := int64(1)
	mlModels := []ml_model.MLModel{
		{
			ModelVersion: modelVersion,
		},
	}

	dBMockClient.On("GetMlModelConfig", mock.Anything, mock.Anything).Return(config.MLModelConfig{}, nil)
	dBMockClient.On("GetModels", mock.Anything, mock.Anything).Return(mlModels, nil)

	js := &TrainingJobService{
		service:  u.AppService,
		dbClient: &dBMockClient,
	}

	err := js.ValidateModelVersionExist("", "", modelVersion)
	if err != nil {
		t.Errorf("ValidateModelVersionExist returned error %v, but expected success", err)
	}
}

func TestTrainingJobService_ValidateModelVersionExist_VersionMismatch_Pass(t *testing.T) {
	Init()
	mlModels := []ml_model.MLModel{
		{
			ModelVersion: 1,
		},
	}

	dBMockClient.On("GetMlModelConfig", mock.Anything, mock.Anything).Return(config.MLModelConfig{}, nil)
	dBMockClient.On("GetModels", mock.Anything, mock.Anything).Return(mlModels, nil)

	js := &TrainingJobService{
		service:  u.AppService,
		dbClient: &dBMockClient,
	}

	err := js.ValidateModelVersionExist("", "", 2)
	if err != nil {
		t.Errorf("ValidateModelVersionExist returned error %v, but expected success", err)
	}
}

func TestTrainingJobService_ValidateModelVersionExist_GetModelConfigFailed(t *testing.T) {
	Init()
	modelVersion := int64(1)
	mlModels := []ml_model.MLModel{
		{
			ModelVersion: modelVersion,
		},
	}
	errMsg := "mock error"
	dBMockClient.On("GetMlModelConfig", mock.Anything, mock.Anything).Return(config.MLModelConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, errMsg))
	dBMockClient.On("GetModels", mock.Anything, mock.Anything).Return(mlModels, nil)

	js := &TrainingJobService{
		service:  u.AppService,
		dbClient: &dBMockClient,
	}

	err := js.ValidateModelVersionExist("", "", modelVersion)
	if err == nil || err.Message() != errMsg {
		t.Errorf("ValidateModelVersionExist() Expected error message: %s, received error: %v", errMsg, err)
	}
}

func TestTrainingJobService_ValidateModelVersionExist_ModelsLenZeroFailure(t *testing.T) {
	Init()
	modelVersion := int64(1)

	dBMockClient.On("GetMlModelConfig", mock.Anything, mock.Anything).Return(config.MLModelConfig{}, nil)
	dBMockClient.On("GetModels", mock.Anything, mock.Anything).Return([]ml_model.MLModel{}, nil)

	js := &TrainingJobService{
		service:  u.AppService,
		dbClient: &dBMockClient,
	}

	err := js.ValidateModelVersionExist("", "", modelVersion)
	if err == nil || err.ErrorType() != hedgeErrors.ErrorTypeNotFound {
		t.Errorf("ValidateModelVersionExist() Expected error type: %s, received error: %v", hedgeErrors.ErrorTypeNotFound, err)
	}
}

func TestTrainingJobService_addJob(t *testing.T) {
	Init()
	mockedJob := job.TrainingJobDetails{Name: "job-1", StartTime: 170150604, EndTime: 170151654, StatusCode: job.TrainingInProgress, EstimatedDuration: 100}
	jobs := []job.TrainingJobDetails{mockedJob}
	dBMockClient.On("GetMLTrainingJobs", mock.Anything).Return(jobs, nil)
	dBMockClient.On("GetMLTrainingJob", mockedJob.Name).Return(mockedJob, nil)
	mockJob.EstimatedDuration = 100
	dBMockClient.On("GetMLTrainingJob", mockJob.Name).Return(job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("AddMLTrainingJob", mockedJob).Return("", nil)
	dBMockClient.On("AddMLTrainingJob", mockJob).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)

	dBMockClient1.On("GetMLTrainingJobs", mock.Anything).Return([]job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	flds3 := fields1{
		appConfig:                            appConfig,
		service:                              u.AppService,
		trainingConfigDataService:            trgConfig.NewMlModelConfigService(u.AppService, connectionConfig, &dBMockClient1, dataSourceProvider, nil, nil),
		trainingDataService:                  &mockedTrainingDataService,
		dbClient:                             &dBMockClient1,
		currentConfigurationsWithRunningJobs: make(map[string]struct{}),
		lastProcessedTime:                    0,
		mqttSender:                           mockMqttSender,
		telemetry:                            &helpers.Telemetry{},
		client:                               &mockHttpClient,
	}
	type args struct {
		job *job.TrainingJobDetails
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    *job.TrainingJobDetails
		wantErr bool
	}{
		{"AddJob - Passed", flds2, args{job: &mockedJob}, &mockedJob, false},
		{"AddJob - Failed", flds2, args{job: &mockJob}, nil, true},
		{"AddJob - Failed1", flds3, args{job: &mockJob}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.addJob(tt.args.job)
			if (err != nil) != tt.wantErr {
				t.Errorf("addJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addJob() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_getNodes(t *testing.T) {
	Init()
	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
	mockHttpClient1 = svcmocks.MockHTTPClient{}
	mockHttpClient1.On("Do", mock.Anything).Return(nil, errors.New("mocked error"))
	flds3 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		client:    &mockHttpClient1,
	}

	tests := []struct {
		name    string
		fields  fields1
		want    []models.Node
		wantErr bool
	}{
		{"GetNodes - Passed", flds2, nodes, false},
		{"GetNodes - Failed", flds3, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.getNodes()
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNodes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_getNodesGroups(t *testing.T) {
	Init()
	mockedNodeGroups := []models.NodeGroup{
		{
			Name:        "Group1",
			DisplayName: "Group 1",
			Node: &models.Node{
				NodeId:           "node1",
				HostName:         "host1",
				IsRemoteHost:     false,
				Name:             "Node 1",
				RuleEndPoint:     "/hedge/api/v3/rules/host1",
				WorkFlowEndPoint: "/hedge/hedge-node-red/host1",
			},
			ChildNodeGroups: nil,
		},
	}
	mockBody, _ := json.Marshal(mockedNodeGroups)
	mockResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}

	mockHttpClient.On("Do", mock.Anything).Return(mockResponse, nil)
	mockHttpClient1 = svcmocks.MockHTTPClient{}
	mockHttpClient1.On("Do", mock.Anything).Return(nil, errors.New("mocked error"))
	flds3 := fields1{
		appConfig: appConfig,
		service:   u.AppService,
		client:    &mockHttpClient1,
	}

	tests := []struct {
		name    string
		fields  fields1
		want    []models.NodeGroup
		wantErr bool
	}{
		{"GetNodesGroups - Passed", flds2, mockedNodeGroups, false},
		{"GetNodesGroups - Failed", flds3, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
				client:                               tt.fields.client,
			}
			got, err := js.getNodesGroups()
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodesGroups() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNodesGroups() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrainingJobService_monitorDeploymentStatus(t *testing.T) {
	Init()
	monitorDeploymentInterval := 2 * time.Second
	dBMockClient.On("GetDeploymentsByNode", mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient1.On("GetDeploymentsByNode", mock.Anything).Return([]ml_model.ModelDeploymentStatus{mockModelDeploymentStatus}, nil)
	dBMockClient1.On("UpdateModelDeployment", mock.Anything).Return(nil)

	flds3 := fields1{
		service:  u.AppService,
		dbClient: &dBMockClient1,
	}

	type args struct {
		deploymentStatus ml_model.ModelDeploymentStatus
	}
	tests := []struct {
		name   string
		fields fields1
		args   args
	}{
		{"MonitorDeploymentStatus - Passed", flds2, args{mockModelDeploymentStatus}},
		{"MonitorDeploymentStatus - Failed", flds3, args{mockModelDeploymentStatus}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js := &TrainingJobService{
				appConfig:                            tt.fields.appConfig,
				service:                              tt.fields.service,
				trainingConfigDataService:            tt.fields.trainingConfigDataService,
				trainingDataService:                  tt.fields.trainingDataService,
				dbClient:                             tt.fields.dbClient,
				currentConfigurationsWithRunningJobs: tt.fields.currentConfigurationsWithRunningJobs,
				lastProcessedTime:                    tt.fields.lastProcessedTime,
				mqttSender:                           tt.fields.mqttSender,
				telemetry:                            tt.fields.telemetry,
			}
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				js.monitorDeploymentStatus(tt.args.deploymentStatus, monitorDeploymentInterval)
			}()

			// Wait for the goroutine to finish or timeout
			if waitTimeout(&wg, 20*time.Second) {
				t.Error("Test timed out")
			}
		})
	}
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // Completed successfully
	case <-time.After(timeout):
		return true // Timeout occurred
	}
}

func Test_buildLatestModelVersionKeys(t *testing.T) {
	deployments := []ml_model.MLModel{
		{MLModelConfigName: "algo1", ModelVersion: 1},
		{MLModelConfigName: "algo2", ModelVersion: 2},
		{MLModelConfigName: "algo1", ModelVersion: 3},
	}

	result := buildLatestModelVersionKeys(deployments)
	expected := []string{"algo1:v1", "algo2:v2", "algo1:v3"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("BuildLatestModelVersionKeys() returned %v, expected %v", result, expected)
	}
}

func Test_buildMatchingDeployment(t *testing.T) {
	deployments := []ml_model.ModelDeploymentStatus{
		{
			MLAlgorithm:          "algo1",
			MLModelConfigName:    "config1",
			ModelName:            "model1",
			NodeId:               "node1",
			DeploymentStatusCode: ml_model.ReadyToDeploy,
			DeploymentStatus:     ml_model.ReadyToDeploy.String(),
			ModelVersion:         1,
			PermittedOption:      "deploy",
		},
	}

	// Case 1: Matching deployment found by NodeId and MLModelConfigName
	modelAndVersionKey := "config1:v1"
	nodeName := "node1"
	nodeDisplayName := "Node One"

	result := buildMatchingDeployment(deployments, modelAndVersionKey, nodeName, nodeDisplayName)
	if result == nil {
		t.Errorf("Expected matching deployment, got nil")
		return
	}
	if result.NodeId != nodeName {
		t.Errorf("Expected NodeId to be %s, got %s", nodeName, result.NodeId)
	}

	if result.NodeDisplayName != nodeDisplayName {
		t.Errorf("Expected NodeDisplayName to be %s, got %s", nodeDisplayName, result.NodeDisplayName)
	}

	// Case 2: No matching deployment found by NodeId and MLModelConfigName
	modelAndVersionKeyNotFound := "config2:v2"
	nodeNameNotFound := "node2"
	nodeDisplayNameNotFound := "Node Two"

	resultNotFound := buildMatchingDeployment(deployments, modelAndVersionKeyNotFound, nodeNameNotFound, nodeDisplayNameNotFound)
	if resultNotFound != nil {
		t.Errorf("Expected no matching deployment, got %v", resultNotFound)
	}

	// Case 3: Matching deployment found
	modelAndVersionKey = "config1:v1"
	nodeName = "node2"
	nodeDisplayName = "Node Two"

	result = buildMatchingDeployment(deployments, modelAndVersionKey, nodeName, nodeDisplayName)
	if result == nil {
		t.Errorf("Expected matching deployment, got nil")
		return
	}
	if result.NodeId != nodeName {
		t.Errorf("Expected NodeId to be %s, got %s", nodeName, result.NodeId)
	}

	if result.NodeDisplayName != nodeDisplayName {
		t.Errorf("Expected NodeDisplayName to be %s, got %s", nodeDisplayName, result.NodeDisplayName)
	}
}

func Test_buildNewDeploymentStatusRecord(t *testing.T) {
	model := ml_model.MLModel{
		MLAlgorithm:       "algorithm1",
		MLModelConfigName: "config1",
		ModelName:         "model1",
		ModelVersion:      1,
		IsModelDeprecated: false,
	}
	node := models.Node{
		NodeId:   "node1",
		Name:     "Node One",
		HostName: "node1.bmc.com",
	}

	result := buildNewDeploymentStatusRecord(model, node)
	expected := ml_model.ModelDeploymentStatus{
		MLAlgorithm:          "algorithm1",
		MLModelConfigName:    "config1",
		ModelName:            "model1",
		NodeId:               "node1",
		NodeDisplayName:      "node1.bmc.com",
		DeploymentStatusCode: ml_model.ReadyToDeploy,
		DeploymentStatus:     ml_model.ReadyToDeploy.String(),
		ModelVersion:         1,
		PermittedOption:      "deploy",
		IsModelDeprecated:    false,
	}

	if result != expected {
		t.Errorf("BuildNewDeploymentStatusRecord() returned %v, expected %v", result, expected)
	}
}

func Test_buildNodeTree(t *testing.T) {
	t.Run("buildNodeTree - Passed without nodeGroup.Node", func(t *testing.T) {
		nodeGroup := &models.NodeGroup{
			DisplayName:     "Root",
			ChildNodeGroups: nil,
			Node:            nil,
		}

		latestModelVersionKeys := []string{"algo1:v1", "algo2:v2"}
		deployments := []ml_model.ModelDeploymentStatus{
			{
				MLAlgorithm:          "algo1",
				MLModelConfigName:    "config1",
				ModelName:            "model1",
				NodeId:               "node1",
				DeploymentStatusCode: ml_model.ReadyToDeploy,
				DeploymentStatus:     ml_model.ReadyToDeploy.String(),
				ModelVersion:         1,
				PermittedOption:      "deploy",
				IsModelDeprecated:    false,
			},
		}
		result := buildNodeTree(nodeGroup, latestModelVersionKeys, deployments, "")
		deploymentsByGroup, ok := result.([]ml_model.DeploymentsByGroup)
		if !ok {
			t.Errorf("buildNodeTree() Unexpected result type. Expected []ml_model.DeploymentsByGroup, got %T", result)
		}
		if len(deploymentsByGroup) != 1 {
			t.Errorf("buildNodeTree() Expected 1 deployment group, got %d", len(deploymentsByGroup))
		}
		firstGroup := deploymentsByGroup[0]
		if firstGroup.GroupField != "NodeGroup" {
			t.Errorf("buildNodeTree() Expected GroupField to be 'NodeGroup', got %s", firstGroup.GroupField)
		}

		if firstGroup.GroupValue != "Root" {
			t.Errorf("buildNodeTree() Expected GroupValue to be 'Root', got %s", firstGroup.GroupValue)
		}
	})
	t.Run("buildNodeTree - Passed with nodeGroup.Node", func(t *testing.T) {
		node := models.Node{
			NodeId: "node",
		}
		nodeGroup := &models.NodeGroup{
			Node: &node,
		}
		latestModelVersionKeys := []string{"config1:v1"}
		deployments := []ml_model.ModelDeploymentStatus{
			{
				MLAlgorithm:          "algo1",
				MLModelConfigName:    "config1",
				ModelName:            "model1",
				NodeId:               "node",
				DeploymentStatusCode: ml_model.ReadyToDeploy,
				DeploymentStatus:     ml_model.ReadyToDeploy.String(),
				ModelVersion:         1,
				PermittedOption:      "deploy",
				IsModelDeprecated:    false,
			},
		}
		result := buildNodeTree(nodeGroup, latestModelVersionKeys, deployments, "")
		modelDeploymentStatus, ok := result.([]ml_model.ModelDeploymentStatus)
		if !ok {
			t.Errorf("buildNodeTree() Unexpected result type. Expected []ml_model.ModelDeploymentStatus, got %T", result)
		}
		assert.Len(t, modelDeploymentStatus, 1)
		assert.Equal(t, deployments[0].MLAlgorithm, modelDeploymentStatus[0].MLAlgorithm)
		assert.Equal(t, deployments[0].MLModelConfigName, modelDeploymentStatus[0].MLModelConfigName)
		assert.Equal(t, node.NodeId, modelDeploymentStatus[0].NodeId)
	})
	t.Run("buildNodeTree - Passed with ChildNodeGroups and DeploymentsByGroup", func(t *testing.T) {
		latestModelVersionKeys := []string{"algo1:v1"}
		deployments := []ml_model.ModelDeploymentStatus{
			{
				MLAlgorithm:          "algo1",
				MLModelConfigName:    "config1",
				ModelName:            "model1",
				NodeId:               "node1",
				DeploymentStatusCode: ml_model.ReadyToDeploy,
				DeploymentStatus:     ml_model.ReadyToDeploy.String(),
				ModelVersion:         1,
				PermittedOption:      "deploy",
				IsModelDeprecated:    false,
			},
		}
		childNodeGroups := []models.NodeGroup{
			{
				DisplayName:     "Root",
				ChildNodeGroups: nil,
				Node:            nil,
			},
		}
		nodeGroup := &models.NodeGroup{
			DisplayName:     "Root",
			ChildNodeGroups: childNodeGroups,
			Node:            nil,
		}
		result := buildNodeTree(nodeGroup, latestModelVersionKeys, deployments, "")
		deploymentsByGroup, ok := result.([]ml_model.DeploymentsByGroup)
		if !ok {
			t.Errorf("buildNodeTree() Unexpected result type. Expected []ml_model.DeploymentsByGroup, got %T", result)
		}
		if len(deploymentsByGroup) != 1 {
			t.Errorf("buildNodeTree() Expected 1 deployment group, got %d", len(deploymentsByGroup))
		}
		firstGroup := deploymentsByGroup[0]
		if firstGroup.GroupField != "NodeGroup" {
			t.Errorf("buildNodeTree() Expected GroupField to be 'NodeGroup', got %s", firstGroup.GroupField)
		}

		if firstGroup.GroupValue != "Root" {
			t.Errorf("buildNodeTree() Expected GroupValue to be 'Root', got %s", firstGroup.GroupValue)
		}
	})
	t.Run("buildNodeTree - Passed with ChildNodeGroups and ModelDeploymentStatus", func(t *testing.T) {
		node := models.Node{
			NodeId: "node",
		}
		latestModelVersionKeys := []string{"config1:v1"}
		deployments := []ml_model.ModelDeploymentStatus{
			{
				MLAlgorithm:          "algo1",
				MLModelConfigName:    "config1",
				ModelName:            "model1",
				NodeId:               "node",
				DeploymentStatusCode: ml_model.ReadyToDeploy,
				DeploymentStatus:     ml_model.ReadyToDeploy.String(),
				ModelVersion:         1,
				PermittedOption:      "deploy",
				IsModelDeprecated:    false,
			},
		}
		childNodeGroups := []models.NodeGroup{
			{
				DisplayName:     "Root",
				ChildNodeGroups: nil,
				Node:            &node,
			},
		}
		nodeGroup := &models.NodeGroup{
			DisplayName:     "Root",
			ChildNodeGroups: childNodeGroups,
			Node:            nil,
		}
		result := buildNodeTree(nodeGroup, latestModelVersionKeys, deployments, "")
		deploymentsByGroup, ok := result.([]ml_model.DeploymentsByGroup)
		if !ok {
			t.Errorf("buildNodeTree() Unexpected result type. Expected []ml_model.DeploymentsByGroup, got %T", result)
		}
		if len(deploymentsByGroup) != 1 {
			t.Errorf("buildNodeTree() Expected 1 deployment group, got %d", len(deploymentsByGroup))
		}
		firstGroup := deploymentsByGroup[0]
		if firstGroup.GroupField != "NodeGroup" {
			t.Errorf("buildNodeTree() Expected GroupField to be 'NodeGroup', got %s", firstGroup.GroupField)
		}

		if firstGroup.GroupValue != "Root" {
			t.Errorf("buildNodeTree() Expected GroupValue to be 'Root', got %s", firstGroup.GroupValue)
		}
	})
}

func TestTrainingJobService_SetHttpClient(t *testing.T) {
	js := &TrainingJobService{}
	js.SetHttpClient()
	if js.client == nil {
		t.Error("Expected non-nil http.Client, got nil")
	}
	if _, ok := js.client.(*http.Client); !ok {
		t.Error("Expected *http.Client, got different type")
	}
}

func TestGetDbClient(t *testing.T) {
	ts := &TrainingJobService{
		dbClient: &dBMockClient,
	}
	dbClient := ts.GetDbClient()
	assert.Equal(t, &dBMockClient, dbClient, "Expected the returned DB client to be the mock client")
}

func TestTrainingJobService_validateMLEventConfigByAlgoType(t *testing.T) {
	Init()
	js := &TrainingJobService{
		service:  u.AppService,
		dbClient: &dBMockClient,
	}
	dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything, mock.Anything).Return([]config.MLEventConfig{}, nil)
	conditions1 := []config.Condition{
		{
			SeverityLevel: models.SEVERITY_MAJOR,
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Operator:       config.GREATER_THAN_OPERATOR,
					ThresholdValue: 20.0,
				},
			},
		},
	}
	conditions2 := []config.Condition{
		{
			SeverityLevel: models.SEVERITY_MAJOR,
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:          "WindTurbine#WindSpeed",
					Operator:       config.GREATER_THAN_OPERATOR,
					ThresholdValue: 20.0,
				},
			},
		},
	}
	anomalyEventConfig := config.MLEventConfig{
		MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
		Conditions:                 conditions1,
		StabilizationPeriodByCount: 3,
	}
	anomalyEventConfigToFail := config.MLEventConfig{
		MLAlgorithmType: helpers.ANOMALY_ALGO_TYPE,
		Conditions: []config.Condition{
			{
				SeverityLevel: models.SEVERITY_MAJOR,
				ThresholdsDefinitions: []config.ThresholdDefinition{
					{
						Operator:       config.GREATER_THAN_OPERATOR,
						ThresholdValue: 20.0,
					},
					{
						Operator:       config.LESS_THAN_OPERATOR,
						ThresholdValue: 10.0,
					},
				},
			},
		},
		StabilizationPeriodByCount: 3,
	}
	classificationEventConfig := config.MLEventConfig{
		MLAlgorithmType: helpers.CLASSIFICATION_ALGO_TYPE,
		Conditions: []config.Condition{
			{
				SeverityLevel: models.SEVERITY_MAJOR,
				ThresholdsDefinitions: []config.ThresholdDefinition{
					{
						Label:          "Predicted class",
						Operator:       config.EQUAL_TO_OPERATOR,
						ThresholdValue: "fault",
					},
					{
						Label:          "Confidence",
						Operator:       config.GREATER_THAN_OPERATOR,
						ThresholdValue: 20.0,
					},
				},
			},
		},
		StabilizationPeriodByCount: 3,
	}
	classificationEventConfigToFail := config.MLEventConfig{
		MLAlgorithmType: helpers.CLASSIFICATION_ALGO_TYPE,
		Conditions: []config.Condition{
			{
				SeverityLevel: models.SEVERITY_MAJOR,
				ThresholdsDefinitions: []config.ThresholdDefinition{
					{
						Label:          "Confidence",
						Operator:       config.GREATER_THAN_OPERATOR,
						ThresholdValue: 20.0,
					},
				},
			},
		},
		StabilizationPeriodByCount: 3,
	}
	timeseriesEventConfig := config.MLEventConfig{
		MLAlgorithmType:            helpers.TIMESERIES_ALGO_TYPE,
		Conditions:                 conditions2,
		StabilizationPeriodByCount: 1,
	}
	timeseriesEventConfigToFail := config.MLEventConfig{
		MLAlgorithmType:            helpers.TIMESERIES_ALGO_TYPE,
		Conditions:                 conditions1,
		StabilizationPeriodByCount: 1,
	}
	regressionEventConfig := config.MLEventConfig{
		MLAlgorithmType:            helpers.REGRESSION_ALGO_TYPE,
		Conditions:                 conditions2,
		StabilizationPeriodByCount: 3,
	}
	regressionEventConfigToFail := config.MLEventConfig{
		MLAlgorithmType:            helpers.REGRESSION_ALGO_TYPE,
		Conditions:                 conditions1,
		StabilizationPeriodByCount: 3,
	}
	unknownTypeEventConfig := config.MLEventConfig{
		MLAlgorithmType: "UNKNOWN_TYPE",
		Conditions: []config.Condition{
			{
				SeverityLevel: models.SEVERITY_MAJOR,
				ThresholdsDefinitions: []config.ThresholdDefinition{
					{
						Label:          "WindTurbine#WindSpeed",
						Operator:       config.GREATER_THAN_OPERATOR,
						ThresholdValue: 20.0,
					},
				},
			},
		},
	}
	tests := []struct {
		name          string
		mlEventConfig config.MLEventConfig
		expectedErr   string
	}{
		{"validateMLEventConfigByAlgoType - Passed (Anomaly)", anomalyEventConfig, ""},
		{"validateMLEventConfigByAlgoType - Failed (Anomaly)", anomalyEventConfigToFail, "ML event config validation failed: invalid amount of thresholds provided per condition - 2 (maximum allowed for Anomaly type - 1)"},
		{"validateMLEventConfigByAlgoType - Failed (Anomaly, common)", config.MLEventConfig{MLAlgorithmType: helpers.ANOMALY_ALGO_TYPE}, "ML event config validation failed: invalid amount of conditions provided - 0 (expected at least 1 condition)"},
		{"validateMLEventConfigByAlgoType - Passed (Classification)", classificationEventConfig, ""},
		{"validateMLEventConfigByAlgoType - Failed (Classification)", classificationEventConfigToFail, "ML event config validation failed: invalid amount of thresholds provided per condition - 1 (expected for Classification type - 2)"},
		{"validateMLEventConfigByAlgoType - Passed (Timeseries)", timeseriesEventConfig, ""},
		{"validateMLEventConfigByAlgoType - Failed (Timeseries)", timeseriesEventConfigToFail, "ML event config validation failed: feature name provided for prediction (label) is empty"},
		{"validateMLEventConfigByAlgoType - Passed (Regression)", regressionEventConfig, ""},
		{"validateMLEventConfigByAlgoType - Failed (Regression)", regressionEventConfigToFail, "ML event config validation failed: prediction class name provided (label) is empty"},
		{"validateMLEventConfigByAlgoType - Failed (Unknown Algo Type)", unknownTypeEventConfig, "ML event config validation failed: unknown algorithm type: UNKNOWN_TYPE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := js.validateMLEventConfigByAlgoType(tt.mlEventConfig)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}

func TestTrainingJobService_applyCommonValidations(t *testing.T) {
	Init()
	js := &TrainingJobService{
		service: u.AppService,
	}
	tests := []struct {
		name          string
		mlEventConfig config.MLEventConfig
		expectedErr   string
	}{
		{
			"applyCommonValidations - Passed",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL, generateThreshold(config.BETWEEN_OPERATOR, 10.0, 20.0, 0, "")),
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 15.0, "")),
			}, 1),
			"",
		},
		{
			"applyCommonValidations - Failed (No Conditions)",
			generateMLEventConfig([]config.Condition{}, 1),
			"ML event config validation failed: invalid amount of conditions provided - 0 (expected at least 1 condition)",
		},
		{
			"applyCommonValidations - Failed (Duplicate Severity)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 15.0, "")),
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.LESS_THAN_OPERATOR, 0, 0, 10.0, "")),
			}, 1),
			"ML event config validation failed: severity level MAJOR appears more than once",
		},
		{
			"applyCommonValidations - Failed (Missing Thresholds)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL),
			}, 1),
			"ML event config validation failed: condition for severity level CRITICAL must contain at least one threshold, but none is provided",
		},
		{
			"applyCommonValidations - Failed (Invalid Operator)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL, generateThreshold("INVALID_OPERATOR", 0, 0, 20.0, "")),
			}, 1),
			"ML event config validation failed: unknown operator INVALID_OPERATOR in severity level CRITICAL",
		},
		{
			"applyCommonValidations - Failed (Invalid BETWEEN Thresholds)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL, generateThreshold(config.BETWEEN_OPERATOR, 20.0, 10.0, 0, "")),
			}, 1),
			"ML event config validation failed: invalid BETWEEN thresholds: UpperThreshold (10.000000) must be greater than LowerThreshold (20.000000) for severity level CRITICAL",
		},
		{
			"applyCommonValidations - Failed (Absent BETWEEN Thresholds)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL, generateThreshold(config.BETWEEN_OPERATOR, 0, 0, 0, "")),
			}, 1),
			"ML event config validation failed: BETWEEN operator requires both lower and upper thresholds in severity level CRITICAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := js.applyCommonValidations(tt.mlEventConfig)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}

func TestTrainingJobService_applyAnomalyAlgoTypeValidations(t *testing.T) {
	Init()
	js := &TrainingJobService{
		service: u.AppService,
	}
	tests := []struct {
		name          string
		mlEventConfig config.MLEventConfig
		expectedErr   string
	}{
		{
			"applyAnomalyAlgoTypeValidations - Passed",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 50.0, "")),
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.LESS_THAN_OPERATOR, 0, 0, 20.0, "")),
			}, 1),
			"",
		},
		{
			"applyAnomalyAlgoTypeValidations - Failed (Too Many Conditions)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 50.0, "")),
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.LESS_THAN_OPERATOR, 0, 0, 20.0, "")),
				generateCondition(models.SEVERITY_MINOR, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 30.0, "")),
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.LESS_THAN_OPERATOR, 0, 0, 10.0, "")),
			}, 1),
			"ML event config validation failed: invalid amount of conditions provided - 4 (maximum allowed for Anomaly type - 3)",
		},
		{
			"applyAnomalyAlgoTypeValidations - Failed (Multiple Thresholds)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL,
					generateThreshold(config.BETWEEN_OPERATOR, 10.0, 20.0, 0, ""),
					generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 25.0, ""),
				),
			}, 1),
			"ML event config validation failed: invalid amount of thresholds provided per condition - 2 (maximum allowed for Anomaly type - 1)",
		},
		{
			"applyAnomalyAlgoTypeValidations - Failed (Threshold Exceeds Max)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 70000.0, "")),
			}, 1),
			"ML event config validation failed: threshold value exceeds the maximum allowed value of 65535 in severity level - MAJOR",
		},
		{
			"applyAnomalyAlgoTypeValidations - Failed (Overlapping Ranges)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_CRITICAL, generateThreshold(config.BETWEEN_OPERATOR, 10.0, 30.0, 0, "")),
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.BETWEEN_OPERATOR, 20.0, 40.0, 0, "")),
			}, 1),
			"ML event config validation failed: range [20.00, 40.00] for severity MAJOR overlaps with range [10.00, 30.00] for severity CRITICAL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := js.applyAnomalyAlgoTypeValidations(tt.mlEventConfig)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}

func TestTrainingJobService_applyClassificationAlgoTypeValidations(t *testing.T) {
	Init()
	js := &TrainingJobService{
		service: u.AppService,
	}
	tests := []struct {
		name          string
		mlEventConfig config.MLEventConfig
		expectedErr   string
	}{
		{
			"applyClassificationAlgoTypeValidations - Passed",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR,
					generateThreshold(config.EQUAL_TO_OPERATOR, 0, 0, "fault", "Predicted class"),
					generateThreshold(config.LESS_THAN_OPERATOR, 0, 0, 0.5, "Confidence"),
				),
			}, 1),
			"",
		},
		{
			"applyClassificationAlgoTypeValidations - Failed (Too Many Conditions)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.EQUAL_TO_OPERATOR, 0, 0, 0, "ClassA")),
				generateCondition(models.SEVERITY_MINOR, generateThreshold(config.EQUAL_TO_OPERATOR, 0, 0, 0, "ClassB")),
			}, 1),
			"ML event config validation failed: invalid amount of conditions provided - 2 (maximum allowed for Classification type - 1)",
		},
		{
			"applyClassificationAlgoTypeValidations - Failed (Multiple Thresholds)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR,
					generateThreshold(config.EQUAL_TO_OPERATOR, 0, 0, 0, "ClassA"),
					generateThreshold(config.EQUAL_TO_OPERATOR, 0, 0, 0, "ClassB"),
					generateThreshold(config.EQUAL_TO_OPERATOR, 0, 0, 0, "ClassC"),
				),
			}, 1),
			"ML event config validation failed: invalid amount of thresholds provided per condition - 3 (expected for Classification type - 2)",
		},
		{
			"applyClassificationAlgoTypeValidations - Failed (No thresholdValue of type string)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR,
					generateThreshold(config.LESS_THAN_OPERATOR, 0, 0, 0, ""),
					generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 0, "")),
			}, 1),
			"ML event config validation failed: none of the thresholds has a ThresholdValue of type string (class name)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := js.applyClassificationAlgoTypeValidations(tt.mlEventConfig)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}

func TestTrainingJobService_applyTimeseriesAlgoTypeValidations(t *testing.T) {
	Init()
	js := &TrainingJobService{
		service: u.AppService,
	}

	tests := []struct {
		name          string
		mlEventConfig config.MLEventConfig
		expectedErr   string
	}{
		{
			"applyTimeseriesAlgoTypeValidations - Passed",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 0, "Profile1#Feature1")),
			}, 1),
			"",
		},
		{
			"applyTimeseriesAlgoTypeValidations - Failed (Too Many Conditions)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 0, "Profile1#Feature1")),
				generateCondition(models.SEVERITY_MINOR, generateThreshold(config.LESS_THAN_OPERATOR, 0, 0, 0, "Profile1#Feature2")),
			}, 1),
			"ML event config validation failed: invalid amount of conditions provided - 2 (maximum allowed for Timeseries type - 1)",
		},
		{
			"applyTimeseriesAlgoTypeValidations - Failed (Empty Label)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.EQUAL_TO_OPERATOR, 0, 0, 0, "")),
			}, 1),
			"ML event config validation failed: feature name provided for prediction (label) is empty",
		},
		{
			"applyTimeseriesAlgoTypeValidations - Failed (Invalid Label Format)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.EQUAL_TO_OPERATOR, 0, 0, 0, "InvalidLabelFormat")),
			}, 1),
			"ML event config validation failed: threshold.Label 'InvalidLabelFormat' does not match the required pattern: '<P/profile name>#<F/feature name>'",
		},
		{
			"applyTimeseriesAlgoTypeValidations - Failed (Duplicate Labels)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR,
					generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 0, "Profile1#Feature1"),
					generateThreshold(config.LESS_THAN_OPERATOR, 0, 0, 0, "Profile1#Feature1")),
			}, 1),
			"ML event config validation failed: duplicated feature name Profile1#Feature1 in thresholds for severity level MAJOR",
		},
		{
			"applyTimeseriesAlgoTypeValidations - Failed (Invalid StabilizationPeriodByCount)",
			generateMLEventConfig([]config.Condition{
				generateCondition(models.SEVERITY_MAJOR, generateThreshold(config.GREATER_THAN_OPERATOR, 0, 0, 0, "Profile1#Feature1")),
			}, 2),
			"ML event config validation failed: stabilizationPeriodByCount must be 1 for Timeseries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := js.applyTimeseriesAlgoTypeValidations(tt.mlEventConfig)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}

func TestTrainingJobService_applyRegressionAlgoTypeValidations(t *testing.T) {
	Init()
	js := &TrainingJobService{
		service: u.AppService,
	}
	validCondition := []config.Condition{
		{
			SeverityLevel: models.SEVERITY_MAJOR,
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:    "Profile1#Feature1",
					Operator: config.GREATER_THAN_OPERATOR,
				},
			},
		},
	}
	tooManyConditions := []config.Condition{
		{
			SeverityLevel: models.SEVERITY_MAJOR,
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:    "Profile1#Feature1",
					Operator: config.GREATER_THAN_OPERATOR,
				},
			},
		},
		{
			SeverityLevel: models.SEVERITY_MINOR,
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:    "Profile1#Feature2",
					Operator: config.LESS_THAN_OPERATOR,
				},
			},
		},
	}
	tooManyThresholds := []config.Condition{
		{
			SeverityLevel: models.SEVERITY_MAJOR,
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:    "Profile1#Feature1",
					Operator: config.GREATER_THAN_OPERATOR,
				},
				{
					Label:    "Profile1#Feature2",
					Operator: config.LESS_THAN_OPERATOR,
				},
			},
		},
	}
	emptyLabelCondition := []config.Condition{
		{
			SeverityLevel: models.SEVERITY_MAJOR,
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:    "",
					Operator: config.EQUAL_TO_OPERATOR,
				},
			},
		},
	}
	invalidLabelFormatCondition := []config.Condition{
		{
			SeverityLevel: models.SEVERITY_MAJOR,
			ThresholdsDefinitions: []config.ThresholdDefinition{
				{
					Label:    "InvalidLabelFormat",
					Operator: config.EQUAL_TO_OPERATOR,
				},
			},
		},
	}
	tests := []struct {
		name          string
		mlEventConfig config.MLEventConfig
		expectedErr   string
	}{
		{"applyRegressionAlgoTypeValidations - Passed", config.MLEventConfig{Conditions: validCondition}, ""},
		{"applyRegressionAlgoTypeValidations - Failed (Too Many Conditions)", config.MLEventConfig{Conditions: tooManyConditions}, "ML event config validation failed: invalid amount of conditions provided - 2 (maximum allowed for Regression type - 1)"},
		{"applyRegressionAlgoTypeValidations - Failed (Too Many Thresholds)", config.MLEventConfig{Conditions: tooManyThresholds}, "ML event config validation failed: invalid amount of thresholds provided per condition - 2 (maximum allowed for Regression type - 1)"},
		{"applyRegressionAlgoTypeValidations - Failed (Empty Label)", config.MLEventConfig{Conditions: emptyLabelCondition}, "ML event config validation failed: prediction class name provided (label) is empty"},
		{"applyRegressionAlgoTypeValidations - Failed (Invalid Label Format)", config.MLEventConfig{Conditions: invalidLabelFormatCondition}, "ML event config validation failed: threshold.Label 'InvalidLabelFormat' does not match the required pattern: '<P/profile name>#<F/feature name>'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := js.applyRegressionAlgoTypeValidations(tt.mlEventConfig)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}

func TestTrainingJobService_GetLatestJobSummary(t *testing.T) {
	t.Run("GetLatestJobSummary - Passed", func(t *testing.T) {
		Init()
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"
		jobs := []job.TrainingJobDetails{
			{Name: "job-1", MLAlgorithm: algoName, StartTime: 170150605, MLModelConfigName: modelConfigName, Status: "Created"},
			{Name: "job-2", MLAlgorithm: algoName, StartTime: 170516780, MLModelConfigName: modelConfigName, Status: "InProgress"},
		}

		dBMockClient.On("GetMLTrainingJobsByConfig", mock.Anything, mock.Anything).Return(jobs, nil)

		jobSummary := job.TrainingJobSummary{
			Name:              "job-2",
			MLAlgorithm:       algoName,
			MLModelConfigName: modelConfigName,
			Status:            "InProgress",
			StartTime:         170516780,
		}
		js := &TrainingJobService{
			service:  u.AppService,
			dbClient: &dBMockClient,
		}
		got, err := js.GetLatestJobSummary(algoName, modelConfigName, "")
		if err != nil {
			t.Errorf("GetLatestJobSummary() error = %v, wantErr false", err)
		}
		if !reflect.DeepEqual(got, &jobSummary) {
			t.Errorf("GetLatestJobSummary() got: %v, want: %v", got, &jobSummary)
		}
	})
	t.Run("GetLatestJobSummary - GetJobSummaryFailed", func(t *testing.T) {
		Init()
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"
		errMsg := "mock error"
		dBMockClient.On("GetMLTrainingJobsByConfig", mock.Anything, mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, errMsg))
		js := &TrainingJobService{
			service:  u.AppService,
			dbClient: &dBMockClient,
		}
		_, err := js.GetLatestJobSummary(algoName, modelConfigName, "")
		if err == nil || err.Message() != errMsg {
			t.Errorf("GetLatestJobSummary() expected error message: %s, received error: %v", errMsg, err)
		}
	})
}

func TestTrainingJobService_GetModelDeploymentsGroupedByNode(t *testing.T) {
	t.Run("GetModelDeploymentsGroupedByNode - Passed", func(t *testing.T) {
		Init()
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"
		nodeId := "MockNode"
		modelVersion := int64(1)

		nodes := []models.Node{
			{
				NodeId: nodeId,
			},
		}

		mockBody, _ := json.Marshal(nodes)

		httpResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       nil,
		}

		runFunc := func(args mock.Arguments) {
			httpResponse.Body = io.NopCloser(bytes.NewBuffer(mockBody))
		}

		mockMlModels := []ml_model.MLModel{
			{
				MLAlgorithm:       algoName,
				MLModelConfigName: modelConfigName,
				ModelVersion:      modelVersion,
			},
		}

		mockHttpClient.On("Do", mock.Anything).Run(runFunc).Return(httpResponse, nil)

		dBMockClient.On("GetModels", mock.Anything, mock.Anything).Return(mockMlModels, nil)
		dBMockClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).Return([]ml_model.ModelDeploymentStatus{}, nil)
		dBMockClient.On("GetLatestModelsByConfig", mock.Anything, mock.Anything).Return(mockMlModels, nil)

		js := &TrainingJobService{
			service:   u.AppService,
			appConfig: appConfig,
			dbClient:  &dBMockClient,
			client:    &mockHttpClient,
		}
		got, err := js.GetModelDeploymentsGroupedByNode(algoName, modelConfigName, "", "")
		if err != nil {
			t.Errorf("GetModelDeploymentsGroupedByNode() error = %v, wantErr false", err)
			t.FailNow()
		}
		if got == nil || len(got) != 1 {
			t.Errorf("GetModelDeploymentsGroupedByNode() got empty slice: %v", got)
			t.FailNow()
		}
		assert.Equal(t, got[0].MLModelConfigName, modelConfigName)
		assert.Equal(t, got[0].MLAlgorithm, algoName)
		assert.Equal(t, got[0].ModelVersion, modelVersion)
		assert.Equal(t, got[0].NodeId, nodeId)
	})
	t.Run("GetModelDeploymentsGroupedByNode - GetNodesFailed", func(t *testing.T) {
		Init()
		mockHttpClient.On("Do", mock.Anything).Return(nil, errors.New("mock error"))
		js := &TrainingJobService{
			service:   u.AppService,
			appConfig: appConfig,
			dbClient:  &dBMockClient,
			client:    &mockHttpClient,
		}
		errMsg := "Error getting list of nodes"
		_, err := js.GetModelDeploymentsGroupedByNode("", "", "", "")
		if err == nil || err.Message() != errMsg {
			t.Errorf("GetModelDeploymentsGroupedByNode() expected error message: %s, received error: %v", errMsg, err)
		}
	})
	t.Run("GetModelDeploymentsGroupedByNode - GetModelDeploymentsByConfigFailed", func(t *testing.T) {
		Init()
		errMsg := "mock error"

		mockBody, _ := json.Marshal([]models.Node{})

		httpResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		}
		mockHttpClient.On("Do", mock.Anything).Return(httpResponse, nil)

		dBMockClient.On("GetModels", mock.Anything, mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, errMsg))

		js := &TrainingJobService{
			service:   u.AppService,
			appConfig: appConfig,
			dbClient:  &dBMockClient,
			client:    &mockHttpClient,
		}
		_, err := js.GetModelDeploymentsGroupedByNode("", "", "", "")
		if err == nil || err.Message() != errMsg {
			t.Errorf("GetModelDeploymentsGroupedByNode() expected error message: %s, received error: %v", errMsg, err)
		}
	})
	t.Run("GetModelDeploymentsGroupedByNode - GetModelsFailed", func(t *testing.T) {
		Init()
		errMsg := "mock error"

		mockBody, _ := json.Marshal([]models.Node{})

		httpResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       nil,
		}
		runFunc := func(args mock.Arguments) {
			httpResponse.Body = io.NopCloser(bytes.NewBuffer(mockBody))
		}
		mockHttpClient.On("Do", mock.Anything).Run(runFunc).Return(httpResponse, nil)

		dBMockClient.On("GetModels", mock.Anything, mock.Anything).Return([]ml_model.MLModel{}, nil)
		dBMockClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).Return([]ml_model.ModelDeploymentStatus{}, nil)
		dBMockClient.On("GetLatestModelsByConfig", mock.Anything, mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, errMsg))

		js := &TrainingJobService{
			service:   u.AppService,
			appConfig: appConfig,
			dbClient:  &dBMockClient,
			client:    &mockHttpClient,
		}
		_, err := js.GetModelDeploymentsGroupedByNode("", "", "", "")
		if err == nil || err.Message() != errMsg {
			t.Errorf("GetModelDeploymentsGroupedByNode() expected error message: %s, received error: %v", errMsg, err)
		}
	})
}

func TestTrainingJobService_AddMLEventConfig(t *testing.T) {
	t.Run("AddMLEventConfig - Passed", func(t *testing.T) {
		Init()
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"

		conditions := []config.Condition{
			{
				ThresholdsDefinitions: []config.ThresholdDefinition{
					{
						Operator:       config.GREATER_THAN_OPERATOR,
						ThresholdValue: 0,
					},
				},
			},
		}

		eventConfig := config.MLEventConfig{
			MLAlgorithm:                algoName,
			MlModelConfigName:          modelConfigName,
			MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
			StabilizationPeriodByCount: 1,
			Conditions:                 conditions,
		}

		nodes := []models.Node{
			{
				NodeId: "MockNode",
			},
		}

		mockBody, _ := json.Marshal(nodes)

		httpResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		}

		mockHttpClient.On("Do", mock.Anything).Return(httpResponse, nil)

		dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).Return([]config.MLEventConfig{}, nil)
		dBMockClient.On("AddMLEventConfig", mock.Anything).Return(eventConfig, nil)

		mockMqttSender.On("MQTTSend", mock.Anything, mock.Anything).Return(true, nil)
		u.AppService.On("BuildContext", "gg", "ml").Return(u.AppFunctionContext)

		js := &TrainingJobService{
			service:    u.AppService,
			appConfig:  appConfig,
			dbClient:   &dBMockClient,
			client:     &mockHttpClient,
			mqttSender: mockMqttSender,
		}
		err := js.AddMLEventConfig(eventConfig)
		if err != nil {
			t.Errorf("AddMLEventConfig() error = %v, wantErr false", err)
			t.FailNow()
		}
	})
	t.Run("AddMLEventConfig - StabilizationPeriodByCountFailed", func(t *testing.T) {
		Init()
		errMsg := "stabilization period count needs to be more than 0"
		eventConfig := config.MLEventConfig{
			StabilizationPeriodByCount: 0,
		}
		js := &TrainingJobService{
			service: u.AppService,
		}
		err := js.AddMLEventConfig(eventConfig)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("AddMLEventConfig - GetAllMLEventConfigsByConfigFailed", func(t *testing.T) {
		Init()
		modelConfigName := "MockModelConfig"
		eventConfig := config.MLEventConfig{
			MlModelConfigName:          modelConfigName,
			MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
			StabilizationPeriodByCount: 1,
		}
		errMsg := fmt.Sprintf("ML event config validation failed: error getting all ML event configs for the ML model %s", eventConfig.MlModelConfigName)
		dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""))
		js := &TrainingJobService{
			service:  u.AppService,
			dbClient: &dBMockClient,
		}
		err := js.AddMLEventConfig(eventConfig)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("AddMLEventConfig - ExistingEventConfigsLenFailed", func(t *testing.T) {
		Init()
		eventConfig := config.MLEventConfig{
			MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
			StabilizationPeriodByCount: 1,
		}
		errMsg := "ML event config validation failed: only 1 ML event config allowed for algorithm of type Anomaly"
		dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).Return([]config.MLEventConfig{{}}, nil)
		js := &TrainingJobService{
			service:  u.AppService,
			dbClient: &dBMockClient,
		}
		err := js.AddMLEventConfig(eventConfig)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("AddMLEventConfig - ValidateMLEventConfigByAlgoTypeFailed", func(t *testing.T) {
		Init()
		eventConfig := config.MLEventConfig{
			MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
			StabilizationPeriodByCount: 1,
		}
		errMsg := "ML event config validation failed: invalid amount of conditions provided - 0 (expected at least 1 condition)"
		dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).Return([]config.MLEventConfig{}, nil)
		js := &TrainingJobService{
			service:  u.AppService,
			dbClient: &dBMockClient,
		}
		err := js.AddMLEventConfig(eventConfig)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("AddMLEventConfig - AddMLEventConfigFailed", func(t *testing.T) {
		Init()
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"
		eventName := "MockEvent"

		conditions := []config.Condition{
			{
				ThresholdsDefinitions: []config.ThresholdDefinition{
					{
						Operator: config.GREATER_THAN_OPERATOR,
					},
				},
			},
		}

		eventConfig := config.MLEventConfig{
			EventName:                  eventName,
			MLAlgorithm:                algoName,
			MlModelConfigName:          modelConfigName,
			MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
			StabilizationPeriodByCount: 1,
			Conditions:                 conditions,
		}
		errMsg := fmt.Sprintf("Error creating ML event config %s for ML model config %s", eventConfig.EventName, eventConfig.MlModelConfigName)
		dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).Return([]config.MLEventConfig{}, nil)
		dBMockClient.On("AddMLEventConfig", mock.Anything).Return(eventConfig, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""))

		js := &TrainingJobService{
			service:  u.AppService,
			dbClient: &dBMockClient,
		}
		err := js.AddMLEventConfig(eventConfig)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("AddMLEventConfig - GetNodesFailed", func(t *testing.T) {
		Init()
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"

		conditions := []config.Condition{
			{
				ThresholdsDefinitions: []config.ThresholdDefinition{
					{
						Operator: config.GREATER_THAN_OPERATOR,
					},
				},
			},
		}

		eventConfig := config.MLEventConfig{
			MLAlgorithm:                algoName,
			MlModelConfigName:          modelConfigName,
			MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
			StabilizationPeriodByCount: 1,
			Conditions:                 conditions,
		}
		errMsg := "Error getting list of nodes"
		dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).Return([]config.MLEventConfig{}, nil)
		dBMockClient.On("AddMLEventConfig", mock.Anything).Return(eventConfig, nil)

		mockHttpClient.On("Do", mock.Anything).Return(nil, errors.New("mock error"))

		js := &TrainingJobService{
			service:   u.AppService,
			appConfig: appConfig,
			dbClient:  &dBMockClient,
			client:    &mockHttpClient,
		}
		err := js.AddMLEventConfig(eventConfig)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("AddMLEventConfig - PublishModelDeploymentCommandFailed", func(t *testing.T) {
		Init()
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"

		conditions := []config.Condition{
			{
				ThresholdsDefinitions: []config.ThresholdDefinition{
					{
						Operator: config.GREATER_THAN_OPERATOR,
					},
				},
			},
		}

		eventConfig := config.MLEventConfig{
			MLAlgorithm:                algoName,
			MlModelConfigName:          modelConfigName,
			MLAlgorithmType:            helpers.ANOMALY_ALGO_TYPE,
			StabilizationPeriodByCount: 1,
			Conditions:                 conditions,
		}

		nodes := []models.Node{
			{
				NodeId: "MockNode",
			},
		}

		mockBody, _ := json.Marshal(nodes)

		httpResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		}
		mockHttpClient.On("Do", mock.Anything).Return(httpResponse, nil)

		dBMockClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).Return([]config.MLEventConfig{}, nil)
		dBMockClient.On("AddMLEventConfig", mock.Anything).Return(eventConfig, nil)

		mockMqttSender.On("MQTTSend", mock.Anything, mock.Anything).Return(false, nil)
		u.AppService.On("BuildContext", "gg", "ml").Return(u.AppFunctionContext)

		js := &TrainingJobService{
			service:    u.AppService,
			appConfig:  appConfig,
			dbClient:   &dBMockClient,
			client:     &mockHttpClient,
			mqttSender: mockMqttSender,
		}
		err := js.AddMLEventConfig(eventConfig)
		if err != nil {
			t.Errorf("AddMLEventConfig() error = %v, wantErr false", err)
			t.FailNow()
		}
	})
}

func TestTrainingJobService_DeleteMLEventConfig(t *testing.T) {
	t.Run("DeleteMLEventConfig - Passed", func(t *testing.T) {
		Init()
		eventName := "MockEvent"
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"

		eventConfig := config.MLEventConfig{
			EventName:         eventName,
			MLAlgorithm:       algoName,
			MlModelConfigName: modelConfigName,
		}

		nodes := []models.Node{
			{
				NodeId: "MockNode",
			},
		}

		mockBody, _ := json.Marshal(nodes)

		httpResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		}
		mockHttpClient.On("Do", mock.Anything).Return(httpResponse, nil)

		dBMockClient.On("GetMLEventConfigByName", algoName, modelConfigName, eventName).Return(eventConfig, nil)
		dBMockClient.On("DeleteMLEventConfigByName", algoName, modelConfigName, eventName).Return(nil)

		mockMqttSender.On("MQTTSend", mock.Anything, mock.Anything).Return(true, nil)
		u.AppService.On("BuildContext", "gg", "ml").Return(u.AppFunctionContext)

		js := &TrainingJobService{
			service:    u.AppService,
			appConfig:  appConfig,
			dbClient:   &dBMockClient,
			client:     &mockHttpClient,
			mqttSender: mockMqttSender,
		}
		err := js.DeleteMLEventConfig(algoName, modelConfigName, eventName)
		if err != nil {
			t.Errorf("DeleteMLEventConfig() error = %v, wantErr false", err)
			t.FailNow()
		}
	})
	t.Run("DeleteMLEventConfig - GetMLEventConfigByNameFailed", func(t *testing.T) {
		Init()
		eventName := "MockEvent"
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"
		errMsg := "mock error"
		dBMockClient.On("GetMLEventConfigByName", algoName, modelConfigName, eventName).Return(config.MLEventConfig{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, errMsg))
		js := &TrainingJobService{
			service:  u.AppService,
			dbClient: &dBMockClient,
		}
		err := js.DeleteMLEventConfig(algoName, modelConfigName, eventName)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("DeleteMLEventConfig - DeleteMLEventConfigByNameFailed", func(t *testing.T) {
		Init()
		eventName := "MockEvent"
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"
		errMsg := "mock error"
		eventConfig := config.MLEventConfig{
			EventName:         eventName,
			MLAlgorithm:       algoName,
			MlModelConfigName: modelConfigName,
		}
		dBMockClient.On("GetMLEventConfigByName", algoName, modelConfigName, eventName).Return(eventConfig, nil)
		dBMockClient.On("DeleteMLEventConfigByName", algoName, modelConfigName, eventName).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, errMsg))
		js := &TrainingJobService{
			service:  u.AppService,
			dbClient: &dBMockClient,
		}
		err := js.DeleteMLEventConfig(algoName, modelConfigName, eventName)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("DeleteMLEventConfig - GetNodesFailed", func(t *testing.T) {
		Init()
		eventName := "MockEvent"
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"
		errMsg := "Error getting nodes for SyncMLEventConfig command"
		eventConfig := config.MLEventConfig{
			EventName:         eventName,
			MLAlgorithm:       algoName,
			MlModelConfigName: modelConfigName,
		}

		mockHttpClient.On("Do", mock.Anything).Return(nil, errors.New("mock error"))

		dBMockClient.On("GetMLEventConfigByName", algoName, modelConfigName, eventName).Return(eventConfig, nil)
		dBMockClient.On("DeleteMLEventConfigByName", algoName, modelConfigName, eventName).Return(nil)

		js := &TrainingJobService{
			service:   u.AppService,
			appConfig: appConfig,
			dbClient:  &dBMockClient,
			client:    &mockHttpClient,
		}
		err := js.DeleteMLEventConfig(algoName, modelConfigName, eventName)
		assert.NotNil(t, err)
		assert.Equal(t, errMsg, err.Message())
	})
	t.Run("DeleteMLEventConfig - PublishModelDeploymentCommandFailed", func(t *testing.T) {
		Init()
		eventName := "MockEvent"
		algoName := "MockAlgo"
		modelConfigName := "MockModelConfig"

		eventConfig := config.MLEventConfig{
			EventName:         eventName,
			MLAlgorithm:       algoName,
			MlModelConfigName: modelConfigName,
		}

		nodes := []models.Node{
			{
				NodeId: "MockNode",
			},
		}

		mockBody, _ := json.Marshal(nodes)

		httpResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		}
		mockHttpClient.On("Do", mock.Anything).Return(httpResponse, nil)

		dBMockClient.On("GetMLEventConfigByName", algoName, modelConfigName, eventName).Return(eventConfig, nil)
		dBMockClient.On("DeleteMLEventConfigByName", algoName, modelConfigName, eventName).Return(nil)

		mockMqttSender.On("MQTTSend", mock.Anything, mock.Anything).Return(false, nil)
		u.AppService.On("BuildContext", "gg", "ml").Return(u.AppFunctionContext)

		js := &TrainingJobService{
			service:    u.AppService,
			appConfig:  appConfig,
			dbClient:   &dBMockClient,
			client:     &mockHttpClient,
			mqttSender: mockMqttSender,
		}
		err := js.DeleteMLEventConfig(algoName, modelConfigName, eventName)
		if err != nil {
			t.Errorf("DeleteMLEventConfig() error = %v, wantErr false", err)
			t.FailNow()
		}
	})
}

// generateCondition creates a config.Condition with the specified severity and thresholds.
func generateCondition(severity string, thresholds ...config.ThresholdDefinition) config.Condition {
	return config.Condition{
		SeverityLevel:         severity,
		ThresholdsDefinitions: thresholds,
	}
}

// generateThreshold creates a config.ThresholdDefinition with the specified parameters.
func generateThreshold(operator string, lower, upper float64, value interface{}, label string) config.ThresholdDefinition {
	return config.ThresholdDefinition{
		Operator:       operator,
		LowerThreshold: lower,
		UpperThreshold: upper,
		ThresholdValue: value,
		Label:          label,
	}
}

// generateMLEventConfig creates a config.MLEventConfig with the specified conditions.
func generateMLEventConfig(conditions []config.Condition, stabilizationPeriod int) config.MLEventConfig {
	return config.MLEventConfig{
		Conditions:                 conditions,
		StabilizationPeriodByCount: stabilizationPeriod,
	}
}
