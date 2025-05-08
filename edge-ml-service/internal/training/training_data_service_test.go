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
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	hedgeErrors "hedge/common/errors"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/stretchr/testify/mock"
	"hedge/common/service"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/data"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/helpers"
	trainingdata "hedge/edge-ml-service/pkg/training-data"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	redisMock "hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	helpersmocks "hedge/mocks/hedge/edge-ml-service/pkg/helpers"
	utilsmocks "hedge/mocks/hedge/edge-ml-service/pkg/training-data"
)

const baseTmpFolder = "tmp"

var (
	u                               *utils.HedgeMockUtils
	appConfig                       *config.MLMgmtConfig
	mockedDataSourceProvider        *svcmocks.MockDataStoreProvider
	modelConfig                     *config.MLModelConfig
	mockedPreProcessor              helpersmocks.MockPreProcessorInterface
	mockedMlStorage                 helpersmocks.MockMLStorageInterface
	mockHttpClient, mockHttpClient1 svcmocks.MockHTTPClient
	mockResponse                    *http.Response
	mockDataCollector               utilsmocks.MockDataCollectorInterface
	dBMockClient2                   redisMock.MockMLDbInterface
	mockedFetchStatusManager        utilsmocks.MockFetchStatusManagerInterface
	testJob                         job.TrainingJobDetails
)

func Init_1() {
	u = utils.NewApplicationServiceMock(
		map[string]string{
			"DataStore_Provider": "ADE",
			"MetaDataServiceUrl": "http://localhost:59881",
			"RedisHost":          "edgex-redis",
			"RedisPort":          "6349",
			"RedisName":          "redisdb",
			"RedisUsername":      "default",
			"RedisPassword":      "bn784738bnxpassword",
			"ADE_TENANT_URL":     "http://dummy-url.com",
			"ADE_TENANT_ID":      "dummy-id",
		},
	)
	mockedDataSourceProvider = &svcmocks.MockDataStoreProvider{}
	mockHttpClient = svcmocks.MockHTTPClient{}
	mockHttpClient1 = svcmocks.MockHTTPClient{}
	modelConfig = &config.MLModelConfig{
		Name:                 "Dummy_file_name",
		Version:              "1.0",
		Description:          "Test description",
		MLAlgorithm:          "TestAlgorithm",
		Enabled:              true,
		ModelConfigVersion:   1,
		LocalModelStorageDir: "local/models/TestAlgorithm/TestProfile",
		TrainedModelCount:    0,
	}

	mockedPreProcessor = helpersmocks.MockPreProcessorInterface{}
	mockedMlStorage = helpersmocks.MockMLStorageInterface{}
	mockDataCollector = utilsmocks.MockDataCollectorInterface{}
	mockedFetchStatusManager = utilsmocks.MockFetchStatusManagerInterface{}

	mockTimeSeriesResponse := data.TimeSeriesResponse{
		Status: "success",
		Data: data.TimeSeriesData{
			ResultType: "matrix",
			Result: []data.MetricResult{
				{
					Metric: map[string]interface{}{
						"__name__":    "TestMetric",
						"deviceName":  "TestDevice",
						"profileName": "TestProfile",
					},
					Values: []interface{}{
						[]interface{}{float64(1604289920), "123.45"},
					},
				},
			},
		},
	}

	mockBody, _ := json.Marshal(mockTimeSeriesResponse)
	mockResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(mockBody)),
		Header:     make(http.Header),
	}

	testJob = job.TrainingJobDetails{
		Name:              "AIF Job",
		MLAlgorithm:       "ADE",
		MLModelConfigName: "Dummy_file_name",
		MLModelConfig:     modelConfig,
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
	appConfig = &config.MLMgmtConfig{}
	appConfig.LoadCoreAppConfigurations(u.AppService)
	appConfig.BaseTrainingDataLocalDir = baseTmpFolder
	dBMockClient = redisMock.MockMLDbInterface{}
	dBMockClient1 = redisMock.MockMLDbInterface{}
	dBMockClient2 = redisMock.MockMLDbInterface{}
}

/*
fileTypes:
0 - status IN_PROGRESS
1 - status COMPLETED
2 - invalid json
*/
func createStatusFile(path string, fileType int) {
	switch fileType {
	case 0:
		trainingdata.UpdateStatus(
			path,
			&trainingdata.StatusResponse{Status: trainingdata.StatusInProgress},
		)
	case 1:
		trainingdata.UpdateStatus(
			path,
			&trainingdata.StatusResponse{Status: trainingdata.StatusCompleted},
		)
	case 2:
		helpers.SaveFile(path, []byte("Invalid json"))
	}
}

func CleanupFolder(folderPath string, t *testing.T) {
	_, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		return
	}
	// Attempt to delete the folder and its content
	if err := os.RemoveAll(folderPath); err != nil {
		t.Errorf("Failed to delete file: %v", err)
	}
}

func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		// If the error is not because the file doesn't exist, log it
		if !os.IsNotExist(err) {
			fmt.Printf("Error checking file: %v\n", err)
			return false
		}
		return false
	}

	// Check if it is indeed a file and not a directory
	return !info.IsDir()
}

func TestNewTrainingDataService(t *testing.T) {
	Init_1()

	mockCmdRunner := svcmocks.MockCommandRunnerInterface{}
	mockCmdRunner.On("Run", mock.Anything).Return(nil)
	result := NewTrainingDataService(
		u.AppService,
		appConfig,
		&dBMockClient,
		mockedDataSourceProvider,
		&mockCmdRunner,
		nil,
	)
	if result == nil {
		t.Error("NewTrainingDataService() returned nil")
		return
	}
	if result.GetService() != u.AppService {
		t.Errorf("Service is %v, want %v", result.GetService(), u.AppService)
	}
	if result.GetAppConfig() != appConfig {
		t.Errorf("AppConfig is %v, want %v", result.GetAppConfig(), appConfig)
	}
	if result.GetDbClient() != &dBMockClient {
		t.Errorf("DbClient is %v, want %v", result.GetDbClient(), &dBMockClient)
	}
	if result.GetDataStoreProvider() != mockedDataSourceProvider {
		t.Errorf(
			"DataStoreProvider is %v, want %v",
			result.GetDataStoreProvider(),
			mockedDataSourceProvider,
		)
	}
}

func TestTrainingDataService_CleanupStatuses(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{}
	appConfig1.BaseTrainingDataLocalDir = baseTmpFolder
	modelConfig := config.MLModelConfig{
		Name:        "TestModelConfig",
		MLAlgorithm: "TestAlgorithm",
	}
	path := filepath.Join(
		appConfig1.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"data_export",
		"status.json",
	)
	type fields struct {
		service   interfaces.ApplicationService
		appConfig *config.MLMgmtConfig
	}
	type args struct {
		mlModelConfigs []config.MLModelConfig
		fileType       int
	}
	arg := args{
		mlModelConfigs: []config.MLModelConfig{modelConfig},
		fileType:       0,
	}
	arg1 := args{
		mlModelConfigs: []config.MLModelConfig{modelConfig},
		fileType:       1,
	}
	arg2 := args{
		mlModelConfigs: []config.MLModelConfig{modelConfig},
		fileType:       2,
	}
	flds := fields{
		service:   u.AppService,
		appConfig: appConfig1,
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantStatus string
	}{
		{"CleanupStatuses - IN_PROGRESS", flds, arg, trainingdata.StatusFailed},
		{"CleanupStatuses - COMPLETED", flds, arg1, trainingdata.StatusCompleted},
		{"CleanupStatuses - Invalid json", flds, arg2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trgSvc := &TrainingDataService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			createStatusFile(path, tt.args.fileType)
			t.Cleanup(func() {
				os.RemoveAll(baseTmpFolder)
			})
			trgSvc.CleanupStatuses(tt.args.mlModelConfigs)
			if tt.wantStatus != "" {
				status, _ := trainingdata.ReadStatus(path)
				if status.Status != tt.wantStatus {
					t.Errorf(
						"CleanupStatuses() status = %v, wantStatus %v",
						status.Status,
						tt.wantStatus,
					)
				}
			} else {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("CleanupStatuses() Invalid file not removed")
				}
			}
		})
	}
}

