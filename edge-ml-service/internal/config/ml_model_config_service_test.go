/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	hedgeErrors "hedge/common/errors"
	"hedge/common/service"
	"hedge/edge-ml-service/pkg/db"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"hedge/edge-ml-service/pkg/helpers"
	svs_mocks "hedge/mocks/hedge/common/service"
	metricDbMock "hedge/mocks/hedge/edge-ml-service/pkg/db"
	redisMock "hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	helpersMock "hedge/mocks/hedge/edge-ml-service/pkg/helpers"
)

var errTest = errors.New("dummy error")

func TestNewMLModelConfigService(t *testing.T) {
	type args struct {
		service            interfaces.ApplicationService
		connectionConfig   *config.MLMgmtConfig
		dbClient           redis.MLDbInterface
		dataSourceProvider service.DataStoreProvider
	}
	mockedDBClient := redisMock.MockMLDbInterface{}

	args1 := args{
		service:            u.AppService,
		connectionConfig:   testConnectionConfig,
		dbClient:           &mockedDBClient,
		dataSourceProvider: testDataSourceProvider,
	}
	mockService := &MLModelConfigService{
		service:      u.AppService,
		mlMgmtConfig: testConnectionConfig,
		dbClient:     &mockedDBClient,
		metricDataDb: db.NewMetricDataDb(
			u.AppService,
			testConnectionConfig,
			testDataSourceProvider,
		),
	}
	tests := []struct {
		name string
		args args
		want *MLModelConfigService
	}{
		{"NewMlModelConfigService - Passed", args1, mockService},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMlModelConfigService(tt.args.service, tt.args.connectionConfig, tt.args.dbClient, tt.args.dataSourceProvider, nil, nil); !reflect.DeepEqual(
				got,
				tt.want,
			) {
				t.Errorf("NewMlModelConfigService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMLModelConfigService_DeleteTrainingDataConfig(t *testing.T) {
	t.Run("DeleteMLModelConfig - Passed", func(t *testing.T) {
		testModelDeployments := buildTestModelDeployments(
			ml_model.ModelUnDeployed,
			"ModelUnDeployed",
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return(testModelDeployments, nil)
		mockedDBClient.On("GetMLTrainingJobsByConfig", mock.Anything, mock.Anything).
			Return([]job.TrainingJobDetails{}, nil)
		mockedDBClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).
			Return([]config.MLEventConfig{}, nil)
		mockedDBClient.On("DeleteMLModelConfig", mock.Anything, mock.Anything).Return(nil)
		mockedDBClient.On("DeleteMLModelsByConfig", mock.Anything, mock.Anything).Return(nil)

		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("exampleAlgorithm", "exampleConfig")
		if err != nil {
			t.Errorf("DeleteMLModelConfig() error = %v, wantErr false", err)
		}
	})
	t.Run("DeleteMLModelConfig - Failed (Error Deleting Models)", func(t *testing.T) {
		testModelDeployments := buildTestModelDeployments(ml_model.ModelDeployed, "ModelDeployed")
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return(testModelDeployments, nil)
		mockedDBClient.On("GetMLTrainingJobsByConfig", mock.Anything, "").
			Return([]job.TrainingJobDetails{}, nil)
		mockedDBClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).
			Return([]config.MLEventConfig{}, nil)
		mockedDBClient.On("DeleteMLModelConfig", mock.Anything, mock.Anything).Return(nil)
		mockedDBClient.On("DeleteMLModelsByConfig", mock.Anything, mock.Anything).Return(errTest)

		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("exampleAlgorithm", "configWithTrainedModels")
		if err == nil {
			t.Errorf("DeleteMLModelConfig() expected error, got nil")
		}
	})
	t.Run("DeleteMLModelConfig - Failed (Error Retrieving Model Config)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{}, nil)
		mockedDBClient.On("GetMLTrainingJobsByConfig", mock.Anything, mock.Anything).
			Return([]job.TrainingJobDetails{}, nil)
		mockedDBClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).
			Return([]config.MLEventConfig{}, nil)
		mockedDBClient.On("DeleteMLModelConfig", mock.Anything, mock.Anything).
			Return(testHedgeError)

		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("exampleAlgorithm", "exampleConfig")
		if err == nil {
			t.Errorf("DeleteMLModelConfig() expected error, got nil")
		}
	})
}

func TestMLModelConfigService_GetAllMLModelConfigs(t *testing.T) {
	t.Run("GetAllMLModelConfigs - Passed", func(t *testing.T) {
		testMlModelConfigs := []config.MLModelConfig{
			*buildTestMLModelConfigByMLModelDataConfig(config.MLModelDataConfig{}),
		}
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllMLModelConfigs", mock.Anything).Return(testMlModelConfigs, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		got, err := cfgSvc.GetAllMLModelConfigs("exampleAlgorithm")
		if err != nil {
			t.Errorf("GetAllMLModelConfigs() error = %v, wantErr false", err)
		}
		if !reflect.DeepEqual(got, testMlModelConfigs) {
			t.Errorf("GetAllMLModelConfigs() got = %v, want %v", got, testMlModelConfigs)
		}
	})
	t.Run("GetAllMLModelConfigs - Failed", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllMLModelConfigs", mock.Anything).Return(nil, testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		got, err := cfgSvc.GetAllMLModelConfigs("invalidAlgorithm")
		if err == nil {
			t.Errorf("GetAllMLModelConfigs() expected error, got nil")
		}
		if got != nil {
			t.Errorf("GetAllMLModelConfigs() got = %v, want nil", got)
		}
	})
}

func TestMLModelConfigService_GetMetricNames(t *testing.T) {
	t.Run("GetMetricNames - Passed", func(t *testing.T) {
		mockLabels := `{"data": ["metric1", "metric2"]}`
		mockedMetricDataDb := &metricDbMock.MockMetricDataDbInterface{}
		mockedMetricDataDb.On("GetTSLabels", "exampleLabel").Return([]byte(mockLabels), nil)

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			metricDataDb: mockedMetricDataDb,
		}

		got, err := cfgSvc.GetMetricNames("exampleLabel")
		if err != nil {
			t.Errorf("GetMetricNames() error = %v, wantErr false", err)
		}
		if !reflect.DeepEqual(got, []interface{}{"metric1", "metric2"}) {
			t.Errorf("GetMetricNames() got = %v, want %v", got, []interface{}{"metric1", "metric2"})
		}
	})
	t.Run("GetMetricNames - Failed", func(t *testing.T) {
		mockedMetricDataDb := &metricDbMock.MockMetricDataDbInterface{}
		mockedMetricDataDb.On("GetTSLabels", "errorLabel").Return(nil, errTest)

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			metricDataDb: mockedMetricDataDb,
		}

		got, err := cfgSvc.GetMetricNames("errorLabel")
		if err == nil {
			t.Errorf("GetMetricNames() expected error, got nil")
		}
		if got != nil {
			t.Errorf("GetMetricNames() got = %v, want nil", got)
		}
	})
}

func TestMLModelConfigService_GetAlgorithm(t *testing.T) {
	t.Run("GetAlgorithm - Passed", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)

		cfgSvc := &MLModelConfigService{
			dbClient: &mockedDBClient,
		}

		got, err := cfgSvc.GetAlgorithm(testMlAlgorithmName)
		if err != nil {
			t.Errorf("GetAlgorithm() error = %v, wantErr %v", err, false)
			return
		}
		if !reflect.DeepEqual(got, testMlAlgorithmDefinition) {
			t.Errorf("GetAlgorithm() got = %v, want %v", got, testMlAlgorithmDefinition)
		}
	})
	t.Run("GetAlgorithm - Failed", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, testHedgeError)

		cfgSvc := &MLModelConfigService{
			dbClient: &mockedDBClient,
		}

		got, err := cfgSvc.GetAlgorithm(testMlAlgorithmName)
		if err == nil {
			t.Errorf("GetAlgorithm() expected error, got nil")
		}
		if got != nil {
			t.Errorf("GetAlgorithm() got = %v, want nil", got)
		}
	})
}

func TestMLModelConfigService_GetAllAlgorithms(t *testing.T) {
	t.Run("GetAllAlgorithms - Passed", func(t *testing.T) {
		testAlgorithmConfigs := []*config.MLAlgorithmDefinition{
			buildTestMlAlgoDefinition(testMlAlgorithmName, "TestType", 49000, false),
		}
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllAlgorithms").Return(testAlgorithmConfigs, nil)

		cfgSvc := &MLModelConfigService{
			dbClient: &mockedDBClient,
		}

		got, err := cfgSvc.GetAllAlgorithms()
		if err != nil {
			t.Errorf("GetAllAlgorithms() error = %v, wantErr %v", err, false)
			return
		}
		if !reflect.DeepEqual(got, testAlgorithmConfigs) {
			t.Errorf("GetAllAlgorithms() got = %v, want %v", got, testAlgorithmConfigs)
		}
	})
	t.Run("GetAllAlgorithms - Failed", func(t *testing.T) {
		dbMockClient := redisMock.MockMLDbInterface{}
		dbMockClient.On("GetAllAlgorithms").
			Return([]*config.MLAlgorithmDefinition{}, testHedgeError)

		cfgSvc := &MLModelConfigService{
			dbClient: &dbMockClient,
		}

		got, err := cfgSvc.GetAllAlgorithms()
		if err == nil {
			t.Errorf("GetAllAlgorithms() expected error, got nil")
		}
		if len(got) > 0 {
			t.Errorf("GetAllAlgorithms() got = %v, want nil or empty slice", got)
		}
	})
}

func TestMLModelConfigService_CreateAlgorithm(t *testing.T) {
	mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
	mockRegistryConfig.On("GetImageDigest", mock.Anything).Return(testImageDigest, nil)

	t.Run("CreateAlgorithm - Passed", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, nil)
		mockedDBClient.On("CreateAlgorithm", mock.Anything).Return(nil)

		testConnectionConfig.TrainingProvider = "Hedge"
		mockedADEConnection := svs_mocks.MockADEConnectionInterface{}

		cfgSvc := &MLModelConfigService{
			service:           u.AppService,
			dbClient:          &mockedDBClient,
			mlMgmtConfig:      testConnectionConfig,
			connectionHandler: &mockedADEConnection,
			registryConfig:    mockRegistryConfig,
		}

		err := cfgSvc.CreateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		if err != nil {
			t.Errorf("CreateAlgorithm() unexpected error: %v", err)
		}
	})
	t.Run("CreateAlgorithm - Failed (Get algorithm error)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.CreateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		if err == nil {
			t.Errorf("CreateAlgorithm() expected error, got nil")
		}
	})
	t.Run("CreateAlgorithm - Failed (Algorithm exists error)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).
			Return(&config.MLAlgorithmDefinition{}, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.CreateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		if err == nil {
			t.Errorf("CreateAlgorithm() expected error, got nil")
		}
	})
	t.Run("CreateAlgorithm - Failed (Non-ADE provider)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, nil)
		mockedDBClient.On("CreateAlgorithm", mock.Anything).Return(nil)

		testConnectionConfig.TrainingProvider = "Non-ADE"
		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			dbClient:       &mockedDBClient,
			mlMgmtConfig:   testConnectionConfig,
			registryConfig: mockRegistryConfig,
		}

		err := cfgSvc.CreateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		if err != nil {
			t.Errorf("CreateAlgorithm() expected no error, got error:%v", err)
		}
	})
	t.Run("CreateAlgorithm - Failed (Add algorithm error)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, nil)
		mockedDBClient.On("CreateAlgorithm", mock.Anything).Return(testHedgeError)

		testConnectionConfig.TrainingProvider = "Hedge"
		//mockedADEConnection := svs_mocks.MockADEConnectionInterface{}

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
			//connectionHandler: &mockedADEConnection,
			registryConfig: mockRegistryConfig,
		}

		err := cfgSvc.CreateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		if err == nil {
			t.Errorf("CreateAlgorithm() expected error, got nil")
		}
	})
	t.Run("CreateAlgorithm - Failed (Invalid Algorithm)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition("", "TestType", 49000, false)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, nil)
		mockedDBClient.On("CreateAlgorithm", mock.Anything).Return(testHedgeError)

		testConnectionConfig.TrainingProvider = "Hedge"
		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
			//connectionHandler: &mockedADEConnection,
			registryConfig: mockRegistryConfig,
		}

		err := cfgSvc.CreateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		if err == nil {
			t.Errorf("CreateAlgorithm() expected error due to invalid algorithm, got nil")
		}
	})
	t.Run("CreateAlgorithm - Failed (Image validation failed)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition("", "TestType", 49000, false)
		mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", mock.Anything).
			Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, testHedgeError.Error()))

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, nil)

		testConnectionConfig.TrainingProvider = "Hedge"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
			//connectionHandler: &mockedADEConnection,
			registryConfig: mockRegistryConfig,
		}

		err := cfgSvc.CreateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		if err == nil {
			t.Errorf("CreateAlgorithm() expected error due to invalid algorithm, got nil")
		}
	})
}

