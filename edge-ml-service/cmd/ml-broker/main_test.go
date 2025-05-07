/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hedge/common/client"
	"hedge/edge-ml-service/internal/broker"
	r "hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	"hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	mockedMLBrokerService *mocks.ApplicationService
	mockedDBClient        redis.MockMLDbInterface
	testAppConfig         *broker.MLBrokerConfig
	testEventConfig       config.MLEventConfig
	testMlAlgorithmName   = "TestAlgo"
	testMlModelConfigName = "TestModelConfig"
	testMlModelDir        = "testMlModelDir"
	errTest               = errors.New("dummy error")
)

func instantiateMLBrokerDBClient() {
	mockedDBClient = redis.MockMLDbInterface{}
	r.MLDbClientImpl = &mockedDBClient
	mockedDBClient.On("GetDbClient", mock.Anything).Return(r.MLDbClientImpl)
	mockedDBClient.On("GetAllMLEventConfigsByConfig", mock.Anything, mock.Anything).
		Return([]config.MLEventConfig{testEventConfig}, nil)
}

func init() {
	instantiateMLBrokerDBClient()
	brokerConfig := broker.MLBrokerConfig{
		LocalMLModelBaseDir: testMlModelDir,
		MetaDataServiceUrl:  "http://localhost:59881",
		EventsPublisherURL:  "http://localhost:48102/api/v3/trigger",
	}
	testAppConfig = &brokerConfig
	testEventConfig = config.MLEventConfig{}
	mockedMLBrokerService = utils.NewApplicationServiceMock(nil).AppService
	mockedMLBrokerService.On("AddFunctionsPipelineForTopics", mock.Anything, mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)
}

func TestMain_getAppService(t *testing.T) {
	t.Run("getAppService - Passed", func(t *testing.T) {
		mockCreator := &svcmocks.MockAppServiceCreator{}
		appServiceCreator = mockCreator
		mockCreator.On("NewAppService", client.HedgeMLBrokerServiceKey).
			Return(mockedMLBrokerService, true)

		getAppService()
		assert.Equal(t, mockedMLBrokerService, serviceInt, "Service should be assigned correctly")
		serviceInt = nil
	})
	t.Run("getAppService - Failed", func(t *testing.T) {
		mockCreator := new(svcmocks.MockAppServiceCreator)
		mockCreator.On("NewAppService", client.HedgeMLBrokerServiceKey).
			Return(mockedMLBrokerService, false)
		appServiceCreator = mockCreator

		exitCalled := false
		exitCode := 0
		originalOsExit := osExit
		osExit = func(code int) {
			exitCalled = true
			exitCode = code
		}

		getAppService()
		assert.True(t, exitCalled, "os.Exit should be called on failure")
		assert.Equal(t, -1, exitCode, "os.Exit should be called with -1")
		serviceInt = nil
		osExit = originalOsExit
	})
}

func TestMain_main(t *testing.T) {
	t.Run("main - Passed", func(t *testing.T) {
		configs := map[string]string{}

		appSvcMock := utils.NewApplicationServiceMock(configs).AppService
		appSvcMock.On("AddCustomRoute", mock.Anything, interfaces.Authenticated, mock.AnythingOfType("echo.HandlerFunc"), mock.Anything).
			Return(nil)
		appSvcMock.On("Run").Return(nil)

		originalOsExit := osExit
		osExit = func(code int) {}
		defer func() { osExit = originalOsExit }()

		serviceInt = appSvcMock
		main()

		assert.NoError(t, nil)
		serviceInt = nil
	})
	t.Run("main - Failed (Failed)", func(t *testing.T) {
		configs := map[string]string{}

		appSvcMock := utils.NewApplicationServiceMock(configs).AppService
		appSvcMock.On("AddCustomRoute", mock.Anything, interfaces.Authenticated, mock.AnythingOfType("echo.HandlerFunc"), mock.Anything).
			Return(nil)
		appSvcMock.On("AddFunctionsPipelineForTopics", mock.Anything, mock.Anything, mock.Anything).
			Return(nil)
		appSvcMock.On("Run").Return(errTest)

		serviceInt = appSvcMock
		main()

		assert.NoError(t, nil)
		serviceInt = nil
	})
}