/*
func TestTrainingDataService_CreateTrainingData_Success(t *testing.T) {
	Init_1()

	modelConfig := &config.MLModelConfig{
		Name:                 "Dummy_file_name",
		Version:              "1.0",
		Description:          "Test description",
		MLAlgorithm:          "TestAlgorithm",
		Enabled:              true,
		ModelConfigVersion:   1,
		LocalModelStorageDir: "local/models/TestAlgorithm/TestProfile",
		TrainedModelCount:    0,
	}

	appConfig.TrainingProvider = "AIF"

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}
	jobSubmissionDetails := &job.JobSubmissionDetails{
		Name:              "AIF Job",
		Description:       "This is a sample job submission",
		MLAlgorithm:       "TestAlgorithm",
		MLModelConfigName: "Dummy_file_name",
	}

	dBMockClient.On("GetLatestModelVersion", modelConfig.MLAlgorithm, modelConfig.Name).
		Return(int64(1), nil)
	dBMockClient.On("GetMLTrainingJob", testJob.Name).Return(testJob, nil)
	dBMockClient.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)
	dBMockClient.On("MarkOldTrainingDataDeprecated", "Dummy_file_name", mock.Anything).Return(nil)

	mockDataCollector.On("ExecuteDeleteMe").Return(nil)
	mockDataCollector.On("GetMlStorage").Return(&mockedMlStorage)

	mockedMlStorage.On("GetTrainingDataFileName").Return("Dummy_file_name")

	filename, err := trgSvc.CreateTrainingData(modelConfig, jobSubmissionDetails)

	if err != nil {
		t.Errorf("ValidateUploadedData returned error, but expected success: %v", err)
	}
	assert.Equal(t, filename, "Dummy_file_name")
}
*/
/*
	func TestTrainingDataService_DataCollectorExecuteError_Failure(t *testing.T) {
		Init_1()

		modelConfig := &config.MLModelConfig{
			Name:                 "Dummy_file_name",
			Version:              "1.0",
			Description:          "Test description",
			MLAlgorithm:          "TestAlgorithm",
			Enabled:              true,
			ModelConfigVersion:   1,
			LocalModelStorageDir: "local/models/TestAlgorithm/TestProfile",
			TrainedModelCount:    0,
		}

		trgSvc := &TrainingDataService{
			service:           u.AppService,
			appConfig:         appConfig,
			dbClient:          &dBMockClient,
			dataStoreProvider: mockedDataSourceProvider,
			dataCollector:     &mockDataCollector,
		}
		jobSubmissionDetails := &job.JobSubmissionDetails{
			Name:              "AIF Job",
			Description:       "This is a sample job submission",
			MLAlgorithm:       "TestAlgorithm",
			MLModelConfigName: "Dummy_file_name",
		}

		dBMockClient.On("GetLatestModelVersion", modelConfig.MLAlgorithm, modelConfig.Name).
			Return(int64(1), nil)
		dBMockClient.On("GetMLTrainingJob", testJob.Name).Return(testJob, nil)
		dBMockClient.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)
		dBMockClient.On("MarkOldTrainingDataDeprecated", "Dummy_file_name", mock.Anything).Return(nil)

		mockDataCollector.On("ExecuteDeleteMe").Return(errors.New("mocked error"))
		mockDataCollector.On("GetMlStorage").Return(&mockedMlStorage)

		mockedMlStorage.On("GetTrainingDataFileName").Return("Dummy_file_name")

		filename, err := trgSvc.CreateTrainingData(modelConfig, jobSubmissionDetails)

		if err == nil {
			t.Errorf("ValidateUploadedData returned success, but expected error")
		}
		assert.Equal(
			t,
			err.Error(),
			"Failed to create training data for job AIF Job: Error while collecting training data: mocked error",
		)
		assert.Equal(t, filename, "")
	}
*/
func TestTrainingDataService_AddMLTrainingJobError_Failure(t *testing.T) {
	Init_1()

	modelConfig := &config.MLModelConfig{
		Name:                 "Dummy_file_name",
		Version:              "1.0",
		Description:          "Test description",
		MLAlgorithm:          "TestAlgorithm",
		Enabled:              true,
		ModelConfigVersion:   1,
		LocalModelStorageDir: "local/models/TestAlgorithm/TestProfile",
		TrainedModelCount:    0,
	}

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}
	jobSubmissionDetails := &job.JobSubmissionDetails{
		Name:              "AIF Job",
		Description:       "This is a sample job submission",
		MLAlgorithm:       "TestAlgorithm",
		MLModelConfigName: "Dummy_file_name",
	}

	dBMockClient.On("GetLatestModelVersion", modelConfig.MLAlgorithm, modelConfig.Name).
		Return(int64(0), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("GetMLTrainingJob", testJob.Name).
		Return(testJob, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("UpdateMLTrainingJob", mock.Anything).Return("", nil)
	dBMockClient.On("AddMLTrainingJob", mock.Anything).
		Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "AddMLTrainingJob mocked error"))

	filename, err := trgSvc.CreateTrainingDataLocal(modelConfig, jobSubmissionDetails)

	if err == nil {
		t.Errorf("ValidateUploadedData returned success, but expected error")
	}
	assert.Equal(t, err.Error(), "AddMLTrainingJob mocked error")
	assert.Equal(t, filename, "")
}