func TestMLModelConfigService_UpdateAlgorithm(t *testing.T) {
	t.Run("UpdateAlgorithm - Passed", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		updatedMlAlgorithDefinition := &config.MLAlgorithmDefinition{
			Name:                  testMlAlgorithmName,
			TrainerImagePath:      testMlAlgorithmDefinition.TrainerImagePath,
			PredictionImagePath:   testMlAlgorithmDefinition.PredictionImagePath,
			DefaultPredictionPort: 49000,
			IsOotb:                true,
		}

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)
		mockedDBClient.On("UpdateAlgorithm", mock.Anything).Return(nil)

		mockRegistryConfig := helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", mock.Anything, mock.Anything).
			Return(testImageDigest, nil)

		testConnectionConfig.TrainingProvider = "Hedge"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
			//connectionHandler: &mockedADEConnection,
			registryConfig: &mockRegistryConfig,
		}

		err := cfgSvc.UpdateAlgorithm(*updatedMlAlgorithDefinition, true, testUserId)
		assert.Nil(t, err, "UpdateAlgorithm() unexpected error: %v", err)
	})
	t.Run("UpdateAlgorithm - Passed (all validations passed for ootb algo", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			true,
		)
		updatedMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49777,
			true,
		)

		testPredictionImageNameAndTag := strings.Split(
			testMlAlgorithmDefinition.PredictionImagePath,
			":",
		)
		updatedMlAlgorithmDefinition.PredictionImagePath = testPredictionImageNameAndTag[0] + ":" + testPredictionImageNameAndTag[1] + "1"
		testTrainerImageNameAndTag := strings.Split(testMlAlgorithmDefinition.TrainerImagePath, ":")
		updatedMlAlgorithmDefinition.TrainerImagePath = testTrainerImageNameAndTag[0] + ":" + testTrainerImageNameAndTag[1] + "1"

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)
		mockedDBClient.On("UpdateAlgorithm", mock.Anything).Return(nil)

		mockRegistryConfig := helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", mock.Anything).Return(testImageDigest, nil)

		testConnectionConfig.TrainingProvider = "Hedge"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
			//connectionHandler: &mockedADEConnection,
			registryConfig: &mockRegistryConfig,
		}

		err := cfgSvc.UpdateAlgorithm(*updatedMlAlgorithmDefinition, true, testUserId)
		assert.Nil(t, err, "UpdateAlgorithm() unexpected error: %v", err)
	})
	t.Run(
		"UpdateAlgorithm - Failed (PredictionImagePath validation failed for ootb algo)",
		func(t *testing.T) {
			testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
				testMlAlgorithmName,
				"TestType",
				49000,
				true,
			)
			updatedMlAlgorithmDefinition := buildTestMlAlgoDefinition(
				testMlAlgorithmName,
				"TestType",
				49777,
				true,
			)

			testPredictionImageNameAndTag := strings.Split(
				testMlAlgorithmDefinition.PredictionImagePath,
				":",
			)
			updatedMlAlgorithmDefinition.PredictionImagePath = testPredictionImageNameAndTag[0] + "1:" + testPredictionImageNameAndTag[1]

			mockedDBClient := redisMock.MockMLDbInterface{}
			mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)

			testConnectionConfig.TrainingProvider = "ADE"

			cfgSvc := &MLModelConfigService{
				service:      u.AppService,
				dbClient:     &mockedDBClient,
				mlMgmtConfig: testConnectionConfig,
				//registryConfig:
			}

			err := cfgSvc.UpdateAlgorithm(*updatedMlAlgorithmDefinition, false, testUserId)
			assert.NotNil(t, err, "UpdateAlgorithm() expected error, got nil")
			assert.Contains(
				t,
				err.Error(),
				"Failed to update the prediction image path of the out of the box algorithm TestAlgo: the image name is not allowed to be updated",
				"Error message mismatch",
			)
		})
	t.Run(
		"UpdateAlgorithm - Failed (TrainerImagePath validation failed for ootb algo)",
		func(t *testing.T) {
			testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
				testMlAlgorithmName,
				"TestType",
				49000,
				true,
			)
			updatedMlAlgorithmDefinition := buildTestMlAlgoDefinition(
				testMlAlgorithmName,
				"TestType",
				49777,
				true,
			)

			testTrainerImageNameAndTag := strings.Split(
				testMlAlgorithmDefinition.TrainerImagePath,
				":",
			)
			updatedMlAlgorithmDefinition.TrainerImagePath = testTrainerImageNameAndTag[0] + "1:" + testTrainerImageNameAndTag[1]

			mockedDBClient := redisMock.MockMLDbInterface{}
			mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)

			testConnectionConfig.TrainingProvider = "ADE"

			cfgSvc := &MLModelConfigService{
				service:      u.AppService,
				dbClient:     &mockedDBClient,
				mlMgmtConfig: testConnectionConfig,
			}

			err := cfgSvc.UpdateAlgorithm(*updatedMlAlgorithmDefinition, false, testUserId)
			assert.NotNil(t, err, "UpdateAlgorithm() expected error, got nil")
			assert.Contains(
				t,
				err.Error(),
				"Failed to update the trainer image path of the out of the box algorithm TestAlgo: the image name is not allowed to be updated",
				"Error message mismatch",
			)
		},
	)
	t.Run("UpdateAlgorithm - Failed (Algorithm Not Found)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.UpdateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		assert.NotNil(t, err, "UpdateAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"Failed to update nonexistent algorithm TestAlgo",
			"Error message mismatch",
		)
	})
	t.Run("UpdateAlgorithm - Failed (DB error on GetAlgorithm)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.UpdateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		assert.NotNil(t, err, "UpdateAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"Failed to update algorithm TestAlgo",
			"Error message mismatch",
		)
	})
	t.Run("UpdateAlgorithm - Failed (Unsupported training provider)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).
			Return(&config.MLAlgorithmDefinition{}, nil)

		testConnectionConfig.TrainingProvider = "UnsupportedProvider"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.UpdateAlgorithm(*testMlAlgorithmDefinition, false, testUserId)
		assert.NotNil(t, err, "UpdateAlgorithm() expected error, got nil")
		assert.Contains(t, err.Error(), "image path has been changed for algo", "Error message mismatch")
	})

	t.Run("UpdateAlgorithm - Failed (DB error on update)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).
			Return(&config.MLAlgorithmDefinition{}, nil)
		mockedDBClient.On("UpdateAlgorithm", mock.Anything).Return(testHedgeError)

		mockRegistryConfig := helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", mock.Anything).Return(testImageDigest, nil)

		testConnectionConfig.TrainingProvider = "Hedge"

		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			dbClient:       &mockedDBClient,
			mlMgmtConfig:   testConnectionConfig,
			registryConfig: &mockRegistryConfig,
		}

		err := cfgSvc.UpdateAlgorithm(*testMlAlgorithmDefinition, true, testUserId)
		assert.NotNil(t, err, "UpdateAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"Failed to update algorithm TestAlgo",
			"Error message mismatch",
		)
	})
	t.Run("UpdateAlgorithm - Failed (Image validation failed)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).
			Return(&config.MLAlgorithmDefinition{}, nil)

		mockRegistryConfig := helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", testMlAlgorithmDefinition.PredictionImagePath).
			Return(testImageDigest, nil)
		mockRegistryConfig.On("GetImageDigest", testMlAlgorithmDefinition.TrainerImagePath).
			Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, testHedgeError.Error()))

		testConnectionConfig.TrainingProvider = "Hedge"

		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			dbClient:       &mockedDBClient,
			mlMgmtConfig:   testConnectionConfig,
			registryConfig: &mockRegistryConfig,
		}

		err := cfgSvc.UpdateAlgorithm(*testMlAlgorithmDefinition, true, testUserId)
		assert.NotNil(t, err, "UpdateAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"Trainer image validation failed for algo TestAlgo, image 'lpade:test-training-image-path', generateDigest 'true', err: hedge dummy error",
			"Error message mismatch",
		)
	})
}