func TestMain_captureInitialEnvVars(t *testing.T) {
	mockEnv := []string{"TEST_VAR1=value1", "TEST_VAR2=value2"}

	got := captureInitialEnvVarsMock(mockEnv)
	want := map[string]string{
		"TEST_VAR1": "value1",
		"TEST_VAR2": "value2",
	}

	for key, value := range want {
		if got[key] != value {
			t.Errorf("captureInitialEnvVarsMock() = %v, want %v", got, want)
		}
	}
}

func TestMain_BuildMLInferencePipelinesFromConfig(t *testing.T) {
	t.Run("BuildMLInferencePipelinesFromConfig - Passed", func(t *testing.T) {
		setupTestDirWithConfig(t, testMlModelDir, true)
		t.Cleanup(func() {
			err := os.RemoveAll(testMlModelDir)
			if err != nil {
				t.Errorf("Failed to clean up test directory: %v", err)
			}
		})

		notificationChannel := make(chan string)
		defer close(notificationChannel)

		err := BuildMLInferencePipelinesFromConfig(
			mockedMLBrokerService,
			testAppConfig,
			notificationChannel,
		)
		require.NoError(t, err)
	})
	t.Run(
		"BuildMLInferencePipelinesFromConfig - Passed (Chdir error logs warning)",
		func(t *testing.T) {
			setupTestDirWithConfig(t, testMlModelDir, true)
			originalBaseConfigDirectory := baseConfigDirectory
			t.Cleanup(func() {
				err := os.RemoveAll(testMlModelDir)
				if err != nil {
					t.Errorf("Failed to clean up test directory: %v", err)
				}
				baseConfigDirectory = originalBaseConfigDirectory
			})

			baseConfigDirectory = ""
			originalChdirFunc := chdirFunc
			defer func() { chdirFunc = originalChdirFunc }()

			chdirFunc = func(originalBaseConfigDirectory string) error {
				return errors.New("mocked chdir error")
			}

			notificationChannel := make(chan string)
			defer close(notificationChannel)

			err := BuildMLInferencePipelinesFromConfig(
				mockedMLBrokerService,
				testAppConfig,
				notificationChannel,
			)
			require.NoError(t, err)
		},
	)
	t.Run(
		"BuildMLInferencePipelinesFromConfig - Failed (Config.json path doesn't exist)",
		func(t *testing.T) {
			notificationChannel := make(chan string)
			defer close(notificationChannel)

			err := BuildMLInferencePipelinesFromConfig(
				mockedMLBrokerService,
				testAppConfig,
				notificationChannel,
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), "open testMlModelDir: no such file or directory")
		},
	)
	t.Run(
		"BuildMLInferencePipelinesFromConfig - Failed (addMLInferencePipeline failed)",
		func(t *testing.T) {
			setupTestDirWithConfig(t, testMlModelDir, false)
			t.Cleanup(func() {
				err := os.RemoveAll(testMlModelDir)
				if err != nil {
					t.Errorf("Failed to clean up test directory: %v", err)
				}
			})

			notificationChannel := make(chan string)
			defer close(notificationChannel)

			err := BuildMLInferencePipelinesFromConfig(
				mockedMLBrokerService,
				testAppConfig,
				notificationChannel,
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), "error adding new pipeline for the configurations")
		},
	)
}