func TestTrainingDataService_CreateTrainingDataExport_Success(t *testing.T) {
	Init_1()

	mlModelConfig := &config.MLModelConfig{
		Name:               "Dummy_file_name",
		Version:            "1.0",
		Description:        "Test description",
		MLAlgorithm:        "TestAlgorithm",
		Enabled:            true,
		ModelConfigVersion: 1,
	}

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		exportCtx:         make(map[string]*ExportDataContext),
	}

	mockDataCollector.On("GetMlStorage").Return(&mockedMlStorage)
	mockDataCollector.On("GetPreProcessor").Return(&mockedPreProcessor)
	mockDataCollector.On("GetFetchStatusManager").Return(&mockedFetchStatusManager)
	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GenerateConfig", mock.Anything).Return(nil)
	mockDataCollector.On("ExecuteWithFileNames", mock.Anything, mock.Anything).Return(nil)

	mockedPreProcessor.On("GetMLModelConfig").Return(mlModelConfig)
	mockedPreProcessor.On("GetMLAlgorithmDefinition").Return(&config.MLAlgorithmDefinition{})

	mockedFetchStatusManager.On("UpdateStatus", mock.Anything).Return()
	mockedFetchStatusManager.On("UpdateStatus", mock.Anything, mock.Anything).Return()
	mockedFetchStatusManager.On("Close").Return()

	dataBaseDir := filepath.Join(
		baseTmpFolder,
		mlModelConfig.MLAlgorithm,
		"TestModelConfig",
		"data",
	)
	dataExportDir := filepath.Join(
		baseTmpFolder,
		mlModelConfig.MLAlgorithm,
		"TestModelConfig",
		"data_export",
	)

	mockedMlStorage.On("GetLocalTrainingDataBaseDir").Return(dataBaseDir)
	mockedMlStorage.On("GetBaseLocalDirectory").Return(baseTmpFolder)
	mockedMlStorage.On("CompressFiles", mock.Anything, mock.Anything).Return(nil)

	statusFilePath := filepath.Join(dataExportDir, "status.json")
	trainingdata.NewFetchStatusManager(statusFilePath)

	trgSvc.CreateTrainingDataExport(mlModelConfig, false)

	expectedConfigFilePath := filepath.Join(dataExportDir, "config.json")
	expectedDataFilePath := filepath.Join(dataExportDir, "Dummy_file_name.csv")
	pathInZip := strings.TrimPrefix(dataBaseDir, baseTmpFolder+string(filepath.Separator))

	expectedFilesToZip := []helpers.ZipFileInfo{
		{
			FilePath:      expectedDataFilePath,
			FilePathInZip: filepath.Join(pathInZip, "Dummy_file_name.csv"),
		},
		{FilePath: expectedConfigFilePath, FilePathInZip: filepath.Join(pathInZip, "config.json")},
	}
	expectedTrainingInputZip := filepath.Join(dataExportDir, "training_input.zip")

	mockDataCollector.AssertCalled(t, "GenerateConfig", expectedConfigFilePath)
	mockDataCollector.AssertCalled(t, "ExecuteWithFileNames", expectedDataFilePath)
	/*	mockedFetchStatusManager.AssertCalled(t, "UpdateStatus", "IN_PROGRESS")
		mockedFetchStatusManager.AssertCalled(t, "UpdateStatus", "COMPLETED")
		mockedFetchStatusManager.AssertNumberOfCalls(t, "UpdateStatus", 2)
		mockedFetchStatusManager.AssertCalled(t, "Close")*/
	mockedMlStorage.AssertCalled(t, "CompressFiles", expectedFilesToZip, expectedTrainingInputZip)
	mockedMlStorage.AssertNumberOfCalls(t, "CompressFiles", 1)
}