func TestMLModelConfigService_DeleteAlgorithm(t *testing.T) {
	t.Run("DeleteAlgorithm - Passed", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)
		mockedDBClient.On("GetAllMLModelConfigs", mock.Anything).
			Return([]config.MLModelConfig{}, nil)
		mockedDBClient.On("DeleteAlgorithm", mock.Anything).Return(nil)

		testConnectionConfig.TrainingProvider = "Hedge"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.DeleteAlgorithm(testMlAlgorithmName)
		assert.Nil(t, err, "DeleteAlgorithm() unexpected error: %v", err)
	})
	t.Run("DeleteAlgorithm - Failed (isOotb algo is immutable)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			true,
		)

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.DeleteAlgorithm(testMlAlgorithmName)
		assert.NotNil(t, err, "UpdateAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"It is not allowed to delete the out of the box algorithm TestAlgo",
			"Error message mismatch",
		)
	})

	t.Run("DeleteAlgorithm - Failed (DB error on GetAlgorithm)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, testHedgeError)

		testConnectionConfig.TrainingProvider = "ADE"
		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.DeleteAlgorithm(testMlAlgorithmName)
		assert.NotNil(t, err, "DeleteAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"Failed to delete algorithm TestAlgo",
			"Error message mismatch",
		)
	})
	t.Run("DeleteAlgorithm - Failed (DB error on GetAllMLModelConfigs)", func(t *testing.T) {
		testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
			Name:   testMlAlgorithmName,
			IsOotb: false,
		}

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)
		mockedDBClient.On("GetAllMLModelConfigs", mock.Anything).
			Return([]config.MLModelConfig{}, testHedgeError)

		testConnectionConfig.TrainingProvider = "ADE"
		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.DeleteAlgorithm(testMlAlgorithmName)
		assert.NotNil(t, err, "DeleteAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"Failed to delete algorithm TestAlgo",
			"Error message mismatch",
		)
	})
	t.Run("DeleteAlgorithm - Failed (Algorithm not found)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, nil)

		testConnectionConfig.TrainingProvider = "ADE"
		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.DeleteAlgorithm(testMlAlgorithmName)
		assert.NotNil(t, err, "DeleteAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"Failed to delete non-existent algorithm TestAlgo",
			"Error message mismatch",
		)
	})
	t.Run("DeleteAlgorithm - Failed (Out-of-the-Box algorithm)", func(t *testing.T) {
		testMlAlgorithmName := "OOTBAlgorithm"
		testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
			Name:   testMlAlgorithmName,
			IsOotb: true,
		}

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", testMlAlgorithmName).
			Return(testMlAlgorithmDefinition, nil)

		testConnectionConfig.TrainingProvider = "ADE"
		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.DeleteAlgorithm(testMlAlgorithmName)
		assert.NotNil(t, err, "DeleteAlgorithm() expected error, got nil")
		assert.Contains(t, err.Error(), "out of the box", "Error message mismatch")
	})
	t.Run("DeleteAlgorithm - Failed (ML model configs exists)", func(t *testing.T) {
		testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
			Name:   testMlAlgorithmName,
			IsOotb: false,
		}

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)
		mockedDBClient.On("GetAllMLModelConfigs", mock.Anything).
			Return([]config.MLModelConfig{{Name: "TestConfig"}}, nil)

		testConnectionConfig.TrainingProvider = "ADE"
		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.DeleteAlgorithm(testMlAlgorithmName)
		assert.NotNil(t, err, "DeleteAlgorithm() expected error, got nil")
		assert.Contains(t, err.Error(), "model configurations", "Error message mismatch")
	})

	t.Run("DeleteAlgorithm - Failed (DB error on delete)", func(t *testing.T) {
		testMlAlgorithmName := "TestAlgorithm"
		testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
			Name:   testMlAlgorithmName,
			IsOotb: false,
		}

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)
		mockedDBClient.On("GetAllMLModelConfigs", mock.Anything).Return(nil, nil)
		mockedDBClient.On("DeleteAlgorithm", mock.Anything).Return(testHedgeError)

		testConnectionConfig.TrainingProvider = "Hedge"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: testConnectionConfig,
		}

		err := cfgSvc.DeleteAlgorithm(testMlAlgorithmName)
		assert.NotNil(t, err, "DeleteAlgorithm() expected error, got nil")
		assert.Contains(
			t,
			err.Error(),
			"Failed to delete algorithm TestAlgorithm",
			"Error message mismatch",
		)
	})
}

func TestMLModelConfigService_GetAllocatedPortNo(t *testing.T) {
	t.Run("GetAllocatedPortNo - Passed", func(t *testing.T) {
		testMlAlgoDefinitions := []*config.MLAlgorithmDefinition{
			buildTestMlAlgoDefinition("TestAlgoPort49000", "TestType", 49000, false),
			buildTestMlAlgoDefinition("TestAlgoPort49001", "TestType", 49001, false),
		}
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllAlgorithms").Return(testMlAlgoDefinitions, nil)

		cfgSvc := &MLModelConfigService{
			dbClient: &mockedDBClient,
		}

		got := cfgSvc.GetAllocatedPortNo()
		want := int64(49002)
		if got != want {
			t.Errorf("GetAllocatedPortNo() got = %v, want %v", got, want)
		}
	})
	t.Run("GetAllocatedPortNo - Failed (No Algorithms)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllAlgorithms").Return([]*config.MLAlgorithmDefinition{}, nil)

		cfgSvc := &MLModelConfigService{
			dbClient: &mockedDBClient,
		}

		got := cfgSvc.GetAllocatedPortNo()
		want := int64(49000)
		if got != want {
			t.Errorf("GetAllocatedPortNo() got = %v, want %v", got, want)
		}
	})
	t.Run("GetAllocatedPortNo - Failed (Error Fetching Algorithms)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllAlgorithms").Return(nil, testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		got := cfgSvc.GetAllocatedPortNo()
		want := int64(0)
		if got != want {
			t.Errorf("GetAllocatedPortNo() got = %v, want %v", got, want)
		}
	})
}

func TestMLModelConfigService_GetAlgorithmTypes(t *testing.T) {
	expectedTypes := []string{
		helpers.ANOMALY_ALGO_TYPE,
		helpers.CLASSIFICATION_ALGO_TYPE,
		helpers.REGRESSION_ALGO_TYPE,
		helpers.TIMESERIES_ALGO_TYPE,
	}
	t.Run("GetAlgorithmTypes - Passed", func(t *testing.T) {
		got := GetAlgorithmTypes()
		sort.Strings(got)
		sort.Strings(expectedTypes)

		if !reflect.DeepEqual(got, expectedTypes) {
			t.Errorf("GetAlgorithmTypes() got = %v, want %v", got, expectedTypes)
		}
	})
}

func TestMLModelConfigService_GetMLModelConfig(t *testing.T) {
	testFeatures := make(map[string][]config.Feature)
	testFeatures["featureKey"] = []config.Feature{}
	expectedMLModelConfig := buildTestMLModelConfigByMLModelDataConfig(
		buildTestMLModelDataConfig(
			testFeatures,
			"http://testalgo:49000/api/v3/predict/TestAlgo/TestMlModelConfig",
			"enriched/events/device/featureKey/#",
		),
	)

	t.Run("GetMLModelConfig - Passed", func(t *testing.T) {
		testMLModelConfig := buildTestMLModelConfigByMLModelDataConfig(
			buildTestMLModelDataConfig(testFeatures, "", ""),
		)
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetMlModelConfig", mock.Anything, mock.Anything).
			Return(*testMLModelConfig, nil)
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		got, err := cfgSvc.GetMLModelConfig(testMlAlgorithmName, testMlModelConfigName)
		if err != nil {
			t.Errorf("GetMLModelConfig() unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, expectedMLModelConfig) {
			t.Errorf("GetMLModelConfig() got = %v, want %v", got, expectedMLModelConfig)
		}
	})
	t.Run("GetMLModelConfig - Passed (Add Topic Name)", func(t *testing.T) {
		testMLModelConfig := buildTestMLModelConfigByMLModelDataConfig(
			buildTestMLModelDataConfig(testFeatures, "",
				"enriched/events/device/featureKey/#"),
		)
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetMlModelConfig", mock.Anything, mock.Anything).
			Return(*testMLModelConfig, nil)
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		got, err := cfgSvc.GetMLModelConfig(testMlAlgorithmName, testMlModelConfigName)
		if err != nil {
			t.Errorf("GetMLModelConfig() unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, expectedMLModelConfig) {
			t.Errorf("GetMLModelConfig() got = %v, want %v", got, expectedMLModelConfig)
		}
	})
	t.Run("GetMLModelConfig - Failed (Error Retrieving Model Config)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetMlModelConfig", mock.Anything, mock.Anything).
			Return(config.MLModelConfig{}, testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		got, err := cfgSvc.GetMLModelConfig(testMlAlgorithmName, testMlModelConfigName)
		if err == nil {
			t.Errorf("GetMLModelConfig() expected error, got nil")
		}
		if got != nil {
			t.Errorf("GetMLModelConfig() got = %v, want nil", got)
		}
	})
}

func TestMLModelConfigService_SaveMLModelConfig(t *testing.T) {
	testFeatures := map[string][]config.Feature{
		"Profile1": {
			{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
			{Name: "Feature2", IsInput: true, IsOutput: true, FromExternalDataSource: false},
		},
	}
	t.Run("SaveMLModelConfig - Passed", func(t *testing.T) {
		testMlModelConfig := buildTestMLModelConfig(
			helpers.TIMESERIES_ALGO_TYPE,
			testFeatures,
			map[string]int{},
			[]string{},
		)
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			helpers.TIMESERIES_ALGO_TYPE,
			49000,
			false,
		)

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("SaveMLModelConfig", mock.Anything).Return("TestConfig", nil)

		cfgSvc := &MLModelConfigService{
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.SaveMLModelConfig(testMlModelConfig, testMlAlgorithmDefinition)
		require.NoError(t, err)
	})
	t.Run("SaveMLModelConfig - Failed (Invalid data collection parameters)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{}

		testMlModelConfig := buildTestMLModelConfig(
			helpers.TIMESERIES_ALGO_TYPE,
			testFeatures,
			map[string]int{},
			[]string{},
		)
		testMlModelConfig.MLDataSourceConfig.TrainingDataSourceConfig.DataCollectionTotalDurationSec = -1
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			helpers.TIMESERIES_ALGO_TYPE,
			49000,
			false,
		)

		err := cfgSvc.SaveMLModelConfig(testMlModelConfig, testMlAlgorithmDefinition)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "training duration & sampling interval need to be >= 0")
	})
	t.Run("SaveMLModelConfig - Failed (Error fetching Digital Twin devices)", func(t *testing.T) {
		testDigitalTwinUrl := "http://localhost:48090"

		mockedHttpClient := svs_mocks.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(nil, testHedgeError)

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("SaveMLModelConfig", mock.Anything).Return("TestConfig", nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
			mlMgmtConfig: &config.MLMgmtConfig{
				DigitalTwinUrl: testDigitalTwinUrl,
			},
			httpClient: &mockedHttpClient,
		}

		testMlModelConfig := buildTestMLModelConfig(
			helpers.REGRESSION_ALGO_TYPE,
			testFeatures,
			map[string]int{},
			[]string{},
		)
		testMlModelConfig.MLDataSourceConfig.EntityType = config.ENTITY_TYPE_DIGITAL_TWIN
		testMlModelConfig.MLDataSourceConfig.EntityName = "InvalidTwin"

		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			helpers.REGRESSION_ALGO_TYPE,
			49000,
			false,
		)

		err := cfgSvc.SaveMLModelConfig(testMlModelConfig, testMlAlgorithmDefinition)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Error getting digital twin devices for InvalidTwin")
	})
	t.Run("SaveMLModelConfig - Failed (Error saving ML model config to DB)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("SaveMLModelConfig", mock.Anything).Return("", testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		testMlModelConfig := buildTestMLModelConfig(
			helpers.CLASSIFICATION_ALGO_TYPE,
			testFeatures,
			map[string]int{},
			[]string{},
		)
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			helpers.CLASSIFICATION_ALGO_TYPE,
			49000,
			false,
		)

		err := cfgSvc.SaveMLModelConfig(testMlModelConfig, testMlAlgorithmDefinition)
		require.Error(t, err)
		assert.Contains(t, err.Error(), errTest.Error())
	})
}

