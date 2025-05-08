/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/stretchr/testify/mock"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/helpers"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	redisMock "hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	helpersmocks "hedge/mocks/hedge/edge-ml-service/pkg/helpers"
	"io"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

var (
	flds5                  fields2
	mockJobProviderService helpersmocks.MockJobServiceProvider
	mockedAdeConnection    svcmocks.MockADEConnectionInterface
)

type fields2 struct {
	job                  *job.TrainingJobDetails
	jobProviderService   JobServiceProvider
	appConfig            *config.MLMgmtConfig
	service              interfaces.ApplicationService
	dbClient             redis.MLDbInterface
	mlStorage            helpers.MLStorageInterface
	monitorInterval      time.Duration
	elapsedTimeIncrement int
	monitorTimeout       int
}

func Init_2() {
	u = utils.NewApplicationServiceMock(map[string]string{"Training_Provider": "Hedge", "MetaDataServiceUrl": "http://localhost:59881", "RedisHost": "edgex-redis",
		"RedisPort": "6349", "RedisName": "redisdb", "RedisUsername": "default", "RedisPassword": "bn784738bnxpassword"})
	mockedDataSourceProvider = &svcmocks.MockDataStoreProvider{}
	mockedMlStorage = helpersmocks.MockMLStorageInterface{}
	mockHttpClient = svcmocks.MockHTTPClient{}
	mockHttpClient1 = svcmocks.MockHTTPClient{}
	dBMockClient = redisMock.MockMLDbInterface{}
	mockJobProviderService = helpersmocks.MockJobServiceProvider{}

	appConfig = &config.MLMgmtConfig{}
	appConfig.LoadCoreAppConfigurations(u.AppService)

	flds5 = fields2{
		job:                  &mockJob,
		jobProviderService:   &mockJobProviderService,
		appConfig:            appConfig,
		service:              u.AppService,
		dbClient:             &dBMockClient,
		mlStorage:            &mockedMlStorage,
		monitorInterval:      1,
		monitorTimeout:       5,
		elapsedTimeIncrement: 1,
	}

	mockmlModelConfig = &config.MLModelConfig{
		Name:        "example_training_config.csv",
		Version:     "1.0",
		Description: "Test description",
		MLAlgorithm: "TestAlgorithm",
		//PrimaryProfile:    "TestProfile",
		//FeaturesByProfile: map[string][]config.Feature{"TestProfile": {}},
		//GroupByMetaData:     []string{"TestMetaData"},
		//TrainingDataFilters: []config.FilterDefinition{},
		/*		LabelledOutputFeatures: []config.Feature{
				{
					Name: "Label1",
					Type: "string",
				},
			},*/
		//SamplingIntervalSecs:            60,
		//TrainingDurationSecs:            3600,
		//InputTopicForInferencing:        "enriched/events/device/TestProfile/#",
		Enabled:            true,
		ModelConfigVersion: 1,
		//MetaDataNumEncodingByFeatureKey: map[string]map[string]float64{},
		//FeatureNameToColumnIndex:        map[string]int{},
		LocalModelStorageDir: "local/models/TestAlgorithm/TestProfile",
		//InferenceEndPointURL:            "http://test-inference-endpoint",
		TrainedModelCount: 0,
	}
	mockJob = job.TrainingJobDetails{
		Name:              "AIF Job",
		MLAlgorithm:       "ADE",
		MLModelConfigName: "example_training_config.csv",
		MLModelConfig:     mockmlModelConfig,
		Enabled:           false,
		StatusCode:        job.New,
		Status:            "",
		StartTime:         1701619616,
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
	Cleanup()
}

func TestJobExecutor_Execute(t *testing.T) {
	Init_2()
	mockedMlStorage.On("GetTrainingDataFileName").Return("example_training_data.csv")
	mockedMlStorage.On("GetMLModelConfigFileName", mock.Anything).Return("example_training_config.csv")
	mockedMlStorage.On("FileExists", mock.Anything).Return(true)
	mockedMlStorage.On("GetBaseLocalDirectory").Return("baseDir/")
	dBMockClient.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)
	mockJobProviderService.On("UploadFile", "example_training_data.csv", "example_training_config.csv").Return(nil)
	mockJobProviderService.On("SubmitTrainingJob", mock.Anything).Return(nil)
	mockJobProviderService.On("GetTrainingJobStatus", mock.Anything).Return(nil)
	mockJobProviderService.On("DownloadModel", "baseDir/", mock.Anything).Return(nil)

	// For failure scenario-1
	mockedMlStorage1 := helpersmocks.MockMLStorageInterface{}
	mockedMlStorage1.On("FileExists", mock.Anything).Return(false)
	mockedMlStorage1.On("GetTrainingDataFileName").Return("example_training_data.csv")
	mockedMlStorage1.On("GetMLModelConfigFileName", mock.Anything).Return("example_training_config.csv")

	// For failure scenario-2
	mockJobProviderService1 := helpersmocks.MockJobServiceProvider{}
	mockJobProviderService1.On("UploadFile", mock.Anything, mock.Anything).Return(errors.New("mocked error"))

	// For failure scenario-3
	mockJobProviderService2 := helpersmocks.MockJobServiceProvider{}
	mockJobProviderService2.On("UploadFile", "example_training_data.csv", "example_training_config.csv").Return(nil)
	mockJobProviderService2.On("SubmitTrainingJob", mock.Anything).Return(errors.New("mocked error"))

	// For failure scenario-4
	mockJobProviderService3 := helpersmocks.MockJobServiceProvider{}
	mockJobProviderService3.On("UploadFile", "example_training_data.csv", "example_training_config.csv").Return(nil)
	mockJobProviderService3.On("SubmitTrainingJob", mock.Anything).Return(nil)
	mockJobProviderService3.On("GetTrainingJobStatus", mock.Anything).Return(nil)
	mockJobProviderService3.On("DownloadModel", "baseDir/", mock.Anything).Return(errors.New("mocked error"))

	// For failure scenario-5
	mockJobProviderService4 := helpersmocks.MockJobServiceProvider{}
	mockJobProviderService4.On("UploadFile", "example_training_data.csv", "example_training_config.csv").Return(nil)
	mockJobProviderService4.On("SubmitTrainingJob", mock.Anything).Return(nil)
	mockJobProviderService4.On("GetTrainingJobStatus", mock.Anything).Return(errors.New("mocker error"))

	flds8 := fields2{
		job:                  &mockJob,
		jobProviderService:   &mockJobProviderService2,
		appConfig:            appConfig,
		service:              u.AppService,
		dbClient:             &dBMockClient,
		mlStorage:            &mockedMlStorage,
		monitorInterval:      1,
		monitorTimeout:       5,
		elapsedTimeIncrement: 1,
	}
	flds9 := fields2{
		job:                  &mockJob,
		jobProviderService:   &mockJobProviderService3,
		appConfig:            appConfig,
		service:              u.AppService,
		dbClient:             &dBMockClient,
		mlStorage:            &mockedMlStorage,
		monitorInterval:      1,
		monitorTimeout:       5,
		elapsedTimeIncrement: 1,
	}
	flds10 := fields2{
		job:                  &mockJob,
		jobProviderService:   &mockJobProviderService4,
		appConfig:            appConfig,
		service:              u.AppService,
		dbClient:             &dBMockClient,
		mlStorage:            &mockedMlStorage,
		monitorInterval:      1,
		monitorTimeout:       5,
		elapsedTimeIncrement: 1,
	}

	tests := []struct {
		name    string
		fields  fields2
		want    *job.TrainingJobDetails
		wantErr bool
	}{
		{"Execute - Passed", flds5, &mockJob, false},
		{"Execute - Failed-3", flds8, &mockJob, true},  //SubmitTrainingJob failed
		{"Execute - Failed-4", flds9, &mockJob, true},  //DownloadModel failed
		{"Execute - Failed-5", flds10, &mockJob, true}, //MonitorJob failed

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MonitorInterval = tt.fields.monitorInterval
			MonitorTimeout = tt.fields.monitorTimeout
			ElapsedTimeIncrement = tt.fields.elapsedTimeIncrement
			jobExecutor := &JobExecutor{
				job:                tt.fields.job,
				jobProviderService: tt.fields.jobProviderService,
				appConfig:          tt.fields.appConfig,
				service:            tt.fields.service,
				dbClient:           tt.fields.dbClient,
				mlStorage:          tt.fields.mlStorage,
			}
			got, err := jobExecutor.Execute()
			MonitorInterval = 0
			MonitorTimeout = 0
			ElapsedTimeIncrement = 0
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Execute() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobExecutor_buildConfigurationFile(t *testing.T) {
	Init_2()

	tests := []struct {
		name    string
		fields  fields2
		want    string
		wantErr bool
	}{
		{"BuildConfigurationFile - Passed", flds5, "example_training_config.csv_config.json", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobExecutor := &JobExecutor{
				job:                tt.fields.job,
				jobProviderService: tt.fields.jobProviderService,
				appConfig:          tt.fields.appConfig,
				service:            tt.fields.service,
				dbClient:           tt.fields.dbClient,
				mlStorage:          tt.fields.mlStorage,
			}
			got, err := jobExecutor.buildConfigurationFile()
			if (err != nil) != tt.wantErr {
				t.Errorf("buildConfigurationFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("buildConfigurationFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJobExecutor_buildGCPTrainingConfigFileName(t *testing.T) {
	Init_2()
	jobExecutor := &JobExecutor{
		job: &mockJob,
	}
	result := jobExecutor.buildGCPTrainingConfigFileName()
	expectedResult := "TestAlgorithm/example_training_config.csv/assets/config.json"

	if result != expectedResult {
		t.Errorf("Expected %s, but got %s", expectedResult, result)
	}
}

func TestJobExecutor_monitorTrainingJob(t *testing.T) {
	Init_2()
	mockJobProviderService.On("GetTrainingJobStatus", mock.Anything).Return(nil)

	flds5 = fields2{
		service:   u.AppService,
		appConfig: appConfig,
		job: &job.TrainingJobDetails{
			StatusCode: job.TrainingCompleted,
		},
		jobProviderService:   &mockJobProviderService,
		monitorInterval:      1,
		monitorTimeout:       5,
		elapsedTimeIncrement: 1,
	}
	flds6 := fields2{
		service:   u.AppService,
		appConfig: appConfig,
		job: &job.TrainingJobDetails{
			StatusCode: job.Failed,
		},
		jobProviderService:   &mockJobProviderService,
		monitorInterval:      1,
		monitorTimeout:       5,
		elapsedTimeIncrement: 1,
	}
	tests := []struct {
		name    string
		fields  fields2
		wantErr bool
	}{
		{"MonitorTrainingJob - Passed", flds5, false},
		{"MonitorTrainingJob - Failed", flds6, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MonitorInterval = tt.fields.monitorInterval
			MonitorTimeout = tt.fields.monitorTimeout
			ElapsedTimeIncrement = tt.fields.elapsedTimeIncrement
			jobExecutor := &JobExecutor{
				job:                tt.fields.job,
				jobProviderService: tt.fields.jobProviderService,
				appConfig:          tt.fields.appConfig,
				service:            tt.fields.service,
				dbClient:           tt.fields.dbClient,
				mlStorage:          tt.fields.mlStorage,
			}
			if err := jobExecutor.monitorTrainingJob(); (err != nil) != tt.wantErr {
				t.Errorf("monitorTrainingJob() error = %v, wantErr %v", err, tt.wantErr)
			}
			MonitorInterval = 0
			MonitorTimeout = 0
			ElapsedTimeIncrement = 0
		})
	}
}

func TestJobExecutor_submitTrainingJob(t *testing.T) {
	Init_2()
	mockJobProviderService.On("SubmitTrainingJob", mock.Anything).Return(nil)
	mockJobProviderService1 := helpersmocks.MockJobServiceProvider{}
	mockJobProviderService1.On("SubmitTrainingJob", mock.Anything).Return(errors.New("mocked error"))
	flds6 := fields2{
		job:                &mockJob,
		jobProviderService: &mockJobProviderService1,
		service:            u.AppService,
		dbClient:           &dBMockClient,
		mlStorage:          &mockedMlStorage,
	}

	tests := []struct {
		name    string
		fields  fields2
		wantErr bool
	}{
		{"SubmitTrainingJob - Passed", flds5, false},
		{"SubmitTrainingJob - Failed", flds6, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobExecutor := &JobExecutor{
				job:                tt.fields.job,
				jobProviderService: tt.fields.jobProviderService,
				appConfig:          tt.fields.appConfig,
				service:            tt.fields.service,
				dbClient:           tt.fields.dbClient,
				mlStorage:          tt.fields.mlStorage,
			}
			if err := jobExecutor.submitTrainingJob(); (err != nil) != tt.wantErr {
				t.Errorf("submitTrainingJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewJobExecutor(t *testing.T) {
	Init_2()

	// Test case 1: GCP as training provider
	appConfig.TrainingProvider = "GCP"
	jobExecutor := &JobExecutor{}
	jobExecutor.mlStorage = &mockedMlStorage
	jobExecutor = NewJobExecutor(u.AppService, appConfig, &mockJob, &dBMockClient)
	if jobExecutor == nil {
		t.Error("Expected non-nil jobExecutor for GCP, got nil")
	}

	// Test case 2: ADE as training provider
	appConfig.TrainingProvider = "ADE"
	algoInstance := map[string]interface{}{
		"response": map[string]interface{}{
			"algorithms": []interface{}{
				map[string]interface{}{
					"templateId": "111",
					"name":       "ADE_example_training_config.csv",
					"id":         "222",
				},
				map[string]interface{}{
					"templateId": "123",
					"name":       "ADE_example_training_config.csv",
					"id":         "777",
				},
			},
		},
	}
	mockResponseBody, _ := json.Marshal(algoInstance)
	response := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(mockResponseBody)),
		Header:     make(http.Header),
	}
	mockedAdeConnection.On("ExecuteADEApi", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(response, nil)
	jobExecutor = NewJobExecutor(u.AppService, appConfig, &mockJob, &dBMockClient)
	if jobExecutor == nil {
		t.Error("Expected nil jobExecutor for ADE, got non-nil")
	}

	// Test case 3: Unsupported training provider
	appConfig.TrainingProvider = "Unknown"
	jobExecutor = NewJobExecutor(u.AppService, appConfig, &mockJob, &dBMockClient)
	if jobExecutor.jobProviderService != nil {
		t.Error("Expected nil jobExecutor for unknown provider, got non-nil")
	}
}
func Cleanup() {
	os.Remove("example_training_config.csv_config.json")
}