/*
func TestTrainingDataService_CreateTrainingDataExport_ExecuteWithFileNamesFails(t *testing.T) {
	Init_1()

	mlModelConfig := &config.MLModelConfig{
		Name:               "Dummy_file_name",
		Version:            "1.0",
		Description:        "Test description",
		MLAlgorithm:        "TestAlgorithm",
		Enabled:            true,
		ModelConfigVersion: 1,
	}

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		exportCtx:         make(map[string]*ExportDataContext),
	}

	mockDataCollector.On("GetMlStorage").Return(&mockedMlStorage)
	mockDataCollector.On("GetPreProcessor").Return(&mockedPreProcessor)
	mockDataCollector.On("GetFetchStatusManager").Return(&mockedFetchStatusManager)
	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GenerateConfig", mock.Anything).Return(nil)
	mockDataCollector.On("ExecuteWithFileNames", mock.Anything, mock.Anything).
		Return(errors.New("ExecuteWithFileNames Mock err"))

	mockedPreProcessor.On("GetMLModelConfig").Return(mlModelConfig)
	mockedPreProcessor.On("GetMLAlgorithmDefinition").Return(&config.MLAlgorithmDefinition{})

	mockedFetchStatusManager.On("UpdateStatus", mock.Anything).Return()
	mockedFetchStatusManager.On("UpdateStatus", mock.Anything, mock.Anything).Return()
	mockedFetchStatusManager.On("Close").Return()

	dataBaseDir := filepath.Join(
		baseTmpFolder,
		mlModelConfig.MLAlgorithm,
		"TestModelConfig",
		"data",
	)
	dataExportDir := filepath.Join(
		baseTmpFolder,
		mlModelConfig.MLAlgorithm,
		"TestModelConfig",
		"data_export",
	)

	mockedMlStorage.On("GetLocalTrainingDataBaseDir").Return(dataBaseDir)

	trgSvc.CreateTrainingDataExport(mlModelConfig, false)

	expectedConfigFilePath := filepath.Join(dataExportDir, "config.json")
	//expectedDataFilePath := filepath.Join(dataExportDir, "Dummy_file_name.csv")

	mockDataCollector.AssertCalled(t, "GenerateConfig", expectedConfigFilePath)
	//mockDataCollector.AssertCalled(t, "ExecuteWithFileNames", expectedDataFilePath)
	mockedFetchStatusManager.AssertCalled(t, "UpdateStatus", "IN_PROGRESS")
	mockedFetchStatusManager.AssertCalled(
		t,
		"UpdateStatus",
		"FAILED",
		"ExecuteWithFileNames Mock err",
	)
	mockedFetchStatusManager.AssertNumberOfCalls(t, "UpdateStatus", 2)
	mockedFetchStatusManager.AssertCalled(t, "Close")
}
*/
/*
func TestTrainingDataService_CreateTrainingDataExport_GenerateConfigFails(t *testing.T) {
	Init_1()

	mlModelConfig := &config.MLModelConfig{
		Name:               "Dummy_file_name",
		Version:            "1.0",
		Description:        "Test description",
		MLAlgorithm:        "TestAlgorithm",
		Enabled:            true,
		ModelConfigVersion: 1,
	}

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		exportCtx:         make(map[string]*ExportDataContext),
	}

	mockDataCollector.On("GetMlStorage").Return(&mockedMlStorage)
	mockDataCollector.On("GetPreProcessor").Return(&mockedPreProcessor)
	mockDataCollector.On("GetFetchStatusManager").Return(&mockedFetchStatusManager)
	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GenerateConfig", mock.Anything).
		Return(errors.New("GenerateConfig Mock err"))

	mockedPreProcessor.On("GetMLModelConfig").Return(mlModelConfig)
	mockedPreProcessor.On("GetMLAlgorithmDefinition").Return(&config.MLAlgorithmDefinition{})

	mockedFetchStatusManager.On("UpdateStatus", mock.Anything).Return()
	mockedFetchStatusManager.On("UpdateStatus", mock.Anything, mock.Anything).Return()
	mockedFetchStatusManager.On("Close").Return()

	dataBaseDir := filepath.Join(
		baseTmpFolder,
		mlModelConfig.MLAlgorithm,
		"TestModelConfig",
		"data",
	)
	mockedMlStorage.On("GetLocalTrainingDataBaseDir").Return(dataBaseDir)

	dataExportDir := filepath.Join(
		baseTmpFolder,
		mlModelConfig.MLAlgorithm,
		"TestModelConfig",
		"data_export",
	)

	trgSvc.CreateTrainingDataExport(mlModelConfig, false)

	expectedConfigFilePath := filepath.Join(dataExportDir, "config.json")

	mockDataCollector.AssertCalled(t, "GenerateConfig", expectedConfigFilePath)
	mockedFetchStatusManager.AssertCalled(t, "UpdateStatus", "IN_PROGRESS")
	mockedFetchStatusManager.AssertCalled(t, "UpdateStatus", "FAILED", "GenerateConfig Mock err")
	mockedFetchStatusManager.AssertNumberOfCalls(t, "UpdateStatus", 2)
	mockedFetchStatusManager.AssertCalled(t, "Close")
}
*/
/*
func TestTrainingDataService_CreateTrainingDataExport_CompressFilesFails_Failure(t *testing.T) {
	Init_1()

	mlModelConfig := &config.MLModelConfig{
		Name:               "Dummy_file_name",
		Version:            "1.0",
		Description:        "Test description",
		MLAlgorithm:        "TestAlgorithm",
		Enabled:            true,
		ModelConfigVersion: 1,
	}

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		exportCtx:         make(map[string]*ExportDataContext),
	}

	mockDataCollector.On("GetMlStorage").Return(&mockedMlStorage)
	mockDataCollector.On("GetPreProcessor").Return(&mockedPreProcessor)
	mockDataCollector.On("GetFetchStatusManager").Return(&mockedFetchStatusManager)
	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GenerateConfig", mock.Anything).Return(nil)
	mockDataCollector.On("ExecuteWithFileNames", mock.Anything, mock.Anything).Return(nil)

	mockedPreProcessor.On("GetMLModelConfig").Return(mlModelConfig)
	mockedPreProcessor.On("GetMLAlgorithmDefinition").Return(&config.MLAlgorithmDefinition{})

	mockedFetchStatusManager.On("UpdateStatus", mock.Anything).Return()
	mockedFetchStatusManager.On("UpdateStatus", mock.Anything, mock.Anything).Return()
	mockedFetchStatusManager.On("Close").Return()

	dataBaseDir := filepath.Join(
		baseTmpFolder,
		mlModelConfig.MLAlgorithm,
		"TestModelConfig",
		"data",
	)
	dataExportDir := filepath.Join(
		baseTmpFolder,
		mlModelConfig.MLAlgorithm,
		"TestModelConfig",
		"data_export",
	)

	mockedMlStorage.On("GetLocalTrainingDataBaseDir").Return(dataBaseDir)
	mockedMlStorage.On("CompressFiles", mock.Anything, mock.Anything).
		Return(errors.New("CompressFiles mock error"))
	mockedMlStorage.On("GetBaseLocalDirectory").Return(baseTmpFolder)

	trgSvc.CreateTrainingDataExport(mlModelConfig, false)

	expectedConfigFilePath := filepath.Join(dataExportDir, "config.json")
	//expectedDataFilePath := filepath.Join(dataExportDir, "Dummy_file_name.csv")

	mockDataCollector.AssertCalled(t, "GenerateConfig", expectedConfigFilePath)
	//mockDataCollector.AssertCalled(t, "ExecuteWithFileNames", expectedDataFilePath)
	mockedFetchStatusManager.AssertCalled(t, "UpdateStatus", "IN_PROGRESS")
	mockedFetchStatusManager.AssertCalled(t, "UpdateStatus", "FAILED", "CompressFiles mock error")
	mockedFetchStatusManager.AssertNumberOfCalls(t, "UpdateStatus", 2)
	mockedFetchStatusManager.AssertCalled(t, "Close")
}
*/
func TestTrainingDataService_ValidateJobExists(t *testing.T) {
	Init_1()
	dBMockClient.On("GetMLTrainingJob", testJob.Name).Return(testJob, nil)
	dBMockClient.On("GetMLTrainingJob", "NonExistentJob").
		Return(job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient1.On("GetMLTrainingJob", testJob.Name).
		Return(testJob, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	type fields struct {
		service           interfaces.ApplicationService
		appConfig         *config.MLMgmtConfig
		dbClient          redis.MLDbInterface
		dataStoreProvider service.DataStoreProvider
		dataCollector     trainingdata.DataCollectorInterface
	}
	type args struct {
		jobName string
	}
	flds := fields{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}
	flds1 := fields{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient1,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"CheckIfJobExists - Passed", flds, args{jobName: testJob.Name}, false},
		{"CheckIfJobExists - Failed", flds, args{jobName: "NonExistentJob"}, true},
		{"CheckIfJobExists - Failed1", flds1, args{jobName: testJob.Name}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trgSvc := &TrainingDataService{
				service:           tt.fields.service,
				appConfig:         tt.fields.appConfig,
				dbClient:          tt.fields.dbClient,
				dataStoreProvider: tt.fields.dataStoreProvider,
				dataCollector:     tt.fields.dataCollector,
			}
			if err := trgSvc.CheckIfJobExists(tt.args.jobName); (err != nil) != tt.wantErr {
				t.Errorf("CheckIfJobExists() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

const uploadedConfigJson = `
{
 "mlModelConfig": {
    "name": "TestModelConfig",
    "mlAlgorithm": "TestAlgorithm",
    "version": "v3",
    "description": "Training config for WindTurbine",
    "mlDataSourceConfig": {
        "entityType": "Profile",
        "entityName": "WindTurbine",
        "featuresByProfile": {
            "WindTurbine": [
                {
                    "type": "METRIC",
                    "name": "TurbinePower",
                    "isInput": true,
                    "fromExternalSource": false
                },
                {
                    "type": "METRIC",
                    "name": "RotorSpeed",
                    "isInput": true,
                    "fromExternalSource": false
                },
                {
                    "type": "METRIC",
                    "name": "WindSpeed",
                    "isInput": true,
                    "fromExternalSource": true
                }
            ],
            "Dummy": [
                {
                    "type": "METRIC",
                    "name": "TurbinePower",
                    "isInput": true,
                    "fromExternalSource": false
                }
            ]
        },
        "groupOrJoinKeys": [
            "deviceName"
        ],
        "trainingDataSourceConfig": {
            "filters": [],
            "samplingIntervalSecs": 30,
            "dataCollectionTotalDurationSec": 18000
        },
        "predictionDataSourceConfig": {
            "streamType": "redis",
            "filters": [],
            "samplingIntervalSecs": 30,
            "topicName": "enriched/#",
            "predictionEndPointURL": "http://ml-inferencing:480999",
            "timeseriesFeaturesSetsCount": 0
        },
        "featureNameToColumnIndex": {
            "WindTurbine#RotorSpeed": 1,
            "WindTurbine#TurbinePower": 0,
            "WindTurbine#WindSpeed": 2
        }
    },
    "externalFeatures": [],
    "enabled": true,
    "message": " on device {{.deviceName}} detected in Windfarm that might be causing low power generation"
},
 "mlAlgoDefinition": {
  "name": "AutoEncoder",
  "description": "my_desc",
  "type": "Anomaly",
  "enabled": true,
  "outputFeaturesPredefined": false,
  "trainingComputeProvider": "",
  "allowDeploymentAtEdge": false,
  "requiresDataPipeline": false,
  "isPredictionRealTime": false,
  "publishPredictionResults": false,
  "predictionPayloadTemplate": {
   "template": ""
  },
  "predictionOutputKeys": [],
  "autoEventGenerationRequired": false,
  "publishPredictionsRequired": false,
  "hyperParameters": null
 }
}`

func TestTrainingDataService_ValidateUploadedData_ValidationReturnedMessage_Success(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{}
	appConfig1.BaseTrainingDataLocalDir = baseTmpFolder

	var compositeConfig config.MLConfig
	_ = json.Unmarshal([]byte(uploadedConfigJson), &compositeConfig)

	features := compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile["WindTurbine"]
	features[0].Name = "DummyName"
	features[1].Type = "DummyType"
	compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile["WindTurbine"] = features
	delete(compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile, "Dummy")
	compositeConfig.MLModelConfig.MLDataSourceConfig.FeaturesByProfile["Dummy1"] = features

	configJsonPath := filepath.Join(
		baseTmpFolder,
		compositeConfig.MLModelConfig.MLAlgorithm,
		compositeConfig.MLModelConfig.Name,
		"validation",
		"config.json",
	)

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	_ = helpers.SaveFile(configJsonPath, []byte(uploadedConfigJson))

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}

	message, hedgeErr := trgSvc.ValidateUploadedData(&compositeConfig.MLModelConfig, nil)

	if hedgeErr != nil {
		t.Errorf("ValidateUploadedData returned error, but expected success: %v", hedgeErr)
	}

	assert.Contains(
		t,
		message,
		"The uploaded model configuration doesn't match the stored one. Please fix it and try again.",
	)
}

func TestTrainingDataService_ValidateUploadedData_InvalidJson_Failure(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{}
	appConfig1.BaseTrainingDataLocalDir = baseTmpFolder

	compositeConfig := config.MLConfig{}
	_ = json.Unmarshal([]byte(uploadedConfigJson), &compositeConfig)

	configJsonPath := filepath.Join(
		baseTmpFolder,
		compositeConfig.MLModelConfig.MLAlgorithm,
		compositeConfig.MLModelConfig.Name,
		"validation",
		"config.json",
	)

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	_ = helpers.SaveFile(configJsonPath, []byte(""))

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}

	_, hedgeErr := trgSvc.ValidateUploadedData(&compositeConfig.MLModelConfig, nil)

	if hedgeErr == nil {
		t.Errorf("ValidateUploadedData returned error, but expected success: %v", hedgeErr)
	}

	expectedError := hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		"Failed to unmarshal config.json",
	)
	if !reflect.DeepEqual(hedgeErr, expectedError) {
		t.Errorf(
			"Error recieved and expected error don't match. Recieved: %v, Expected: %v",
			hedgeErr,
			expectedError,
		)
	}
}

func TestTrainingDataService_ValidateUploadedData_ConfigFileNotFound_Failure(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{}
	appConfig1.BaseTrainingDataLocalDir = baseTmpFolder

	compositeConfig := config.MLConfig{}
	_ = json.Unmarshal([]byte(uploadedConfigJson), &compositeConfig)

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}

	_, hedgeErr := trgSvc.ValidateUploadedData(&compositeConfig.MLModelConfig, nil)

	if hedgeErr == nil {
		t.Errorf("ValidateUploadedData returned error, but expected success: %v", hedgeErr)
	}

	expectedError := hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		"Failed to read extracted config.json",
	)
	if !reflect.DeepEqual(hedgeErr, expectedError) {
		t.Errorf(
			"Error recieved and expected error don't match. Recieved: %v, Expected: %v",
			hedgeErr,
			expectedError,
		)
	}
}

func createTrainingDataZipFile(path string, includeConfigJson bool, includeCsv bool) *os.File {
	if _, err := os.Stat(path); err == nil {
		// File exists, so remove it
		os.Remove(path)
	}
	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		return nil
	}
	zipFile, err := os.Create(path)
	if err != nil {
		return nil
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()
	if includeConfigJson {
		w, err := zipWriter.Create("config.json")
		if err != nil {
			return nil
		}

		_, err = w.Write([]byte("some data"))
		if err != nil {
			return nil
		}
	}
	if includeCsv {
		w1, err := zipWriter.Create("something.csv")
		if err != nil {
			return nil
		}
		_, err = w1.Write([]byte("some data"))
		if err != nil {
			return nil
		}
	}
	return nil
}

func TestTrainingDataService_SaveUploadedTrainingData_Valid_Success(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{
		BaseTrainingDataLocalDir: baseTmpFolder,
	}

	modelConfig := config.MLModelConfig{
		Name:        "TestModelConfig",
		MLAlgorithm: "TestAlgorithm",
	}
	folderPath := filepath.Join(
		appConfig1.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"validation",
	)
	cleanedTrainingDataPath := filepath.Join(folderPath, "cleaned_training_data.zip")
	trainingDataFilePath := filepath.Join(folderPath, "training_data.zip")
	createTrainingDataZipFile(trainingDataFilePath, true, true)

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	mockCmdRunner := svcmocks.MockCommandRunnerInterface{}
	mockCmdRunner.On("GetCmd", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(exec.Command("sh"))
	mockCmdRunner.On("Run", mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			createTrainingDataZipFile(cleanedTrainingDataPath, true, true)
		})

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		cmdRunner:         &mockCmdRunner,
	}

	fileContents, _ := os.ReadFile(trainingDataFilePath)
	buffer := &mockMultipartFile{Reader: bytes.NewReader(fileContents)}
	hedgeErr := trgSvc.SaveUploadedTrainingData(&modelConfig, buffer)

	if hedgeErr != nil {
		t.Errorf("SaveUploadedTrainingData returned error, but expected success: %v", hedgeErr)
	}

	if !FileExists(trainingDataFilePath) {
		t.Errorf("Expected file `%s` to be found", trainingDataFilePath)
	}
	if !FileExists(cleanedTrainingDataPath) {
		t.Errorf("Expected file `%s` to be found", cleanedTrainingDataPath)
	}
}