func TestMLModelConfigService_DeleteMLModelConfig(t *testing.T) {
	t.Run("DeleteMLModelConfig - Passed", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{}, nil)
		mockedDBClient.On("DeleteMLModelConfig", mock.Anything, mock.Anything).Return(nil)
		mockedDBClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).
			Return([]config.MLEventConfig{}, nil)
		mockedDBClient.On("DeleteMLModelsByConfig", mock.Anything, mock.Anything).Return(nil)
		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("TestAlgorithm", "TestConfig")
		require.NoError(t, err)
	})
	t.Run("DeleteMLModelConfig - Failed (Model not undeployed)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{
				{
					DeploymentStatusCode: ml_model.ModelDeployed,
				},
			}, nil)
		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("TestAlgorithm", "TestConfig")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "There is a trained model in the deployed status")
	})
	t.Run("DeleteMLModelConfig - Failed (Error getting deployments)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return(nil, testHedgeError)
		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("TestAlgorithm", "TestConfig")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed deleting ML model config: TestConfig")
	})
	t.Run("DeleteMLModelConfig - Failed (Error deleting ML model config)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{}, nil)
		mockedDBClient.On("DeleteMLModelConfig", mock.Anything, mock.Anything).
			Return(testHedgeError)
		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("TestAlgorithm", "TestConfig")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed deleting ML model config: TestConfig")
	})
	t.Run("DeleteMLModelConfig - Failed (Error getting ML event configs)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{}, nil)
		mockedDBClient.On("DeleteMLModelConfig", mock.Anything, mock.Anything).Return(nil)
		mockedDBClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).
			Return(nil, testHedgeError)
		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("TestAlgorithm", "TestConfig")
		require.Error(t, err)
		assert.Contains(
			t,
			err.Error(),
			"Failed getting all existing ML event configs for the ML model config: TestConfig",
		)
	})
	t.Run("DeleteMLModelConfig - Failed (Error Deleting Event Config)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{}, nil)
		mockedDBClient.On("DeleteMLModelConfig", mock.Anything, mock.Anything).Return(nil)
		mockedDBClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).
			Return([]config.MLEventConfig{
				{EventName: "TestEvent"},
			}, nil)
		mockedDBClient.On("DeleteMLEventConfigByName", mock.Anything, mock.Anything, mock.Anything).
			Return(testHedgeError)
		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("TestAlgorithm", "TestConfig")
		require.Error(t, err)
		assert.Contains(
			t,
			err.Error(),
			"Failed deleting existing ML event config TestEvent for ML model config: TestConfig",
		)
	})
	t.Run("DeleteMLModelConfig - Failed (Error Deleting ML Models)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetDeploymentsByConfig", mock.Anything, mock.Anything).
			Return([]ml_model.ModelDeploymentStatus{}, nil)
		mockedDBClient.On("DeleteMLModelConfig", mock.Anything, mock.Anything).Return(nil)
		mockedDBClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).
			Return([]config.MLEventConfig{}, nil)
		mockedDBClient.On("DeleteMLModelsByConfig", mock.Anything, mock.Anything).
			Return(testHedgeError)
		mlMgmtConfig := config.NewMLMgmtConfig()
		mlMgmtConfig.BaseTrainingDataLocalDir = "tmp"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			dbClient:     &mockedDBClient,
			mlMgmtConfig: mlMgmtConfig,
		}

		err := cfgSvc.DeleteMLModelConfig("TestAlgorithm", "TestConfig")
		require.Error(t, err)
		assert.Contains(
			t,
			err.Error(),
			"Failed deleting all undeployed trained models associated with the ML model config: TestConfig",
		)
	})
}

func TestMLModelConfigService_validateAlgoAndSetDefaults(t *testing.T) {
	t.Run("validateAlgoAndSetDefaults - Passed (Classification algo)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			"TestClassificationAlgo",
			helpers.CLASSIFICATION_ALGO_TYPE,
			49000,
			false,
		)

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllAlgorithms").
			Return([]*config.MLAlgorithmDefinition{testMlAlgorithmDefinition}, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.validateAlgoAndSetDefaults(testMlAlgorithmDefinition)
		assert.NoError(t, err, "validateAlgoAndSetDefaults() unexpected error")

		assert.True(
			t,
			testMlAlgorithmDefinition.OutputFeaturesPredefined,
			"OutputFeaturesPredefined should be true for classification",
		)
		assert.True(
			t,
			testMlAlgorithmDefinition.RequiresDataPipeline,
			"RequiresDataPipeline should be true for edge deployment",
		)
		assert.NotEmpty(
			t,
			testMlAlgorithmDefinition.DefaultPredictionEndpointURL,
			"DefaultPredictionEndpointURL should be set",
		)
	})
	t.Run("validateAlgoAndSetDefaults - Passed (Timeseries algo)", func(t *testing.T) {
		testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
			Name:                "TestTimeseriesAlgo",
			Type:                helpers.TIMESERIES_ALGO_TYPE,
			PredictionImagePath: "timeseries-predictor-image",
			TrainerImagePath:    "timeseries-trainer-image",
		}
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllAlgorithms").
			Return([]*config.MLAlgorithmDefinition{testMlAlgorithmDefinition}, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.validateAlgoAndSetDefaults(testMlAlgorithmDefinition)
		assert.NoError(t, err, "validateAlgoAndSetDefaults() unexpected error")

		assert.True(
			t,
			testMlAlgorithmDefinition.GroupByAttributesRequired,
			"GroupByAttributesRequired should be true for timeseries",
		)
		assert.True(
			t,
			testMlAlgorithmDefinition.TimeSeriesAttributeRequired,
			"TimeSeriesAttributeRequired should be true for timeseries",
		)
	})
	t.Run("validateAlgoAndSetDefaults - Passed (Default port allocation)", func(t *testing.T) {
		testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
			Name:                "DefaultPortAlgo",
			PredictionImagePath: "predictor-image",
			TrainerImagePath:    "trainer-image",
		}
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllAlgorithms").
			Return([]*config.MLAlgorithmDefinition{testMlAlgorithmDefinition}, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.validateAlgoAndSetDefaults(testMlAlgorithmDefinition)
		assert.NoError(t, err, "validateAlgoAndSetDefaults() unexpected error")
		assert.Greater(
			t,
			testMlAlgorithmDefinition.DefaultPredictionPort,
			int64(0),
			"DefaultPredictionPort should be allocated",
		)
	})
	t.Run("validateAlgoAndSetDefaults - Passed (Autofix prediction URL)", func(t *testing.T) {
		testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
			Name:                         "FixPredictionURLAlgo",
			PredictionImagePath:          "predictor-image",
			TrainerImagePath:             "trainer-image",
			DefaultPredictionEndpointURL: "http://localhost:49000/api/v3/predict",
		}
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAllAlgorithms").
			Return([]*config.MLAlgorithmDefinition{testMlAlgorithmDefinition}, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.validateAlgoAndSetDefaults(testMlAlgorithmDefinition)
		assert.NoError(t, err, "validateAlgoAndSetDefaults() unexpected error")
		assert.Equal(
			t,
			"/api/v3/predict",
			testMlAlgorithmDefinition.DefaultPredictionEndpointURL,
			"Prediction URL should be fixed",
		)
	})
	t.Run(
		"validateAlgoAndSetDefaults - Failed (Missing prediction image path)",
		func(t *testing.T) {
			testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
				Name: "TestAlgoWithoutPredictionImage",
			}
			mockedDBClient := redisMock.MockMLDbInterface{}

			cfgSvc := &MLModelConfigService{
				service:  u.AppService,
				dbClient: &mockedDBClient,
			}

			err := cfgSvc.validateAlgoAndSetDefaults(testMlAlgorithmDefinition)
			assert.Error(t, err, "validateAlgoAndSetDefaults() expected error, got nil")
			assert.Contains(t, err.Error(), "Prediction Image name is required")
		},
	)
	t.Run(
		"validateAlgoAndSetDefaults - Failed (Missing trainer image path for internal training)",
		func(t *testing.T) {
			testMlAlgorithmDefinition := &config.MLAlgorithmDefinition{
				Name:                "TestAlgoWithoutTrainerImage",
				PredictionImagePath: "some-predictor-image",
				IsTrainingExternal:  false,
			}
			mockedDBClient := redisMock.MockMLDbInterface{}

			cfgSvc := &MLModelConfigService{
				service:  u.AppService,
				dbClient: &mockedDBClient,
			}

			err := cfgSvc.validateAlgoAndSetDefaults(testMlAlgorithmDefinition)
			assert.Error(t, err, "validateAlgoAndSetDefaults() expected error, got nil")
			assert.Contains(t, err.Error(), "Trainer Image name is required")
		},
	)
}

func TestMLModelConfigService_SetHttpClient(t *testing.T) {
	t.Run("SetHttpClient - Passed", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{}

		if cfgSvc.httpClient != nil {
			t.Fatalf("Expected httpClient to be nil, but got %v", cfgSvc.httpClient)
		}
		cfgSvc.SetHttpClient()
		if cfgSvc.httpClient == nil {
			t.Errorf("Expected httpClient to be set, but got nil")
		}
	})
}

func TestMLModelConfigService_GetRegistryConfig(t *testing.T) {
	t.Run("GetRegistryConfig - Passed", func(t *testing.T) {
		mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
		cfgSvc := &MLModelConfigService{
			registryConfig: mockRegistryConfig,
		}

		registryConfig := cfgSvc.GetRegistryConfig()
		require.NotNil(t, registryConfig)
		assert.Equal(t, mockRegistryConfig, registryConfig)
	})
}

func TestMLModelConfigService_ValidateMLModelConfigFilters(t *testing.T) {
	t.Run("ValidateMLModelConfigFilters - Passed (No duplicates)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{}
		testFilters := []config.FilterDefinition{
			{Label: "label1", Operator: "CONTAINS", Value: "value1"},
			{Label: "label2", Operator: "EXCLUDES", Value: "value2"},
		}

		err := cfgSvc.ValidateMLModelConfigFilters(testFilters)
		if err != nil {
			t.Errorf("ValidateMLModelConfigFilters() unexpected error: %v", err)
		}
	})
	t.Run(
		"ValidateMLModelConfigFilters - Passed (Valid CONTAINS and EXCLUDES combination)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{}
			testFilters := []config.FilterDefinition{
				{Label: "label1", Operator: "CONTAINS", Value: "value1"},
				{Label: "label1", Operator: "EXCLUDES", Value: "value2"},
			}

			err := cfgSvc.ValidateMLModelConfigFilters(testFilters)
			if err != nil {
				t.Errorf("ValidateMLModelConfigFilters() unexpected error: %v", err)
			}
		},
	)
	t.Run("ValidateMLModelConfigFilters - Passed (Empty filters)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{}
		var testFilters []config.FilterDefinition

		err := cfgSvc.ValidateMLModelConfigFilters(testFilters)
		if err != nil {
			t.Errorf("ValidateMLModelConfigFilters() unexpected error: %v", err)
		}
	})
	t.Run("ValidateMLModelConfigFilters - Failed (With invalid duplicates)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{}
		testFilters := []config.FilterDefinition{
			{Label: "label1", Operator: "CONTAINS", Value: "value1"},
			{Label: "label1", Operator: "CONTAINS", Value: "value2"},
		}

		err := cfgSvc.ValidateMLModelConfigFilters(testFilters)
		if err == nil {
			t.Errorf("ValidateMLModelConfigFilters() expected error, got nil")
		}
	})
	t.Run(
		"ValidateMLModelConfigFilters - Failed (With non-CONTAINS/EXCLUDES combination)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{}
			testFilters := []config.FilterDefinition{
				{Label: "label1", Operator: "EQUALS", Value: "value1"},
				{Label: "label1", Operator: "EXCLUDES", Value: "value2"},
			}

			err := cfgSvc.ValidateMLModelConfigFilters(testFilters)
			if err == nil {
				t.Errorf("ValidateMLModelConfigFilters() expected error, got nil")
			}
		},
	)
}