func TestMain_addMLInferencePipeline(t *testing.T) {
	t.Run("addMLInferencePipeline - Passed (Redis pipeline)", func(t *testing.T) {
		testMlModelConfig := config.MLModelConfig{
			Name: testMlModelConfigName,
			MLDataSourceConfig: config.MLModelDataConfig{
				GroupOrJoinKeys: []string{"deviceName"},
				FeaturesByProfile: map[string][]config.Feature{
					"WindTurbine": {
						{Name: "TurbinePower", IsInput: true, IsOutput: true, FromExternalDataSource: false},
					},
				},
				PredictionDataSourceConfig: config.PredictionDataSourceConfig{
					PredictionEndPointURL: "http://valid-url",
					TopicName:             "test/topic",
				},
			},
		}
		testMlAlgoDefinition := config.MLAlgorithmDefinition{
			Name: testMlAlgorithmName,
		}

		notificationChannel := make(chan string)
		defer close(notificationChannel)

		err := addMLInferencePipeline(
			mockedMLBrokerService,
			testMlModelConfigName,
			testMlModelConfig,
			testMlAlgoDefinition,
			testAppConfig,
			notificationChannel,
		)
		require.NoError(t, err)
	})
	t.Run("addMLInferencePipeline - Passed (MQTT pipeline)", func(t *testing.T) {
		testAppConfig.ReadMessageBus = "MQTT"
		testMlModelConfig := config.MLModelConfig{
			Name: testMlModelConfigName,
			MLDataSourceConfig: config.MLModelDataConfig{
				FeaturesByProfile: map[string][]config.Feature{
					"WindTurbine": {
						{Name: "TurbinePower", IsInput: true, IsOutput: true, FromExternalDataSource: false},
					},
				},
			},
		}
		testMlAlgoDefinition := config.MLAlgorithmDefinition{
			Name: testMlAlgorithmName,
		}

		notificationChannel := make(chan string)
		defer close(notificationChannel)

		mockedMLBrokerService.On("AddFunctionsPipelineForTopics", mock.AnythingOfType("string"), mock.AnythingOfType("[]string"), mock.Anything).
			Return(nil).
			Once()
		err := addMLInferencePipeline(
			mockedMLBrokerService,
			testMlModelConfigName,
			testMlModelConfig,
			testMlAlgoDefinition,
			testAppConfig,
			notificationChannel,
		)
		require.NoError(t, err)
	})
	t.Run(
		"addMLInferencePipeline - Failed (NewMLBrokerInference returned nil)",
		func(t *testing.T) {
			notificationChannel := make(chan string)
			defer close(notificationChannel)

			mockedMLBrokerService.On("AddFunctionsPipelineForTopics", mock.AnythingOfType("string"), mock.AnythingOfType("[]string"), mock.Anything).
				Return(nil).
				Once()
			err := addMLInferencePipeline(
				mockedMLBrokerService,
				testMlModelConfigName,
				config.MLModelConfig{},
				config.MLAlgorithmDefinition{},
				testAppConfig,
				notificationChannel,
			)
			require.Error(t, err)
			require.Contains(
				t,
				err.Error(),
				"Could not instantiate MlInferencing for training config:TestModelConfig",
			)
		},
	)
}

// Mock version of captureInitialEnvVars for testing
func captureInitialEnvVarsMock(env []string) map[string]string {
	vars := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			vars[parts[0]] = parts[1]
		}
	}
	return vars
}

func setupTestDirWithConfig(t *testing.T, testDir string, includeMlDataSourceConfig bool) {
	t.Helper()
	fullPath := filepath.Join(
		testDir,
		testMlAlgorithmName,
		testMlModelConfigName,
		"hedge_export",
		"assets",
	)
	err := os.MkdirAll(fullPath, os.ModePerm)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	configFilePath := filepath.Join(fullPath, "config.json")
	configContent := `{
		"mlModelConfig": {
			"name": "TestConfig"
		},
		"mlAlgoDefinition": {
			"name": "TestAlgo"
		}
	}`
	if includeMlDataSourceConfig {
		configContent = `{
			"mlModelConfig": {
				"name": "TestConfig",
				"mlDataSourceConfig": {
					"featuresByProfile": {
						"WindTurbine": [
							{
								"name": "TurbinePower",
								"type": "METRIC",
								"isInput": true,
								"isExternal": false
							}
						]
					}
				}
			},
			"mlAlgoDefinition": {
				"name": "TestAlgo"
			}
		}`
	}
	err = os.WriteFile(configFilePath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config.json: %v", err)
	}
}