func TestTrainingDataService_SaveUploadedTrainingData_NoTrainingFile_Failure(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{
		BaseTrainingDataLocalDir: baseTmpFolder,
	}

	modelConfig := config.MLModelConfig{
		Name:        "TestModelConfig",
		MLAlgorithm: "TestAlgorithm",
	}

	mockCmdRunner := svcmocks.MockCommandRunnerInterface{}
	mockCmdRunner.On("GetCmd", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(exec.Command("sh"))
	mockCmdRunner.On("Run", mock.Anything).Return(nil)

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		cmdRunner:         &mockCmdRunner,
	}

	var file *os.File
	hedgeErr := trgSvc.SaveUploadedTrainingData(&modelConfig, file)
	file.Close()
	if hedgeErr == nil {
		t.Errorf("SaveUploadedTrainingData returned success, but expected error")
	}
	expectedError := hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		"Failed to save uploaded training data",
	)
	assert.Contains(t, hedgeErr.Message(), expectedError.Message())
	assert.Equal(t, hedgeErr.ErrorType(), expectedError.ErrorType())
}

func TestTrainingDataService_SaveUploadedTrainingData_CmdFailure_Failure(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{
		BaseTrainingDataLocalDir: baseTmpFolder,
	}

	modelConfig := config.MLModelConfig{
		Name:        "TestModelConfig",
		MLAlgorithm: "TestAlgorithm",
	}

	folderPath := filepath.Join(
		appConfig1.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"validation",
	)
	trainingDataFilePath := filepath.Join(folderPath, "training_data.zip")
	createTrainingDataZipFile(trainingDataFilePath, true, true)

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	mockCmdRunner := svcmocks.MockCommandRunnerInterface{}
	mockCmdRunner.On("GetCmd", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(exec.Command("sh"))
	mockCmdRunner.On("Run", mock.Anything).
		Return(errors.New("some Error"))

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		cmdRunner:         &mockCmdRunner,
	}

	fileContents, _ := os.ReadFile(trainingDataFilePath)
	buffer := &mockMultipartFile{Reader: bytes.NewReader(fileContents)}
	hedgeErr := trgSvc.SaveUploadedTrainingData(&modelConfig, buffer)

	if hedgeErr == nil {
		t.Errorf("SaveUploadedTrainingData returned success, but expected error")
	}
	expectedError := hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		"Failed to clean Mac-specific attributes from zip file",
	)
	assert.Contains(t, hedgeErr.Message(), expectedError.Message())
	assert.Equal(t, hedgeErr.ErrorType(), expectedError.ErrorType())

	if !FileExists(trainingDataFilePath) {
		t.Errorf("Expected file `%s` to be found", trainingDataFilePath)
	}
}