func TestMLModelConfigService_BuildFeatureNameToIndexMap(t *testing.T) {
	t.Run(
		"BuildFeatureNameToIndexMap - Passed (No features and default keys added)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.ANOMALY_ALGO_TYPE,
				map[string][]config.Feature{},
				nil,
				[]string{},
			)

			cfgSvc.BuildFeatureNameToIndexMap(testMlModelConfig, true, true)

			expectedKeys := []string{"Timestamp", "deviceName"}
			for _, key := range expectedKeys {
				_, exists := testMlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex[key]
				assert.True(t, exists, "Expected key %s not found in FeatureNameToColumnIndex", key)
			}
		},
	)
	t.Run("BuildFeatureNameToIndexMap - Passed (Features Added from Profiles)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{}
		testFeaturesByProfile := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
				{Name: "Feature2", IsInput: true, IsOutput: true, FromExternalDataSource: false},
			},
		}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.REGRESSION_ALGO_TYPE,
			testFeaturesByProfile,
			nil,
			[]string{"deviceName"},
		)

		cfgSvc.BuildFeatureNameToIndexMap(testMlModelConfig, false, true)

		expectedFeatures := []string{
			"Profile1#Feature1", "Profile1#Feature2", "deviceName",
		}
		for _, feature := range expectedFeatures {
			_, exists := testMlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex[feature]
			assert.True(
				t,
				exists,
				"Expected feature %s not found in FeatureNameToColumnIndex",
				feature,
			)
		}
	})
	t.Run(
		"BuildFeatureNameToIndexMap - Passed (TimeSeries attribute included)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{}
			testFeaturesByProfile := buildTestMLModelConfig(
				helpers.ANOMALY_ALGO_TYPE,
				map[string][]config.Feature{},
				nil,
				[]string{},
			)

			cfgSvc.BuildFeatureNameToIndexMap(testFeaturesByProfile, true, false)

			_, exists := testFeaturesByProfile.MLDataSourceConfig.FeatureNameToColumnIndex["Timestamp"]
			assert.True(
				t,
				exists,
				"Expected 'Timestamp' feature not found in FeatureNameToColumnIndex",
			)
		},
	)
	t.Run("BuildFeatureNameToIndexMap - Passed (External feature appended)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{}
		testFeaturesByProfile := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
				{Name: "Feature2", IsInput: false, IsOutput: false, FromExternalDataSource: true},
			},
		}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.ANOMALY_ALGO_TYPE,
			testFeaturesByProfile,
			nil,
			[]string{},
		)

		cfgSvc.BuildFeatureNameToIndexMap(testMlModelConfig, false, false)

		assert.Equal(
			t,
			2,
			len(testMlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex),
			"Unexpected number of features",
		)
		assert.Equal(
			t,
			0,
			testMlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex["Profile1#Feature1"],
		)
		assert.Equal(
			t,
			1,
			testMlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex["Profile1#Feature2"],
		)
	})
	t.Run(
		"BuildFeatureNameToIndexMap - Passed (GroupOrJoinKeys default added)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.ANOMALY_ALGO_TYPE,
				map[string][]config.Feature{},
				nil,
				[]string{},
			)

			cfgSvc.BuildFeatureNameToIndexMap(testMlModelConfig, false, true)

			_, exists := testMlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex["deviceName"]
			assert.True(
				t,
				exists,
				"Expected 'deviceName' in GroupOrJoinKeys not found in FeatureNameToColumnIndex",
			)
		},
	)
}

func TestMLModelConfigService_ChangeAlgorithmStatus(t *testing.T) {
	t.Run("ChangeAlgorithmStatus - Passed", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)
		mockedDBClient.On("UpdateAlgorithm", mock.Anything).Return(nil)

		cfgSvc := &MLModelConfigService{
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.ChangeAlgorithmStatus("mock_algo", true)
		require.NoError(t, err)
	})
	t.Run("ChangeAlgorithmStatus - Failed (GetAlgorithm failed)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.ChangeAlgorithmStatus(testMlAlgorithmName, true)
		require.Error(t, err)
		assert.EqualError(t, err, "Failed to enable algorithm TestAlgo")
	})
	t.Run("ChangeAlgorithmStatus - Failed (No algorithm found)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(nil, nil)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.ChangeAlgorithmStatus(testMlAlgorithmName, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Failed to enable non-existing algorithm TestAlgo")
	})
	t.Run("ChangeAlgorithmStatus - Failed (UpdateAlgorithm failed)", func(t *testing.T) {
		testMlAlgorithmDefinition := buildTestMlAlgoDefinition(
			testMlAlgorithmName,
			"TestType",
			49000,
			false,
		)
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("GetAlgorithm", mock.Anything).Return(testMlAlgorithmDefinition, nil)
		mockedDBClient.On("UpdateAlgorithm", mock.Anything).Return(testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:  u.AppService,
			dbClient: &mockedDBClient,
		}

		err := cfgSvc.ChangeAlgorithmStatus(testMlAlgorithmName, true)
		require.Error(t, err)
		assert.EqualError(t, err, "Failed to enable algorithm TestAlgo")
	})
}

func TestMLModelConfigService_GetDigitalTwinDevices(t *testing.T) {
	t.Run("GetDigitalTwinDevices - Passed (Devices retrieved)", func(t *testing.T) {
		testDigitalTwinName := "WindTurbineFarm"
		testDigitalTwinUrl := "http://localhost:48090"
		expectedDevices := []string{"Device1", "Device2"}

		testResponseBody, _ := json.Marshal(expectedDevices)
		mockedHttpClient := svs_mocks.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(testResponseBody)),
		}, nil)

		cfgSvc := &MLModelConfigService{
			service:    u.AppService,
			httpClient: &mockedHttpClient,
			mlMgmtConfig: &config.MLMgmtConfig{
				DigitalTwinUrl: testDigitalTwinUrl,
			},
		}

		devices, err := cfgSvc.getDigitalTwinDevices(testDigitalTwinName)
		require.NoError(t, err)
		assert.Equal(t, expectedDevices, devices)
	})
	t.Run("GetDigitalTwinDevices - Failed (Missing digitalTwinUrl config)", func(t *testing.T) {
		testDigitalTwinName := "WindTurbineFarm"

		cfgSvc := &MLModelConfigService{
			service:      u.AppService,
			mlMgmtConfig: &config.MLMgmtConfig{},
		}

		devices, err := cfgSvc.getDigitalTwinDevices(testDigitalTwinName)
		require.Error(t, err)
		assert.Nil(t, devices)
		assert.Contains(t, err.Error(), "missing DigitalTwinUrl configuration")
	})
	t.Run("GetDigitalTwinDevices - Failed (HTTP request failure)", func(t *testing.T) {
		testDigitalTwinName := "WindTurbineFarm"
		testDigitalTwinUrl := "http://localhost:48090"

		mockedHttpClient := svs_mocks.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(nil, errTest)

		cfgSvc := &MLModelConfigService{
			service: u.AppService,
			mlMgmtConfig: &config.MLMgmtConfig{
				DigitalTwinUrl: testDigitalTwinUrl,
			},
			httpClient: &mockedHttpClient,
		}

		devices, err := cfgSvc.getDigitalTwinDevices(testDigitalTwinName)
		require.Error(t, err)
		assert.Nil(t, devices)
		assert.Contains(t, err.Error(), "Error getting digital twin devices for WindTurbineFarm")
	})
	t.Run("GetDigitalTwinDevices - Failed (Non-200 Status Code)", func(t *testing.T) {
		testDigitalTwinName := "WindTurbineFarm"
		testDigitalTwinUrl := "http://localhost:48090"

		mockedHttpClient := svs_mocks.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("")),
		}, errors.New("dummy 500"))

		cfgSvc := &MLModelConfigService{
			service: u.AppService,
			mlMgmtConfig: &config.MLMgmtConfig{
				DigitalTwinUrl: testDigitalTwinUrl,
			},
			httpClient: &mockedHttpClient,
		}

		devices, err := cfgSvc.getDigitalTwinDevices(testDigitalTwinName)
		require.Error(t, err)
		assert.Nil(t, devices)
		assert.Equal(t, err.ErrorType(), hedgeErrors.ErrorTypeServerError)
		assert.Contains(t, err.Error(), "Error getting digital twin devices for WindTurbineFarm")
	})
	t.Run("GetDigitalTwinDevices - Failed (Response body unreadable)", func(t *testing.T) {
		testDigitalTwinName := "WindTurbineFarm"
		testDigitalTwinUrl := "http://localhost:48090"

		mockedHttpClient := svs_mocks.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(FaultyReader{}),
		}, nil)

		cfgSvc := &MLModelConfigService{
			service: u.AppService,
			mlMgmtConfig: &config.MLMgmtConfig{
				DigitalTwinUrl: testDigitalTwinUrl,
			},
			httpClient: &mockedHttpClient,
		}

		devices, err := cfgSvc.getDigitalTwinDevices(testDigitalTwinName)
		require.Error(t, err)
		assert.Nil(t, devices)
		assert.Contains(t, err.Error(), "Error getting digital twin devices for WindTurbineFarm")
	})
	t.Run("GetDigitalTwinDevices - Failed (JSON unmarshal failure)", func(t *testing.T) {
		testDigitalTwinName := "WindTurbineFarm"
		testDigitalTwinUrl := "http://localhost:48090"

		mockedHttpClient := svs_mocks.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("invalid json")),
		}, nil)

		cfgSvc := &MLModelConfigService{
			service: u.AppService,
			mlMgmtConfig: &config.MLMgmtConfig{
				DigitalTwinUrl: testDigitalTwinUrl,
			},
			httpClient: &mockedHttpClient,
		}

		devices, err := cfgSvc.getDigitalTwinDevices(testDigitalTwinName)
		require.Error(t, err)
		assert.Nil(t, devices)
		assert.Contains(t, err.Error(), "Error getting digital twin devices for WindTurbineFarm")
	})
}

func TestMLModelConfigService_ValidateFeaturesToProfileByAlgoType(t *testing.T) {
	// Anomaly
	t.Run("ValidateFeaturesToProfileByAlgoType - Passed (Anomaly)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{
			service: u.AppService,
		}
		testFeatures := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
				{Name: "Feature2", IsInput: true, IsOutput: false, FromExternalDataSource: false},
			},
		}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.ANOMALY_ALGO_TYPE,
			testFeatures,
			nil,
			[]string{"deviceName"},
		)
		err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, false)
		require.NoError(t, err)
	})
	t.Run(
		"ValidateFeaturesToProfileByAlgoType - Failed (Feature with isOutput=true for Anomaly)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeatures := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: true, FromExternalDataSource: false},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.ANOMALY_ALGO_TYPE,
				testFeatures,
				nil,
				[]string{"deviceName"},
			)
			err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, false)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: feature 'Feature1' in profile 'Profile1' must be isInput=true and isOutput=false for Anomaly algorithm type",
			)
		},
	)
	// Classification
	t.Run("ValidateFeaturesToProfileByAlgoType - Passed (Classification)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{
			service: u.AppService,
		}
		testFeatures := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: false},
				{Name: "Feature2", IsInput: false, IsOutput: false},
			},
		}

		testFeatureIndexMap := map[string]int{"Profile1#Feature1": 0, "Profile1#Feature2": 1}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.CLASSIFICATION_ALGO_TYPE,
			testFeatures,
			testFeatureIndexMap,
			[]string{"deviceName"},
		)
		testMlModelConfig = addExternalFeature(testMlModelConfig)
		err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, true)
		require.NoError(t, err)
	})
	t.Run(
		"ValidateFeaturesToProfileByAlgoType - Failed (Multiple Output Features for Classification)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeatures := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: false, IsOutput: true},
					{Name: "Feature2", IsInput: true, IsOutput: true},
				},
			}
			testFeatureIndexMap := map[string]int{"Profile1#Feature1": 1, "Profile1#Feature2": 0}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.CLASSIFICATION_ALGO_TYPE,
				testFeatures,
				testFeatureIndexMap,
				[]string{"deviceName"},
			)
			err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, false)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: multiple output features found, when only 1 output feature is allowed for classification algorithm type as of now",
			)
		},
	)
	// Timeseries
	t.Run("ValidateFeaturesToProfileByAlgoType - Passed (Timeseries)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{
			service: u.AppService,
		}
		testFeatures := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: true, FromExternalDataSource: false},
				{Name: "Feature2", IsInput: true, IsOutput: true, FromExternalDataSource: false},
			},
		}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.TIMESERIES_ALGO_TYPE,
			testFeatures,
			nil,
			[]string{"deviceName"},
		)
		err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, true)
		require.NoError(t, err)
	})

	/*
		t.Run(
			"ValidateFeaturesToProfileByAlgoType - Failed (Invalid Configuration for Timeseries)",
			func(t *testing.T) {
				cfgSvc := &MLModelConfigService{
					service: u.AppService,
				}
				testFeatures := map[string][]config.Feature{
					"Profile1": {
						{Name: "Feature1", IsInput: false, IsOutput: true},
						{Name: "Feature2", IsInput: true, IsOutput: true},
					},
				}
				testMlModelConfig := buildTestMLModelConfig(
					helpers.TIMESERIES_ALGO_TYPE,
					testFeatures,
					nil,
					[]string{"deviceName"},
				)
				err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, false)
				require.Error(t, err)
				assert.EqualError(
					t,
					err,
					"ML model config validation failed: feature 'Feature1' in profile 'Profile1' must be isInput=true and isExternal=false for Timeseries algorithm type",
				)
			},
		)*/
	// Regression
	t.Run("ValidateFeaturesToProfileByAlgoType - Passed (Regression)", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{
			service: u.AppService,
		}
		testFeatures := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: false},
				{Name: "Feature2", IsInput: false, IsOutput: true},
			},
		}
		testFeatureIndexMap := map[string]int{"Profile1#Feature1": 0, "Profile1#Feature2": 1}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.REGRESSION_ALGO_TYPE,
			testFeatures,
			testFeatureIndexMap,
			[]string{"deviceName"},
		)
		err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, false)
		require.NoError(t, err)
	})
	t.Run(
		"ValidateFeaturesToProfileByAlgoType - Failed (Multiple Output Features for Regression)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeatures := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: false},
					{Name: "Feature2", IsInput: false, IsOutput: true},
					{Name: "Feature3", IsInput: false, IsOutput: true},
				},
			}
			testFeatureIndexMap := map[string]int{"Profile1#Feature1": 1, "Profile1#Feature2": 0}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.REGRESSION_ALGO_TYPE,
				testFeatures,
				testFeatureIndexMap,
				[]string{"deviceName"},
			)
			err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, false)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: only one output feature configuration required, found: 2",
			)
		},
	)
	t.Run(
		"ValidateFeaturesToProfileByAlgoType - Failed (Invalid GroupOrJoinKeys for 1-profile)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeatures := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: true},
					{Name: "Feature2", IsInput: true, IsOutput: true},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.TIMESERIES_ALGO_TYPE,
				testFeatures,
				nil,
				[]string{"someOtherKey"},
			)
			err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, true)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: for 1 profile scenario there must be only 1 element of 'groupOrJoinKeys' - 'deviceName'",
			)
		},
	)
	t.Run(
		"ValidateFeaturesToProfileByAlgoType - Failed (Unsupported MLAlgorithmType)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeatures := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				"UNSUPPORTED_TYPE",
				testFeatures,
				nil,
				[]string{"deviceName"},
			)
			err := cfgSvc.ValidateFeaturesToProfileByAlgoType(testMlModelConfig, false)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: unsupported MLAlgorithmType: UNSUPPORTED_TYPE",
			)
		},
	)
}

func TestMLModelConfigService_applyAnomalyAlgoTypeValidations(t *testing.T) {
	t.Run("applyAnomalyAlgoTypeValidations - Passed", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{
			service: u.AppService,
		}
		testFeaturesValid := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: false},
				{Name: "Feature2", IsInput: true, IsOutput: false},
			},
		}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.ANOMALY_ALGO_TYPE,
			testFeaturesValid,
			nil,
			[]string{},
		)
		err := cfgSvc.applyAnomalyAlgoTypeValidations(testMlModelConfig)
		require.NoError(t, err)
	})
	t.Run(
		"applyAnomalyAlgoTypeValidations - Failed (Feature with isOutput=true)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesWithOutput := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: true},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.ANOMALY_ALGO_TYPE,
				testFeaturesWithOutput,
				nil,
				[]string{},
			)
			err := cfgSvc.applyAnomalyAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: feature 'Feature1' in profile 'Profile1' must be isInput=true and isOutput=false for Anomaly algorithm type",
			)
		},
	)
	t.Run(
		"applyAnomalyAlgoTypeValidations - Failed (Feature with isInput=false)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesWithInvalidInput := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature2", IsInput: false, IsOutput: false},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.ANOMALY_ALGO_TYPE,
				testFeaturesWithInvalidInput,
				nil,
				[]string{},
			)
			err := cfgSvc.applyAnomalyAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: feature 'Feature2' in profile 'Profile1' must be isInput=true and isOutput=false for Anomaly algorithm type",
			)
		},
	)
	/* No longer applicable since we don't differentiate if the input or output is external or not
	t.Run(
		"applyAnomalyAlgoTypeValidations - Failed (Feature with isExternal=true)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesWithExternal := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature3", IsInput: true, IsOutput: false},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.ANOMALY_ALGO_TYPE,
				testFeaturesWithExternal,
				nil,
				[]string{},
			)
			testMlModelConfig = addExternalFeature(testMlModelConfig)
			err := cfgSvc.applyAnomalyAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: feature 'Feature3' in profile 'Profile1' must be isInput=true and isOutput=false for Anomaly algorithm type",
			)
		},
	)*/
}

func TestMLModelConfigService_applyClassificationAlgoTypeValidations(t *testing.T) {
	t.Run("applyClassificationAlgoTypeValidations - Passed", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{
			service: u.AppService,
		}
		testFeaturesValid := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: false},
				{Name: "Feature2", IsInput: false, IsOutput: true},
			},
		}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.CLASSIFICATION_ALGO_TYPE,
			testFeaturesValid,
			map[string]int{"Profile1#Feature1": 0, "Profile1#Feature2": 1},
			[]string{},
		)
		err := cfgSvc.applyClassificationAlgoTypeValidations(testMlModelConfig)
		require.NoError(t, err)
	})
	t.Run(
		"applyClassificationAlgoTypeValidations - Failed (Multiple Output features)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesMultipleOutputExternalNotInput := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: false, IsOutput: true},
					{Name: "Feature2", IsInput: false, IsOutput: true},
					{Name: "Feature3", IsInput: false, IsOutput: true},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.CLASSIFICATION_ALGO_TYPE,
				testFeaturesMultipleOutputExternalNotInput,
				map[string]int{"Profile1#Feature1": 1, "Profile1#Feature2": 0},
				[]string{},
			)
			err := cfgSvc.applyClassificationAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: multiple output features found, when only 1 output feature is allowed for classification algorithm type as of now",
			)
		},
	)
	t.Run(
		"applyClassificationAlgoTypeValidations - Failed (Multiple Output features)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesMultipleOutput := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: false, IsOutput: true},
					{Name: "Feature2", IsInput: true, IsOutput: true},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.CLASSIFICATION_ALGO_TYPE,
				testFeaturesMultipleOutput,
				map[string]int{"Profile1#Feature1": 1, "Profile1#Feature2": 0},
				[]string{},
			)
			err := cfgSvc.applyClassificationAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: multiple output features found, when only 1 output feature is allowed for classification algorithm type as of now",
			)
		},
	)
	t.Run(
		"applyClassificationAlgoTypeValidations - Failed (Multiple External features)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesMultipleExternal := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: false},
					{Name: "Feature2", IsInput: true, IsOutput: true},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.CLASSIFICATION_ALGO_TYPE,
				testFeaturesMultipleExternal,
				map[string]int{"Profile1#Feature1": 0, "Profile1#Feature2": 1},
				[]string{},
			)
			testMlModelConfig = addExternalFeature(testMlModelConfig)
			err := cfgSvc.applyClassificationAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: multiple output features found, when only 1 output feature is allowed for classification algorithm type as of now",
			)
		},
	)
	t.Run(
		"applyClassificationAlgoTypeValidations - Failed (No Output External feature found)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesNoOutput := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: false},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.CLASSIFICATION_ALGO_TYPE,
				testFeaturesNoOutput,
				map[string]int{"Profile1#Feature1": 0},
				[]string{},
			)
			err := cfgSvc.applyClassificationAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: no output feature defined for classification",
			)
		},
	)
}

func TestMLModelConfigService_applyTimeseriesAlgoTypeValidations(t *testing.T) {
	t.Run("applyTimeseriesAlgoTypeValidations - Passed", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{
			service: u.AppService,
		}
		testFeaturesValid := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: true},
				{Name: "Feature2", IsInput: true, IsOutput: true},
			},
		}
		testGroupOrJoinKeysValid := []string{"deviceName"}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.TIMESERIES_ALGO_TYPE,
			testFeaturesValid,
			nil,
			testGroupOrJoinKeysValid,
		)

		err := cfgSvc.applyTimeseriesAlgoTypeValidations(testMlModelConfig, true)
		require.NoError(t, err)
		for profileName, features := range testMlModelConfig.MLDataSourceConfig.FeaturesByProfile {
			for _, feature := range features {
				assert.True(
					t,
					feature.IsOutput,
					"Feature '%s' in profile '%s' should have isOutput=true after adjustment",
					feature.Name,
					profileName,
				)
			}
		}
	})
	t.Run(
		"applyTimeseriesAlgoTypeValidations - Failed (Input sequence needed)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesValid := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: true, FromExternalDataSource: false},
					{Name: "Feature2", IsInput: true, IsOutput: true, FromExternalDataSource: false},
				},
			}
			testGroupOrJoinKeysValid := []string{"deviceName"}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.TIMESERIES_ALGO_TYPE,
				testFeaturesValid,
				nil,
				testGroupOrJoinKeysValid,
			)
			testMlModelConfig.MLDataSourceConfig.InputContextCount = 0

			err := cfgSvc.applyTimeseriesAlgoTypeValidations(testMlModelConfig, true)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: Input sequence length for timeseries prediction needs to be greater than 0",
			)
		},
	)
	t.Run(
		"applyTimeseriesAlgoTypeValidations - Passed (Adjust isOutput for Feature1)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesWithOutputAdjustment := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
					{Name: "Feature2", IsInput: true, IsOutput: false, FromExternalDataSource: false},
				},
			}
			testGroupOrJoinKeysValid := []string{"deviceName"}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.TIMESERIES_ALGO_TYPE,
				testFeaturesWithOutputAdjustment,
				nil,
				testGroupOrJoinKeysValid,
			)

			err := cfgSvc.applyTimeseriesAlgoTypeValidations(testMlModelConfig, true)
			require.NoError(t, err)
			for profileName, features := range testMlModelConfig.MLDataSourceConfig.FeaturesByProfile {
				for _, feature := range features {
					assert.True(
						t,
						feature.IsOutput,
						"Feature '%s' in profile '%s' should have isOutput=true after adjustment",
						feature.Name,
						profileName,
					)
				}
			}
		},
	)

	/* The below test case no longer applicable since it uses isExternal specific handling
	t.Run(
		"applyTimeseriesAlgoTypeValidations - Failed (Invalid configuration: Feature1 isInput=false)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesWithInvalidInput := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: false, IsOutput: true, IsExternal: false},
					{Name: "Feature2", IsInput: true, IsOutput: true, IsExternal: false},
				},
			}
			testGroupOrJoinKeysValid := []string{"deviceName"}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.TIMESERIES_ALGO_TYPE,
				testFeaturesWithInvalidInput,
				nil,
				testGroupOrJoinKeysValid,
			)

			err := cfgSvc.applyTimeseriesAlgoTypeValidations(testMlModelConfig, false)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: feature 'Feature1' in profile 'Profile1' must be isInput=true and isExternal=false for Timeseries algorithm type",
			)
		},
	)

	t.Run(
		"applyTimeseriesAlgoTypeValidations - Failed (Invalid configuration: Feature1 isExternal=true)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesWithExternal := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: true, IsExternal: true},
				},
			}
			testGroupOrJoinKeysValid := []string{"deviceName"}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.TIMESERIES_ALGO_TYPE,
				testFeaturesWithExternal,
				nil,
				testGroupOrJoinKeysValid,
			)

			err := cfgSvc.applyTimeseriesAlgoTypeValidations(testMlModelConfig, true)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: feature 'Feature1' in profile 'Profile1' must be isInput=true and isExternal=false for Timeseries algorithm type",
			)
		},
	)
	*/
}