func TestTrainingDataService_SaveUploadedTrainingData_CleanedTrainingDataFileNotFound_Failure(
	t *testing.T,
) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{
		BaseTrainingDataLocalDir: baseTmpFolder,
	}

	modelConfig := config.MLModelConfig{
		Name:        "TestModelConfig",
		MLAlgorithm: "TestAlgorithm",
	}

	folderPath := filepath.Join(
		appConfig1.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"validation",
	)
	trainingDataFilePath := filepath.Join(folderPath, "training_data.zip")
	createTrainingDataZipFile(trainingDataFilePath, true, true)

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	mockCmdRunner := svcmocks.MockCommandRunnerInterface{}
	mockCmdRunner.On("Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		cmdRunner:         &mockCmdRunner,
	}

	buffer := &mockMultipartFile{Reader: bytes.NewReader([]byte{})}
	hedgeErr := trgSvc.SaveUploadedTrainingData(&modelConfig, buffer)

	if hedgeErr == nil {
		t.Errorf("SaveUploadedTrainingData returned success, but expected error")
	}
	expectedError := hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeBadRequest,
		"Failed to read training data file: not a valid .zip file",
	)
	assert.Contains(t, hedgeErr.Message(), expectedError.Message())
	assert.Equal(t, hedgeErr.ErrorType(), expectedError.ErrorType())

	if !FileExists(trainingDataFilePath) {
		t.Errorf("Expected file `%s` to be found", trainingDataFilePath)
	}
}

func TestTrainingDataService_SaveUploadedTrainingData_MissingConfigFile_Failure(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{
		BaseTrainingDataLocalDir: baseTmpFolder,
	}

	modelConfig := config.MLModelConfig{
		Name:        "TestModelConfig",
		MLAlgorithm: "TestAlgorithm",
	}
	folderPath := filepath.Join(
		appConfig1.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"validation",
	)
	cleanedTrainingDataPath := filepath.Join(folderPath, "cleaned_training_data.zip")
	trainingDataFilePath := filepath.Join(folderPath, "training_data.zip")
	createTrainingDataZipFile(trainingDataFilePath, false, true)

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	mockCmdRunner := svcmocks.MockCommandRunnerInterface{}
	mockCmdRunner.On("GetCmd", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(exec.Command("sh"))
	mockCmdRunner.On("Run", mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			createTrainingDataZipFile(cleanedTrainingDataPath, false, true)
		})

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		cmdRunner:         &mockCmdRunner,
	}

	fileContents, _ := os.ReadFile(trainingDataFilePath)
	buffer := &mockMultipartFile{Reader: bytes.NewReader(fileContents)}
	hedgeErr := trgSvc.SaveUploadedTrainingData(&modelConfig, buffer)

	if hedgeErr == nil {
		t.Errorf("SaveUploadedTrainingData returned success, but expected error")
	}
	expectedError := hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		"Missing or empty config.json file",
	)
	assert.Contains(t, hedgeErr.Message(), expectedError.Message())
	assert.Equal(t, hedgeErr.ErrorType(), expectedError.ErrorType())

	if !FileExists(trainingDataFilePath) {
		t.Errorf("Expected file `%s` to be found", trainingDataFilePath)
	}
	if !FileExists(cleanedTrainingDataPath) {
		t.Errorf("Expected file `%s` to be found", cleanedTrainingDataPath)
	}
}

func TestTrainingDataService_SaveUploadedTrainingData_MissingCSVFile_Failure(t *testing.T) {
	Init_1()

	appConfig1 := &config.MLMgmtConfig{
		BaseTrainingDataLocalDir: baseTmpFolder,
	}

	modelConfig := config.MLModelConfig{
		Name:        "TestModelConfig",
		MLAlgorithm: "TestAlgorithm",
	}
	folderPath := filepath.Join(
		appConfig1.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"validation",
	)
	cleanedTrainingDataPath := filepath.Join(folderPath, "cleaned_training_data.zip")
	trainingDataFilePath := filepath.Join(folderPath, "training_data.zip")
	createTrainingDataZipFile(trainingDataFilePath, true, false)

	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})

	mockCmdRunner := svcmocks.MockCommandRunnerInterface{}
	mockCmdRunner.On("GetCmd", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(exec.Command("sh"))
	mockCmdRunner.On("Run", mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			createTrainingDataZipFile(cleanedTrainingDataPath, true, false)
		})

	trgSvc := &TrainingDataService{
		service:           u.AppService,
		appConfig:         appConfig1,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
		cmdRunner:         &mockCmdRunner,
	}

	fileContents, _ := os.ReadFile(trainingDataFilePath)
	buffer := &mockMultipartFile{Reader: bytes.NewReader(fileContents)}
	hedgeErr := trgSvc.SaveUploadedTrainingData(&modelConfig, buffer)

	if hedgeErr == nil {
		t.Errorf("SaveUploadedTrainingData returned success, but expected error")
	}
	expectedError := hedgeErrors.NewCommonHedgeError(
		hedgeErrors.ErrorTypeServerError,
		"Missing or empty csv file",
	)
	assert.Contains(t, hedgeErr.Message(), expectedError.Message())
	assert.Equal(t, hedgeErr.ErrorType(), expectedError.ErrorType())

	if !FileExists(trainingDataFilePath) {
		t.Errorf("Expected file `%s` to be found", trainingDataFilePath)
	}
	if !FileExists(cleanedTrainingDataPath) {
		t.Errorf("Expected file `%s` to be found", cleanedTrainingDataPath)
	}
}

func TestTrainingDataService_NewDataCollectionProcessor(t *testing.T) {
	Init_1()

	dBMockClient.On("GetAlgorithm", mock.Anything).Return(&config.MLAlgorithmDefinition{}, nil)
	dBMockClient1.On("GetAlgorithm", mock.Anything).
		Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	features := []config.Feature{
		{
			Name: "feature",
		},
	}
	mlDataSourceConfig := config.MLModelDataConfig{
		FeaturesByProfile: map[string][]config.Feature{"key": features},
	}
	modelConfig := config.MLModelConfig{
		Name:               "TestModelConfig",
		MLAlgorithm:        "TestAlgorithm",
		MLDataSourceConfig: mlDataSourceConfig,
	}
	type fields struct {
		service           interfaces.ApplicationService
		appConfig         *config.MLMgmtConfig
		dbClient          redis.MLDbInterface
		dataStoreProvider service.DataStoreProvider
		dataCollector     trainingdata.DataCollectorInterface
	}
	type args struct {
		mlModelConfig *config.MLModelConfig
	}
	arg := args{
		mlModelConfig: &modelConfig,
	}
	arg1 := args{
		mlModelConfig: &config.MLModelConfig{},
	}
	flds := fields{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}
	flds1 := fields{
		service:           u.AppService,
		appConfig:         appConfig,
		dbClient:          &dBMockClient1,
		dataStoreProvider: mockedDataSourceProvider,
		dataCollector:     &mockDataCollector,
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantRes bool
	}{
		{"NewDataCollectionProcessor - Passed", flds, arg, true},
		{"NewDataCollectionProcessor - Failed1", flds1, arg, false},
		{"NewDataCollectionProcessor - Failed2", flds, arg1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trgSvc := &TrainingDataService{
				service:           tt.fields.service,
				appConfig:         tt.fields.appConfig,
				dbClient:          tt.fields.dbClient,
				dataStoreProvider: tt.fields.dataStoreProvider,
				dataCollector:     tt.fields.dataCollector,
			}
			res := trgSvc.NewDataCollectionProcessor(
				tt.args.mlModelConfig,
				trgSvc.service,
				trgSvc.appConfig,
			)
			if (res != nil) != tt.wantRes {
				t.Errorf("NewDataCollectionProcessor() res = %v, wantRes %v", res, tt.wantRes)
			}
			if res != nil {
				if res.PreProcessor == nil {
					t.Error("PreProcessor is nil")
				}
				//if res.Service != trgSvc.service {
				//	t.Errorf("Service is %v, want %v", res.Service, trgSvc.service)
				//}
				if res.AppConfig != trgSvc.appConfig {
					t.Errorf("AppConfig is %v, want %v", res.AppConfig, trgSvc.appConfig)
				}
				if res.MlStorage == nil {
					t.Error("MlStorage is nil")
				}
			}
		})
	}
}