func TestMLModelConfigService_applyRegressionAlgoTypeValidations(t *testing.T) {
	t.Run("applyRegressionAlgoTypeValidations - Passed", func(t *testing.T) {
		cfgSvc := &MLModelConfigService{
			service: u.AppService,
		}
		testFeaturesValid := map[string][]config.Feature{
			"Profile1": {
				{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
				{Name: "Feature2", IsInput: false, IsOutput: true, FromExternalDataSource: false},
			},
		}
		testMlModelConfig := buildTestMLModelConfig(
			helpers.REGRESSION_ALGO_TYPE,
			testFeaturesValid,
			map[string]int{"Profile1#Feature1": 0, "Profile1#Feature2": 1},
			[]string{},
		)

		err := cfgSvc.applyRegressionAlgoTypeValidations(testMlModelConfig)
		require.NoError(t, err)
	})
	t.Run(
		"applyRegressionAlgoTypeValidations - Failed (Multiple Output features)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesMultipleOutput := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
					{Name: "Feature2", IsInput: false, IsOutput: true, FromExternalDataSource: false},
					{Name: "Feature3", IsInput: false, IsOutput: true, FromExternalDataSource: false},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.REGRESSION_ALGO_TYPE,
				testFeaturesMultipleOutput,
				map[string]int{
					"Profile1#Feature1": 0,
					"Profile1#Feature2": 1,
					"Profile1#Feature3": 2,
				},
				[]string{},
			)

			err := cfgSvc.applyRegressionAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: only one output feature configuration required, found: 2",
			)
		},
	)
	/*
		t.Run(
			"applyRegressionAlgoTypeValidations - Failed (External feature not allowed)",
			func(t *testing.T) {
				cfgSvc := &MLModelConfigService{
					service: u.AppService,
				}
				testFeaturesWithExternal := map[string][]config.Feature{
					"Profile1": {
						{Name: "Feature1", IsInput: true, IsOutput: false, IsExternal: false},
						{Name: "Feature2", IsInput: true, IsOutput: true, IsExternal: true},
					},
				}
				testMlModelConfig := buildTestMLModelConfig(
					helpers.REGRESSION_ALGO_TYPE,
					testFeaturesWithExternal,
					map[string]int{"Profile1#Feature1": 0, "Profile1#Feature2": 1},
					[]string{},
				)

				err := cfgSvc.applyRegressionAlgoTypeValidations(testMlModelConfig)
				require.Error(t, err)
				assert.EqualError(
					t,
					err,
					"ML model config validation failed: feature 'Feature2' in profile 'Profile1' expected to be isInput=true, isOutput=false, and isExternal=false for Regression algorithm type. Or, for predicting regression for this feature you must make it isInput=false, isOutput=true, and isExternal=false",
				)
			},
		)*/
	t.Run(
		"applyRegressionAlgoTypeValidations - Failed (No Output feature found)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesNoOutput := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: false, FromExternalDataSource: false},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.REGRESSION_ALGO_TYPE,
				testFeaturesNoOutput,
				map[string]int{"Profile1#Feature1": 0},
				[]string{},
			)

			err := cfgSvc.applyRegressionAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: only one output feature configuration required, found: 0",
			)
		},
	)
	/* We build the feature to index map in a way that the last index(s) is always an output, might need to add this test for the method that builds this map
	t.Run(
		"applyRegressionAlgoTypeValidations - Failed (Output feature not last index)",
		func(t *testing.T) {
			cfgSvc := &MLModelConfigService{
				service: u.AppService,
			}
			testFeaturesOutputNotLast := map[string][]config.Feature{
				"Profile1": {
					{Name: "Feature1", IsInput: true, IsOutput: false, IsExternal: false},
					{Name: "Feature2", IsInput: false, IsOutput: true, IsExternal: false},
				},
			}
			testMlModelConfig := buildTestMLModelConfig(
				helpers.REGRESSION_ALGO_TYPE,
				testFeaturesOutputNotLast,
				map[string]int{"Profile1#Feature1": 1, "Profile1#Feature2": 0},
				[]string{},
			)

			err := cfgSvc.applyRegressionAlgoTypeValidations(testMlModelConfig)
			require.Error(t, err)
			assert.EqualError(
				t,
				err,
				"ML model config validation failed: output feature 'Profile1#Feature2' must have the last index in FeatureNameToColumnIndex for Regression algorithm type",
			)
		},
	)*/
}

func TestMLModelConfigService_validateMLImagesForCreateAlgoFlow(t *testing.T) {
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Passed (digests fetched with generateDigest==true)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return(testImageDigest, nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, true)
			require.NoError(t, err)
			assert.Equal(t, testImageDigest, testAlgo.TrainerImageDigest)
			assert.Equal(t, testImageDigest, testAlgo.PredictionImageDigest)
		},
	)
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Passed (provided digests validated)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return(testImageDigest, nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    testImageDigest,
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: testImageDigest,
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, false)
			require.NoError(t, err)
			assert.Equal(t, testImageDigest, testAlgo.TrainerImageDigest)
			assert.Equal(t, testImageDigest, testAlgo.PredictionImageDigest)
		},
	)
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Failed (digest not provided with generateDigest==false)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "digest is not provided")
		},
	)
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Failed (trainer image digest not found in registry)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, testHedgeError.Message()))

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    testImageDigest,
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: testImageDigest,
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Trainer image validation failed")
		},
	)
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Failed (provided trainer digest does not match registry digest)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return("different-digest", nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    "provided-digest",
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: "provided-digest",
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "provided digest is not found in image registry")
		},
	)
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Failed (provided prediction digest does not match registry digest)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return("different-digest", nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    testImageDigest,
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: testImageDigest,
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "provided digest is not found in image registry")
		},
	)
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Failed (prediction digest not found in registry, generateDigest==false)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, testHedgeError.Message()))

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    testImageDigest,
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: testImageDigest,
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, false)
			require.Error(t, err)
			assert.Contains(
				t,
				err.Error(),
				"Prediction image validation failed for algo TestAlgo, image 'prediction-image:latest', generateDigest 'false, err: hedge dummy error",
			)
		},
	)
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Failed (digest not found in registry, generateDigest==true)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "image not found"))

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "image not found")
		},
	)
	t.Run(
		"validateMLImagesForCreateAlgoFlow - Failed (provided digest does not match registry digest)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return("different-digest", nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			testAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    testImageDigest,
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: "provided-digest",
			}

			err := cfgSvc.validateMLImagesForCreateAlgoFlow(testAlgo, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "provided digest is not found in image registry")
		},
	)
}

func TestMLModelConfigService_validateMLImagesForUpdateAlgoFlow(t *testing.T) {
	t.Run(
		"validateMLImagesForUpdateAlgoFlow - Passed (existing digests match retrieved digests)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return(testImageDigest, nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return(testImageDigest, nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			existingAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    testImageDigest,
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: testImageDigest,
			}
			updatedAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}

			err := cfgSvc.validateMLImagesForUpdateAlgoFlow(updatedAlgo, existingAlgo, false)
			require.NoError(t, err)
			assert.Equal(t, testImageDigest, updatedAlgo.TrainerImageDigest)
			assert.Equal(t, testImageDigest, updatedAlgo.PredictionImageDigest)
		},
	)
	t.Run(
		"validateMLImagesForUpdateAlgoFlow - Passed (digests fetched when generateDigest==true)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return("new-trainer-digest", nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return("new-prediction-digest", nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			existingAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    "old-trainer-digest",
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: "old-prediction-digest",
			}
			updatedAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}

			err := cfgSvc.validateMLImagesForUpdateAlgoFlow(updatedAlgo, existingAlgo, true)
			require.NoError(t, err)
			assert.Equal(t, "new-trainer-digest", updatedAlgo.TrainerImageDigest)
			assert.Equal(t, "new-prediction-digest", updatedAlgo.PredictionImageDigest)
		},
	)
	t.Run(
		"validateMLImagesForUpdateAlgoFlow - Failed (image path changed with generateDigest==false)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			existingAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}
			updatedAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "new-trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}

			err := cfgSvc.validateMLImagesForUpdateAlgoFlow(updatedAlgo, existingAlgo, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "image path has been changed")
		},
	)
	t.Run(
		"validateMLImagesForUpdateAlgoFlow - Failed (existing digest not in registry)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return("new-trainer-digest", nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return(testImageDigest, nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			existingAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				TrainerImageDigest:    "old-trainer-digest",
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: testImageDigest,
			}
			updatedAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}

			err := cfgSvc.validateMLImagesForUpdateAlgoFlow(updatedAlgo, existingAlgo, false)
			require.Error(t, err)
			assert.Contains(
				t,
				err.Error(),
				"existing digest old-trainer-digest is not existing in image registry anymore",
			)
		},
	)

	t.Run(
		"validateMLImagesForUpdateAlgoFlow - Passed (provided digest already upto date)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return("new-trainer-digest", nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return(testImageDigest, nil)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			existingAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}
			updatedAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				TrainerImageDigest:  "trainer-digest",
				PredictionImagePath: "prediction-image:latest",
			}

			err := cfgSvc.validateMLImagesForUpdateAlgoFlow(updatedAlgo, existingAlgo, true)
			require.NoError(t, err)
		},
	)
	t.Run(
		"validateMLImagesForUpdateAlgoFlow - Failed (failed while get prediction image digest from image registry)",
		func(t *testing.T) {
			mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
			mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
				Return("trainer-image:latest", nil)
			mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
				Return("", testHedgeError)

			cfgSvc := &MLModelConfigService{
				service:        u.AppService,
				registryConfig: mockRegistryConfig,
			}

			existingAlgo := &config.MLAlgorithmDefinition{
				Name:                testMlAlgorithmName,
				TrainerImagePath:    "trainer-image:latest",
				PredictionImagePath: "prediction-image:latest",
			}
			updatedAlgo := &config.MLAlgorithmDefinition{
				Name:                  testMlAlgorithmName,
				TrainerImagePath:      "trainer-image:latest",
				PredictionImagePath:   "prediction-image:latest",
				PredictionImageDigest: "wrong-prediction-digest",
			}

			err := cfgSvc.validateMLImagesForUpdateAlgoFlow(updatedAlgo, existingAlgo, true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), testHedgeError.Message())
		},
	)
}