func TestTrainingDataService_CreateTrainingSampleAsync_Success(t *testing.T) {
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})
	Init_1()
	appConfig := config.NewMLMgmtConfig()
	appConfig.BaseTrainingDataLocalDir = baseTmpFolder

	modelConfig := config.MLModelConfig{
		Name:        "MockModelConfig",
		MLAlgorithm: "MockAlgo",
	}

	mlStorage := helpers.NewMLStorage(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		u.AppService.LoggingClient(),
	)
	mockDataCollector.On("GenerateSample", mock.Anything).
		Return([]string{"param1", "param2", "param3"}, nil)
	mockDataCollector.On("GetMlStorage").Return(mlStorage)

	statusFilePath := filepath.Join(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"sample",
		"status.json",
	)
	fetchStatusManager := trainingdata.NewFetchStatusManager(statusFilePath)

	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GetFetchStatusManager").Return(fetchStatusManager)

	trgSvc := &TrainingDataService{
		service:       u.AppService,
		appConfig:     appConfig,
		dataCollector: &mockDataCollector,
	}
	trgSvc.CreateTrainingSampleAsync(&modelConfig, 1)
	status, _ := trainingdata.ReadStatus(statusFilePath)
	expectedStatusResponse := trainingdata.StatusResponse{
		Status: trainingdata.StatusCompleted,
	}
	if status == nil || !reflect.DeepEqual(&expectedStatusResponse, status) {
		t.Errorf("Expected status: %v, received: %v", &expectedStatusResponse, status)
	}
}

func TestTrainingDataService_CreateTrainingSampleAsync_initStatusFileFailure(t *testing.T) {
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})
	Init_1()
	appConfig := config.NewMLMgmtConfig()
	appConfig.BaseTrainingDataLocalDir = baseTmpFolder

	modelConfig := config.MLModelConfig{
		Name:        "MockModelConfig",
		MLAlgorithm: "MockAlgo",
	}

	mlStorage := helpers.NewMLStorage(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		u.AppService.LoggingClient(),
	)
	mockDataCollector.On("GenerateSample", 1).Return([]string{"param1", "param2", "param3"}, nil)
	mockDataCollector.On("GetMlStorage").Return(mlStorage)

	statusFilePath := filepath.Join(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"sample",
		"status.json",
	)
	fetchStatusManager := trainingdata.NewFetchStatusManager(statusFilePath)

	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GetFetchStatusManager").Return(fetchStatusManager)

	expectedStatusResponse := trainingdata.StatusResponse{
		Status: trainingdata.StatusInProgress,
	}
	_ = trainingdata.UpdateStatus(statusFilePath, &expectedStatusResponse)

	trgSvc := &TrainingDataService{
		service:       u.AppService,
		appConfig:     appConfig,
		dataCollector: &mockDataCollector,
	}
	trgSvc.CreateTrainingSampleAsync(&modelConfig, 1)
	status, _ := trainingdata.ReadStatus(statusFilePath)
	if status == nil || !reflect.DeepEqual(&expectedStatusResponse, status) {
		t.Errorf("Expected status: %v, received: %v", &expectedStatusResponse, status)
	}
}

func TestTrainingDataService_CreateTrainingSampleAsync_generateSampleFailure(t *testing.T) {
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})
	Init_1()
	appConfig := config.NewMLMgmtConfig()
	appConfig.BaseTrainingDataLocalDir = baseTmpFolder

	modelConfig := config.MLModelConfig{
		Name:        "MockModelConfig",
		MLAlgorithm: "MockAlgo",
	}

	expectedStatusResponse := trainingdata.StatusResponse{
		Status:       trainingdata.StatusFailed,
		ErrorMessage: "generate sample failed",
	}

	mlStorage := helpers.NewMLStorage(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		u.AppService.LoggingClient(),
	)
	mockDataCollector.On("GenerateSample", 1).
		Return([]string{"param1", "param2", "param3"}, errors.New(expectedStatusResponse.ErrorMessage))
	mockDataCollector.On("GetMlStorage").Return(mlStorage)

	statusFilePath := filepath.Join(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"sample",
		"status.json",
	)
	fetchStatusManager := trainingdata.NewFetchStatusManager(statusFilePath)

	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GetFetchStatusManager").Return(fetchStatusManager)

	trgSvc := &TrainingDataService{
		service:       u.AppService,
		appConfig:     appConfig,
		dataCollector: &mockDataCollector,
	}
	trgSvc.CreateTrainingSampleAsync(&modelConfig, 1)
	status, _ := trainingdata.ReadStatus(statusFilePath)
	if status == nil || !reflect.DeepEqual(&expectedStatusResponse, status) {
		t.Errorf("Expected status: %v, received: %v", &expectedStatusResponse, status)
	}
}

func TestTrainingDataService_CreateTrainingSampleAsync_sampleDataTooSmallFailure(t *testing.T) {
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})
	Init_1()
	appConfig := config.NewMLMgmtConfig()
	appConfig.BaseTrainingDataLocalDir = baseTmpFolder

	modelConfig := config.MLModelConfig{
		Name:        "MockModelConfig",
		MLAlgorithm: "MockAlgo",
	}

	mlStorage := helpers.NewMLStorage(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		u.AppService.LoggingClient(),
	)
	mockDataCollector.On("GenerateSample", mock.Anything).Return([]string{}, nil)
	mockDataCollector.On("GetMlStorage").Return(mlStorage)

	statusFilePath := filepath.Join(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"sample",
		"status.json",
	)
	fetchStatusManager := trainingdata.NewFetchStatusManager(statusFilePath)

	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GetFetchStatusManager").Return(fetchStatusManager)

	trgSvc := &TrainingDataService{
		service:       u.AppService,
		appConfig:     appConfig,
		dataCollector: &mockDataCollector,
	}
	trgSvc.CreateTrainingSampleAsync(&modelConfig, 1)
	status, _ := trainingdata.ReadStatus(statusFilePath)
	expectedStatusResponse := trainingdata.StatusResponse{
		Status:       trainingdata.StatusFailed,
		ErrorMessage: "Sample data is empty",
	}
	if status == nil || !reflect.DeepEqual(&expectedStatusResponse, status) {
		t.Errorf("Expected status: %v, received: %v", &expectedStatusResponse, status)
	}
}

func TestTrainingDataService_CreateTrainingSampleAsync_saveToJsonFailure(t *testing.T) {
	t.Cleanup(func() {
		CleanupFolder(baseTmpFolder, t)
	})
	Init_1()
	appConfig := config.NewMLMgmtConfig()
	appConfig.BaseTrainingDataLocalDir = baseTmpFolder

	modelConfig := config.MLModelConfig{
		Name:        "MockModelConfig",
		MLAlgorithm: "MockAlgo",
	}

	mlStorage := helpers.NewMLStorage(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		u.AppService.LoggingClient(),
	)
	mockDataCollector.On("GenerateSample", mock.Anything).Return([]string{"invalidParamNum"}, nil)
	mockDataCollector.On("GetMlStorage").Return(mlStorage)

	statusFilePath := filepath.Join(
		appConfig.BaseTrainingDataLocalDir,
		modelConfig.MLAlgorithm,
		modelConfig.Name,
		"sample",
		"status.json",
	)
	fetchStatusManager := trainingdata.NewFetchStatusManager(statusFilePath)

	mockDataCollector.On("SetFetchStatusManager", mock.Anything).Return()
	mockDataCollector.On("GetFetchStatusManager").Return(fetchStatusManager)

	trgSvc := &TrainingDataService{
		service:       u.AppService,
		appConfig:     appConfig,
		dataCollector: &mockDataCollector,
	}
	trgSvc.CreateTrainingSampleAsync(&modelConfig, 1)
	status, _ := trainingdata.ReadStatus(statusFilePath)
	expectedStatusResponse := trainingdata.StatusResponse{
		Status:       trainingdata.StatusFailed,
		ErrorMessage: "data not available",
	}
	if status == nil || !reflect.DeepEqual(&expectedStatusResponse, status) {
		t.Errorf("Expected status: %v, received: %v", &expectedStatusResponse, status)
	}
}

func TestTrainingDataService_SetTrainingDataExportDeprecated(t *testing.T) {
	t.Run("SetTrainingDataExportDeprecated - statusCompletedSuccess", func(t *testing.T) {
		t.Cleanup(func() {
			CleanupFolder(baseTmpFolder, t)
		})
		Init_1()

		mlModelConfig := &config.MLModelConfig{
			Name:        "MockModelConfig",
			MLAlgorithm: "MockAlgorithm",
		}

		trgSvc := &TrainingDataService{
			service:   u.AppService,
			appConfig: appConfig,
			exportCtx: make(map[string]*ExportDataContext),
		}

		ctx := ExportDataContext{}
		trgSvc.exportCtx[trgSvc.getExportDataCtxKey(mlModelConfig)] = &ctx

		statusFilePath := filepath.Join(
			appConfig.BaseTrainingDataLocalDir,
			mlModelConfig.MLAlgorithm,
			mlModelConfig.Name,
			"data_export",
			"status.json",
		)
		statusResponse := trainingdata.StatusResponse{
			Status: trainingdata.StatusCompleted,
		}
		_ = trainingdata.UpdateStatus(statusFilePath, &statusResponse)

		err := trgSvc.SetTrainingDataExportDeprecated(mlModelConfig)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			t.FailNow()
		}
		expectedStatusResponse := trainingdata.StatusResponse{
			Status: trainingdata.StatusDeprecated,
		}
		status, _ := trainingdata.ReadStatus(statusFilePath)
		if status == nil || !reflect.DeepEqual(&expectedStatusResponse, status) {
			t.Errorf("Expected status: %v, received: %v", &expectedStatusResponse, status)
		}
	})
	t.Run("SetTrainingDataExportDeprecated - statusFailedSuccess", func(t *testing.T) {
		t.Cleanup(func() {
			CleanupFolder(baseTmpFolder, t)
		})
		Init_1()

		mlModelConfig := &config.MLModelConfig{
			Name:        "MockModelConfig",
			MLAlgorithm: "MockAlgorithm",
		}

		trgSvc := &TrainingDataService{
			service:   u.AppService,
			appConfig: appConfig,
			exportCtx: make(map[string]*ExportDataContext),
		}

		ctx := ExportDataContext{}
		trgSvc.exportCtx[trgSvc.getExportDataCtxKey(mlModelConfig)] = &ctx

		statusFilePath := filepath.Join(
			appConfig.BaseTrainingDataLocalDir,
			mlModelConfig.MLAlgorithm,
			mlModelConfig.Name,
			"data_export",
			"status.json",
		)
		expectedStatusResponse := trainingdata.StatusResponse{
			Status: trainingdata.StatusFailed,
		}
		_ = trainingdata.UpdateStatus(statusFilePath, &expectedStatusResponse)

		err := trgSvc.SetTrainingDataExportDeprecated(mlModelConfig)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			t.FailNow()
		}
		status, _ := trainingdata.ReadStatus(statusFilePath)
		if status == nil || !reflect.DeepEqual(&expectedStatusResponse, status) {
			t.Errorf("Expected status: %v, received: %v", &expectedStatusResponse, status)
		}
	})
	t.Run("SetTrainingDataExportDeprecated - statusFileNotExistsFailed", func(t *testing.T) {
		t.Cleanup(func() {
			CleanupFolder(baseTmpFolder, t)
		})
		Init_1()

		mlModelConfig := &config.MLModelConfig{
			Name:        "MockModelConfig",
			MLAlgorithm: "MockAlgorithm",
		}

		trgSvc := &TrainingDataService{
			service:   u.AppService,
			appConfig: appConfig,
		}
		err := trgSvc.SetTrainingDataExportDeprecated(mlModelConfig)
		if err == nil {
			t.Errorf("Error is nil")
			t.FailNow()
		}
	})
	t.Run("SetTrainingDataExportDeprecated - readStatusFileFailed", func(t *testing.T) {
		t.Cleanup(func() {
			CleanupFolder(baseTmpFolder, t)
		})
		Init_1()

		mlModelConfig := &config.MLModelConfig{
			Name:        "MockModelConfig",
			MLAlgorithm: "MockAlgorithm",
		}

		trgSvc := &TrainingDataService{
			service:   u.AppService,
			appConfig: appConfig,
		}
		statusFilePath := filepath.Join(
			appConfig.BaseTrainingDataLocalDir,
			mlModelConfig.MLAlgorithm,
			mlModelConfig.Name,
			"data_export",
			"status.json",
		)
		helpers.SaveFile(statusFilePath, []byte{})
		err := trgSvc.SetTrainingDataExportDeprecated(mlModelConfig)
		if err == nil {
			t.Errorf("Error is nil")
			t.FailNow()
		}
	})
	t.Run("SetTrainingDataExportDeprecated - exportCtxNotFoundFailed", func(t *testing.T) {
		t.Cleanup(func() {
			CleanupFolder(baseTmpFolder, t)
		})
		Init_1()

		mlModelConfig := &config.MLModelConfig{
			Name:        "MockModelConfig",
			MLAlgorithm: "MockAlgorithm",
		}

		trgSvc := &TrainingDataService{
			service:   u.AppService,
			appConfig: appConfig,
			exportCtx: make(map[string]*ExportDataContext),
		}

		statusFilePath := filepath.Join(
			appConfig.BaseTrainingDataLocalDir,
			mlModelConfig.MLAlgorithm,
			mlModelConfig.Name,
			"data_export",
			"status.json",
		)
		statusResponse := trainingdata.StatusResponse{
			Status: trainingdata.StatusInProgress,
		}
		_ = trainingdata.UpdateStatus(statusFilePath, &statusResponse)

		err := trgSvc.SetTrainingDataExportDeprecated(mlModelConfig)
		if err == nil {
			t.Errorf("Error is nil")
			t.FailNow()
		}
	})
}

type mockMultipartFile struct {
	*bytes.Reader
}

func (m *mockMultipartFile) Close() error {
	return nil // no-op, as there's nothing to close for a buffer
}