func TestMLModelConfigService_ValidateMLImagesForGetAlgoFlow(t *testing.T) {
	t.Run("ValidateMLImagesForGetAlgoFlow - Passed (existing digests match retrieved digests)", func(t *testing.T) {
		ctx := u.AppService.AppContext()
		mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").Return(testImageDigest, nil)
		mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").Return(testImageDigest, nil)

		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			registryConfig: mockRegistryConfig,
		}

		testAlgo := &config.MLAlgorithmDefinition{
			Name:                  testMlAlgorithmName,
			TrainerImagePath:      "trainer-image:latest",
			TrainerImageDigest:    testImageDigest,
			PredictionImagePath:   "prediction-image:latest",
			PredictionImageDigest: testImageDigest,
		}

		cfgSvc.ValidateMLImagesForGetAlgoFlow(ctx, testAlgo)
		assert.Equal(t, testImageDigest, testAlgo.TrainerImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.PredictionImageDigest)
	})
	t.Run("ValidateMLImagesForGetAlgoFlow - Passed with updated digests", func(t *testing.T) {
		ctx := u.AppService.AppContext()
		mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").Return("new-trainer-digest", nil)
		mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").Return("new-prediction-digest", nil)

		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			registryConfig: mockRegistryConfig,
		}

		testAlgo := &config.MLAlgorithmDefinition{
			Name:                  testMlAlgorithmName,
			TrainerImagePath:      "trainer-image:latest",
			TrainerImageDigest:    "old-trainer-digest",
			PredictionImagePath:   "prediction-image:latest",
			PredictionImageDigest: "old-prediction-digest",
		}

		cfgSvc.ValidateMLImagesForGetAlgoFlow(ctx, testAlgo)
		assert.Equal(t, "new-trainer-digest", testAlgo.TrainerImageDigest)
		assert.Equal(t, "new-prediction-digest", testAlgo.PredictionImageDigest)
		assert.Equal(t, "old-trainer-digest", testAlgo.DeprecatedTrainerImageDigest)
		assert.Equal(t, "old-prediction-digest", testAlgo.DeprecatedPredictionImageDigest)
	})
	t.Run("ValidateMLImagesForGetAlgoFlow - Trainer image digest retrieval failed on server error", func(t *testing.T) {
		ctx := u.AppService.AppContext()
		mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").Return("", testHedgeError)
		mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").Return(testImageDigest, nil)

		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			registryConfig: mockRegistryConfig,
		}

		testAlgo := &config.MLAlgorithmDefinition{
			Name:                  testMlAlgorithmName,
			TrainerImagePath:      "trainer-image:latest",
			PredictionImagePath:   "prediction-image:latest",
			TrainerImageDigest:    testImageDigest,
			PredictionImageDigest: testImageDigest,
		}

		cfgSvc.ValidateMLImagesForGetAlgoFlow(ctx, testAlgo)
		assert.Equal(t, "image validation failed due to server error", testAlgo.DeprecatedTrainerImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.TrainerImageDigest)
		assert.Equal(t, "", testAlgo.DeprecatedPredictionImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.PredictionImageDigest)
	})
	t.Run("ValidateMLImagesForGetAlgoFlow - Prediction image digest retrieval failed on server error", func(t *testing.T) {
		ctx := u.AppService.AppContext()
		mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").Return(testImageDigest, nil)
		mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").Return("", testHedgeError)

		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			registryConfig: mockRegistryConfig,
		}

		testAlgo := &config.MLAlgorithmDefinition{
			Name:                  testMlAlgorithmName,
			TrainerImagePath:      "trainer-image:latest",
			TrainerImageDigest:    testImageDigest,
			PredictionImagePath:   "prediction-image:latest",
			PredictionImageDigest: testImageDigest,
		}

		cfgSvc.ValidateMLImagesForGetAlgoFlow(ctx, testAlgo)
		assert.Equal(t, "image validation failed due to server error", testAlgo.DeprecatedPredictionImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.PredictionImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.TrainerImageDigest)
		assert.Equal(t, "", testAlgo.DeprecatedTrainerImageDigest)
	})
	t.Run("ValidateMLImagesForGetAlgoFlow - Trainer image digest retrieval failed: image not found in registry", func(t *testing.T) {
		ctx := u.AppService.AppContext()
		mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").
			Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, testHedgeError.Message()))
		mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").Return(testImageDigest, nil)

		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			registryConfig: mockRegistryConfig,
		}

		testAlgo := &config.MLAlgorithmDefinition{
			Name:                  testMlAlgorithmName,
			TrainerImagePath:      "trainer-image:latest",
			TrainerImageDigest:    testImageDigest,
			PredictionImagePath:   "prediction-image:latest",
			PredictionImageDigest: testImageDigest,
		}

		cfgSvc.ValidateMLImagesForGetAlgoFlow(ctx, testAlgo)
		assert.Equal(t, "image digest is missing in registry", testAlgo.TrainerImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.DeprecatedTrainerImageDigest)
		assert.Equal(t, "", testAlgo.DeprecatedPredictionImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.PredictionImageDigest)
	})
	t.Run("ValidateMLImagesForGetAlgoFlow - Prediction image digest retrieval failed: image not found", func(t *testing.T) {
		ctx := u.AppService.AppContext()
		mockRegistryConfig := &helpersMock.MockImageRegistryConfigInterface{}
		mockRegistryConfig.On("GetImageDigest", "trainer-image:latest").Return(testImageDigest, nil)
		mockRegistryConfig.On("GetImageDigest", "prediction-image:latest").
			Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, testHedgeError.Message()))

		cfgSvc := &MLModelConfigService{
			service:        u.AppService,
			registryConfig: mockRegistryConfig,
		}

		testAlgo := &config.MLAlgorithmDefinition{
			Name:                  testMlAlgorithmName,
			TrainerImagePath:      "trainer-image:latest",
			TrainerImageDigest:    testImageDigest,
			PredictionImagePath:   "prediction-image:latest",
			PredictionImageDigest: testImageDigest,
		}

		cfgSvc.ValidateMLImagesForGetAlgoFlow(ctx, testAlgo)
		assert.Equal(t, "image digest is missing in registry", testAlgo.PredictionImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.DeprecatedPredictionImageDigest)
		assert.Equal(t, testImageDigest, testAlgo.TrainerImageDigest)
		assert.Equal(t, "", testAlgo.DeprecatedTrainerImageDigest)
	})
}

func TestBuildTwinFilter(t *testing.T) {
	t.Run("buildTwinFilter - No existing filters (adds device filter)", func(t *testing.T) {
		testDeviceFilter := config.FilterDefinition{
			Label:    "deviceName",
			Operator: "CONTAINS",
			Value:    "Device1",
		}

		result := buildTwinFilter(nil, testDeviceFilter)

		assert.Len(t, result, 1)
		assert.Equal(t, testDeviceFilter, result[0])
	})
	t.Run(
		"buildTwinFilter - Existing filters without device filter (appends device filter)",
		func(t *testing.T) {
			testFilters := []config.FilterDefinition{
				{Label: "otherLabel", Operator: "EQUALS", Value: "Value1"},
			}
			testDeviceFilter := config.FilterDefinition{
				Label:    "deviceName",
				Operator: "CONTAINS",
				Value:    "Device2",
			}

			result := buildTwinFilter(testFilters, testDeviceFilter)

			assert.Len(t, result, 2)
			assert.Equal(t, "otherLabel", result[0].Label)
			assert.Equal(t, testDeviceFilter, result[1])
		},
	)
	t.Run(
		"buildTwinFilter - Existing filters with device filter (updates device filter)",
		func(t *testing.T) {
			testFilters := []config.FilterDefinition{
				{Label: "deviceName", Operator: "EQUALS", Value: "OldDevice"},
			}
			testDeviceFilter := config.FilterDefinition{
				Label:    "deviceName",
				Operator: "CONTAINS",
				Value:    "NewDevice",
			}

			result := buildTwinFilter(testFilters, testDeviceFilter)

			assert.Len(t, result, 1)
			assert.Equal(t, testDeviceFilter, result[0])
		},
	)
	t.Run("buildTwinFilter - No filters and no 'deviceName' label", func(t *testing.T) {
		testDeviceFilter := config.FilterDefinition{
			Label:    "dummyLabel",
			Operator: "CONTAINS",
			Value:    "Device3",
		}

		result := buildTwinFilter(nil, testDeviceFilter)

		assert.Len(t, result, 1)
		assert.Equal(t, testDeviceFilter, result[0])
	})
}

func TestImageTagChanged(t *testing.T) {
	t.Run("imageTagChanged - Passed (Tag has changed)", func(t *testing.T) {
		existingImage := "nginx:1.21.0"
		updatedImage := "nginx:1.22.0"

		changed, err := imageTagChanged(existingImage, updatedImage)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Errorf("expected tag to have changed, but got false")
		}
	})
	t.Run("imageTagChanged - Passed (Tag has not changed)", func(t *testing.T) {
		existingImage := "nginx:1.21.0"
		updatedImage := "nginx:1.21.0"

		changed, err := imageTagChanged(existingImage, updatedImage)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if changed {
			t.Errorf("expected tag to have not changed, but got true")
		}
	})
	t.Run("imageTagChanged - Failed (Image name has changed)", func(t *testing.T) {
		existingImage := "nginx:1.21.0"
		updatedImage := "httpd:1.21.0"

		_, err := imageTagChanged(existingImage, updatedImage)
		if err == nil || err.Error() != "the image name is not allowed to be updated" {
			t.Errorf("expected error about image name update, but got: %v", err)
		}
	})
	t.Run("imageTagChanged - Failed (Invalid existing image format)", func(t *testing.T) {
		existingImage := "nginx"
		updatedImage := "nginx:1.21.0"

		_, err := imageTagChanged(existingImage, updatedImage)
		if err == nil ||
			err.Error() != "invalid image format: both images must have a name and tag (e.g., 'nginx:1.21.0')" {
			t.Errorf("expected error about invalid existing image format, but got: %v", err)
		}
	})
	t.Run("imageTagChanged - Failed (Invalid updated image format)", func(t *testing.T) {
		existingImage := "nginx:1.21.0"
		updatedImage := "nginx"

		_, err := imageTagChanged(existingImage, updatedImage)
		if err == nil ||
			err.Error() != "invalid image format: both images must have a name and tag (e.g., 'nginx:1.21.0')" {
			t.Errorf("expected error about invalid updated image format, but got: %v", err)
		}
	})
	t.Run("imageTagChanged - Failed (Both images invalid)", func(t *testing.T) {
		existingImage := "nginx"
		updatedImage := "nginx"

		_, err := imageTagChanged(existingImage, updatedImage)
		if err == nil ||
			err.Error() != "invalid image format: both images must have a name and tag (e.g., 'nginx:1.21.0')" {
			t.Errorf("expected error about invalid image formats, but got: %v", err)
		}
	})
}

func addExternalFeature(mlModelConfig *config.MLModelConfig) *config.MLModelConfig {
	externalFeatureName := "Severity"
	externalFeature := config.Feature{
		Type:     "CATEGORICAL",
		Name:     externalFeatureName,
		IsOutput: true,
		IsInput:  false,
	}
	mlModelConfig.MLDataSourceConfig.ExternalFeatures = []config.Feature{externalFeature}

	if mlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex == nil {
		mlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex = make(map[string]int)
	}
	mlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex["external#"+externalFeatureName] = len(mlModelConfig.MLDataSourceConfig.FeatureNameToColumnIndex)
	return mlModelConfig
}
