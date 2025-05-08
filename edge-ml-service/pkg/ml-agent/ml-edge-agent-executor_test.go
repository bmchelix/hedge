/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package ml_agent_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/config"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	mlagent "hedge/edge-ml-service/pkg/ml-agent"
	svcmocks "hedge/mocks/hedge/common/service"
	redisMock "hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	containerMngrMock "hedge/mocks/hedge/edge-ml-service/pkg/docker"
	helpersMock "hedge/mocks/hedge/edge-ml-service/pkg/helpers"
	mlagentmocks "hedge/mocks/hedge/edge-ml-service/pkg/ml-agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewMLEdgeAgentConfig(t *testing.T) {
	t.Run("NewMLEdgeAgentConfig - Passed", func(t *testing.T) {
		cfg, err := mlagent.NewMLEdgeAgentConfig(u.AppService)
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
	})
}

func TestMLEdgeAgentConfig_DownloadModel(t *testing.T) {
	t.Run("DownloadModel - Passed (Deploy model)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedHttpSender := &svcmocks.MockHTTPClient{}
		body, _ := json.Marshal(ml_model.ChunkMessage{
			ModelID:     "test-model-123",
			ChunkIndex:  0,
			TotalChunks: 1,
			ChunkData:   []byte("sample chunk data"),
		})
		testResponse := &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}
		mockedHttpSender.On("Do", mock.Anything).Return(testResponse, nil)
		mlagent.HttpClient = mockedHttpSender

		mockedMlStorage := helpersMock.MockMLStorageInterface{}
		mockedMlStorage.On("AddModelAndConfigFile", mock.Anything).Return(nil)
		mockedMlStorage.On("RemoveModelIgnoreFile").Return(nil)

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, true, "Success").
			Return(testModelDeploymentStatusSuccess)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			ModelDownloadEndpoint: "test/download",
			LocalMLModelDir:       testMlModelDir,
			RemoteNodeId:          "node-123",
			InsecureSkipVerify:    false,
			MlStorage:             &mockedMlStorage,
			Subscriber:            mockedMLAgentMQTTPublisher,
		}

		mlagent.MlStorage = &mockedMlStorage
		result, data := mlCfg.DownloadModel(u.AppFunctionContext, testModelDeployCommand)

		assert.True(t, result)
		assert.Equal(t, testModelDeployCommand, data)
	})
	t.Run("DownloadModel - Passed (Input data of type ModelDeploymentStatus)", func(t *testing.T) {
		mlCfg := &mlagent.MLEdgeAgentConfig{}

		result, data := mlCfg.DownloadModel(u.AppFunctionContext, ml_model.ModelDeploymentStatus{})

		assert.True(t, result)
		assert.Equal(t, ml_model.ModelDeploymentStatus{}, data)
	})
	t.Run("DownloadModel - Passed (cmd SyncMLEventConfig)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"SyncMLEventConfig",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mlCfg := &mlagent.MLEdgeAgentConfig{}

		result, data := mlCfg.DownloadModel(u.AppFunctionContext, testModelDeployCommand)

		assert.True(t, result)
		assert.Equal(t, testModelDeployCommand, data)
	})
	t.Run("DownloadModel - Failed (AddModelAndConfigFile failure)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedHttpSender := &svcmocks.MockHTTPClient{}
		body, _ := json.Marshal(ml_model.ChunkMessage{
			ModelID:     "test-model-123",
			ChunkIndex:  0,
			TotalChunks: 1,
			ChunkData:   []byte("sample chunk data"),
		})
		testResponse := &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}
		mockedHttpSender.On("Do", mock.Anything).Return(testResponse, nil)
		mlagent.HttpClient = mockedHttpSender

		mockedMlStorage := helpersMock.MockMLStorageInterface{}
		mockedMlStorage.On("AddModelAndConfigFile", mock.Anything).Return(errDummy)
		mlagent.MlStorage = &mockedMlStorage

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, false, "error downloading the model, error : dummy error").
			Return(testModelDeploymentStatusFailure)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			ModelDownloadEndpoint: "test/download",
			LocalMLModelDir:       testMlModelDir,
			RemoteNodeId:          "node-123",
			InsecureSkipVerify:    false,
			MlStorage:             &mockedMlStorage,
			Subscriber:            mockedMLAgentMQTTPublisher,
		}

		result, data := mlCfg.DownloadModel(u.AppFunctionContext, testModelDeployCommand)

		assert.True(t, result)
		updateTimestampIfModelDeploymentStatus(data)
		assert.Equal(t, testModelDeploymentStatusFailure, data)
	})

	t.Run("DownloadModel - Failed (Invalid data type)", func(t *testing.T) {
		mlCfg := &mlagent.MLEdgeAgentConfig{}

		result, data := mlCfg.DownloadModel(u.AppFunctionContext, "invalidData")

		assert.False(t, result)
		assert.Error(t, data.(error))
		assert.Contains(t, data.(error).Error(), "type received is not a ModelDeployCommand")
	})
	t.Run("DownloadModel - Failed (HTTP Request Error)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedHttpSender := &svcmocks.MockHTTPClient{}
		mockedHttpSender.On("Do", mock.Anything).Return(nil, errDummy)
		mlagent.HttpClient = mockedHttpSender

		mockedMlStorage := helpersMock.MockMLStorageInterface{}

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand,
			false, "error While downloading model from url: /mock/download?mlModelConfigName=TestConfig&mlAlgorithm=TestAlgo&chunkIndex=0, error: dummy error").
			Return(testModelDeploymentStatusFailure)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			ModelDownloadEndpoint: "/mock/download",
			LocalMLModelDir:       testMlModelDir,
			RemoteNodeId:          "node-123",
			InsecureSkipVerify:    false,
			MlStorage:             &mockedMlStorage,
			Subscriber:            mockedMLAgentMQTTPublisher,
		}

		result, data := mlCfg.DownloadModel(u.AppFunctionContext, testModelDeployCommand)

		assert.True(t, result)
		assert.Contains(
			t,
			data.(ml_model.ModelDeploymentStatus).Message,
			"Deployment of model failed",
		)
	})
}

func TestMLEdgeAgentConfig_ExecuteMLCommands(t *testing.T) {
	t.Run("ExecuteMLCommands - Passed", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedHttpSender := svcmocks.MockHTTPSenderInterface{}
		mockedHttpSender.On("HTTPPost", u.AppFunctionContext, mock.Anything).Return(true, nil)

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, true, "Success").
			Return(testModelDeploymentStatusSuccess)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			Subscriber: mockedMLAgentMQTTPublisher,
			HttpSender: &mockedHttpSender,
		}

		gotContinuePipeline, gotResult := mlCfg.ExecuteMLCommands(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, gotContinuePipeline)
		assert.Equal(t, testModelDeploymentStatusSuccess, gotResult)
	})
	t.Run(
		"ExecuteMLCommands - Passed (Input data of type ModelDeploymentStatus)",
		func(t *testing.T) {
			mlCfg := &mlagent.MLEdgeAgentConfig{}

			gotContinuePipeline, gotResult := mlCfg.ExecuteMLCommands(
				u.AppFunctionContext,
				ml_model.ModelDeploymentStatus{},
			)

			assert.True(t, gotContinuePipeline)
			assert.Equal(t, ml_model.ModelDeploymentStatus{}, gotResult)
		},
	)
	t.Run("ExecuteMLCommands - Failed (Invalid data type)", func(t *testing.T) {
		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, ml_model.ModelDeployCommand{}, false, "Technical error parsing model deploy command").
			Return(testModelDeploymentStatusFailure)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			Subscriber: mockedMLAgentMQTTPublisher,
		}

		gotContinuePipeline, gotResult := mlCfg.ExecuteMLCommands(
			u.AppFunctionContext,
			"invalidData",
		)

		assert.True(t, gotContinuePipeline)
		updateTimestampIfModelDeploymentStatus(gotResult)
		assert.Equal(t, testModelDeploymentStatusFailure, gotResult)
	})
	t.Run("ExecuteMLCommands - Failed (Ml-broker reinitialization failed)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedHttpSender := svcmocks.MockHTTPSenderInterface{}
		mockedHttpSender.On("HTTPPost", u.AppFunctionContext, mock.Anything).Return(false, nil)

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand,
			false, "error from API broker reinitialization").
			Return(testModelDeploymentStatusFailure)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			Subscriber: mockedMLAgentMQTTPublisher,
			HttpSender: &mockedHttpSender,
		}

		gotContinuePipeline, gotResult := mlCfg.ExecuteMLCommands(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, gotContinuePipeline)
		updateTimestampIfModelDeploymentStatus(gotResult)
		assert.Equal(t, testModelDeploymentStatusFailure, gotResult)
	})
	t.Run("ExecuteMLCommands - Failed (cmd SyncMLEventConfig failed)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"SyncMLEventConfig",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)
		testModelDeployCommand.MLEventConfigsToSync = []ml_model.SyncMLEventConfig{
			{SyncCommand: ml_model.CREATE, MLEventConfig: config.MLEventConfig{}},
		}

		mockedDBClient := redisMock.MockMLDbInterface{}
		mockedDBClient.On("AddMLEventConfig", mock.Anything).
			Return(config.MLEventConfig{}, testHedgeError)

		mockedHttpSender := svcmocks.MockHTTPSenderInterface{}
		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand,
			false, "error while saving ML event configuration, error: hedge dummy error").
			Return(testModelDeploymentStatusFailure)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			Subscriber: mockedMLAgentMQTTPublisher,
			HttpSender: &mockedHttpSender,
			DbClient:   &mockedDBClient,
		}

		gotContinuePipeline, gotResult := mlCfg.ExecuteMLCommands(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, gotContinuePipeline)
		updateTimestampIfModelDeploymentStatus(gotResult)
		assert.Equal(t, testModelDeploymentStatusFailure, gotResult)
	})
}

func TestMLEdgeAgentConfig_DeployInferenceImplementation(t *testing.T) {
	t.Run("DeployInferenceImplementation - Passed (Deploy model)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, true, "Success").
			Return(testModelDeploymentStatusSuccess)

		mockMlStorage := &helpersMock.MockMLStorageInterface{}
		mockMlStorage.On("RemoveModelIgnoreFile").Return(nil)

		mockedContainerManager := &containerMngrMock.MockContainerManager{}
		mockedContainerManager.On("RemoveContainer", mock.Anything).Return(nil)
		mockedContainerManager.On("CreatePredicationContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return("container-id", nil)
		mockedContainerManager.On("CopyModelToContainer", mock.Anything, mock.Anything).Return(nil)
		mockedContainerManager.On("StartContainer", mock.Anything, mock.Anything).Return(nil)

		setupTestDirWithConfig(t, testMlModelDir)
		t.Cleanup(func() {
			err := os.RemoveAll(testMlModelDir)
			if err != nil {
				t.Errorf("Failed to clean up test directory: %v", err)
			}
		})

		mlCfg := &mlagent.MLEdgeAgentConfig{
			ContainerManager: mockedContainerManager,
			Subscriber:       mockedMLAgentMQTTPublisher,
			MlStorage:        mockMlStorage,
			LocalMLModelDir:  testMlModelDir,
		}

		result, data := mlCfg.DeployInferenceImplementation(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, result)
		assert.Equal(t, testModelDeployCommand, data)
	})
	t.Run("DeployInferenceImplementation - Passed (Undeploy model)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"UnDeploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, true, "Success").
			Return(testModelUnDeploymentStatusSuccess)

		mockMlStorage := &helpersMock.MockMLStorageInterface{}
		mockMlStorage.On("AddModelIgnoreFile").Return(nil)

		mockedContainerManager := &containerMngrMock.MockContainerManager{}
		mockedContainerManager.On("RemoveContainer", mock.Anything).Return(nil)
		mockedContainerManager.On("CreatePredicationContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return("container-id", nil)
		mockedContainerManager.On("CopyModelToContainer", mock.Anything, mock.Anything).Return(nil)
		mockedContainerManager.On("StartContainer", mock.Anything, mock.Anything).Return(nil)

		setupTestDirWithConfig(t, testMlModelDir)
		t.Cleanup(func() {
			err := os.RemoveAll(testMlModelDir)
			if err != nil {
				t.Errorf("Failed to clean up test directory: %v", err)
			}
		})

		mlCfg := &mlagent.MLEdgeAgentConfig{
			ContainerManager: mockedContainerManager,
			Subscriber:       mockedMLAgentMQTTPublisher,
			MlStorage:        mockMlStorage,
			LocalMLModelDir:  testMlModelDir,
		}

		result, data := mlCfg.DeployInferenceImplementation(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, result)
		assert.Equal(t, testModelDeployCommand, data)
	})
	t.Run("DeployInferenceImplementation - Passed (cmd SyncMLEventConfig)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"SyncMLEventConfig",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mlCfg := &mlagent.MLEdgeAgentConfig{}

		result, data := mlCfg.DeployInferenceImplementation(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, result)
		assert.Equal(t, testModelDeployCommand, data)
	})
	t.Run(
		"DeployInferenceImplementation - Passed (Input data of type ModelDeploymentStatus)",
		func(t *testing.T) {
			mlCfg := &mlagent.MLEdgeAgentConfig{}

			gotContinuePipeline, gotResult := mlCfg.DeployInferenceImplementation(
				u.AppFunctionContext,
				ml_model.ModelDeploymentStatus{},
			)

			assert.True(t, gotContinuePipeline)
			assert.Equal(t, ml_model.ModelDeploymentStatus{}, gotResult)
		},
	)
	t.Run("DeployInferenceImplementation - Failed (Invalid input data)", func(t *testing.T) {
		mlCfg := &mlagent.MLEdgeAgentConfig{}

		result, data := mlCfg.DeployInferenceImplementation(u.AppFunctionContext, "invalidData")

		assert.False(t, result)
		assert.Equal(t, nil, data)
	})
	t.Run(
		"DeployInferenceImplementation - Failed (AddModelIgnoreFile failure)",
		func(t *testing.T) {
			testModelDeployCommand := buildTestModelDeployCommand(
				"UnDeploy",
				testMlAlgorithmName,
				testMlModelConfigName,
				1,
				[]string{"Node1"},
			)

			mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
			mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, false, "error while undeploying, modelignore file creation failed").
				Return(testModelDeploymentStatusUndeployFailure)

			mockMlStorage := &helpersMock.MockMLStorageInterface{}
			mockMlStorage.On("AddModelIgnoreFile").Return(errDummy)

			mockedContainerManager := &containerMngrMock.MockContainerManager{}
			mockedContainerManager.On("RemoveContainer", mock.Anything).Return(nil)
			mockedContainerManager.On("CreatePredicationContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return("container-id", nil)
			mockedContainerManager.On("CopyModelToContainer", mock.Anything, mock.Anything).
				Return(nil)
			mockedContainerManager.On("StartContainer", mock.Anything, mock.Anything).Return(nil)

			setupTestDirWithConfig(t, testMlModelDir)
			t.Cleanup(func() {
				err := os.RemoveAll(testMlModelDir)
				if err != nil {
					t.Errorf("Failed to clean up test directory: %v", err)
				}
			})

			mlCfg := &mlagent.MLEdgeAgentConfig{
				ContainerManager: mockedContainerManager,
				Subscriber:       mockedMLAgentMQTTPublisher,
				MlStorage:        mockMlStorage,
				LocalMLModelDir:  testMlModelDir,
			}

			result, data := mlCfg.DeployInferenceImplementation(
				u.AppFunctionContext,
				testModelDeployCommand,
			)

			assert.True(t, result)
			updateTimestampIfModelDeploymentStatus(data)
			assert.Equal(t, testModelDeploymentStatusUndeployFailure, data)
		},
	)
	t.Run(
		"DeployInferenceImplementation - Failed (ReadMLConfigWithAlgo failure)",
		func(t *testing.T) {
			testModelDeployCommand := buildTestModelDeployCommand(
				"Deploy",
				testMlAlgorithmName,
				testMlModelConfigName,
				1,
				[]string{"Node1"},
			)

			mockedContainerManager := &containerMngrMock.MockContainerManager{}
			mockedContainerManager.On("RemoveContainer", mock.Anything).Return(errDummy)

			mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
			mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, false, "error while reading config.json file of deployment, error: open testMlModelDir: no such file or directory").
				Return(testModelDeploymentStatusFailure)

			mockMlStorage := &helpersMock.MockMLStorageInterface{}
			mockMlStorage.On("RemoveModelIgnoreFile").Return(nil)

			setupTestDirWithConfig(t, "")
			t.Cleanup(func() {
				err := os.RemoveAll(testMlAlgorithmName)
				if err != nil {
					t.Errorf("Failed to clean up test directory: %v", err)
				}
			})

			mlCfg := &mlagent.MLEdgeAgentConfig{
				ContainerManager: mockedContainerManager,
				Subscriber:       mockedMLAgentMQTTPublisher,
				MlStorage:        mockMlStorage,
				LocalMLModelDir:  testMlModelDir,
			}

			result, data := mlCfg.DeployInferenceImplementation(
				u.AppFunctionContext,
				testModelDeployCommand,
			)

			assert.True(t, result)
			updateTimestampIfModelDeploymentStatus(data)
			assert.Equal(t, testModelDeploymentStatusFailure, data)
		},
	)
	t.Run("DeployInferenceImplementation - Failed (RemoveContainer failure)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedContainerManager := &containerMngrMock.MockContainerManager{}
		mockedContainerManager.On("RemoveContainer", mock.Anything).Return(errDummy)

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, false, "error while removing container testalgo").
			Return(testModelDeploymentStatusFailure)

		mockMlStorage := &helpersMock.MockMLStorageInterface{}
		mockMlStorage.On("RemoveModelIgnoreFile").Return(nil)

		setupTestDirWithConfig(t, testMlModelDir)
		t.Cleanup(func() {
			err := os.RemoveAll(testMlModelDir)
			if err != nil {
				t.Errorf("Failed to clean up test directory: %v", err)
			}
		})

		mlCfg := &mlagent.MLEdgeAgentConfig{
			ContainerManager: mockedContainerManager,
			Subscriber:       mockedMLAgentMQTTPublisher,
			MlStorage:        mockMlStorage,
			LocalMLModelDir:  testMlModelDir,
		}

		result, data := mlCfg.DeployInferenceImplementation(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, result)
		updateTimestampIfModelDeploymentStatus(data)
		assert.Equal(t, testModelDeploymentStatusFailure, data)
	})
	t.Run("DeployInferenceImplementation - Failed (CreatePredicationContainer failure)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, false, "error while creating container for testalgo").
			Return(testModelDeploymentStatusFailure)

		mockMlStorage := &helpersMock.MockMLStorageInterface{}
		mockMlStorage.On("RemoveModelIgnoreFile").Return(nil)

		mockedContainerManager := &containerMngrMock.MockContainerManager{}
		mockedContainerManager.On("RemoveContainer", mock.Anything).Return(nil)
		mockedContainerManager.On("CreatePredicationContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return("", errDummy)

		setupTestDirWithConfig(t, testMlModelDir)
		t.Cleanup(func() {
			err := os.RemoveAll(testMlModelDir)
			if err != nil {
				t.Errorf("Failed to clean up test directory: %v", err)
			}
		})

		mlCfg := &mlagent.MLEdgeAgentConfig{
			ContainerManager: mockedContainerManager,
			Subscriber:       mockedMLAgentMQTTPublisher,
			MlStorage:        mockMlStorage,
			LocalMLModelDir:  testMlModelDir,
		}

		result, data := mlCfg.DeployInferenceImplementation(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, result)
		updateTimestampIfModelDeploymentStatus(data)
		assert.Equal(t, testModelDeploymentStatusFailure, data)
	})
	t.Run(
		"DeployInferenceImplementation - Failed (CopyModelToContainer failure)",
		func(t *testing.T) {
			testModelDeployCommand := buildTestModelDeployCommand(
				"Deploy",
				testMlAlgorithmName,
				testMlModelConfigName,
				1,
				[]string{"Node1"},
			)

			mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
			mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, false, "failed copying model directory to inferencing container, error : dummy error").
				Return(testModelDeploymentStatusFailure)

			mockMlStorage := &helpersMock.MockMLStorageInterface{}
			mockMlStorage.On("RemoveModelIgnoreFile").Return(nil)

			mockedContainerManager := &containerMngrMock.MockContainerManager{}
			mockedContainerManager.On("RemoveContainer", mock.Anything).Return(nil)
			mockedContainerManager.On("CreatePredicationContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return("container-id", nil)
			mockedContainerManager.On("CopyModelToContainer", mock.Anything, mock.Anything).
				Return(errDummy)

			setupTestDirWithConfig(t, testMlModelDir)
			t.Cleanup(func() {
				err := os.RemoveAll(testMlModelDir)
				if err != nil {
					t.Errorf("Failed to clean up test directory: %v", err)
				}
			})

			mlCfg := &mlagent.MLEdgeAgentConfig{
				ContainerManager: mockedContainerManager,
				Subscriber:       mockedMLAgentMQTTPublisher,
				MlStorage:        mockMlStorage,
				LocalMLModelDir:  testMlModelDir,
			}

			result, data := mlCfg.DeployInferenceImplementation(
				u.AppFunctionContext,
				testModelDeployCommand,
			)

			assert.True(t, result)
			updateTimestampIfModelDeploymentStatus(data)
			assert.Equal(t, testModelDeploymentStatusFailure, data)
		},
	)
	t.Run("DeployInferenceImplementation - Failed (StartContainer failure)", func(t *testing.T) {
		testModelDeployCommand := buildTestModelDeployCommand(
			"Deploy",
			testMlAlgorithmName,
			testMlModelConfigName,
			1,
			[]string{"Node1"},
		)

		mockedMLAgentMQTTPublisher := &mlagentmocks.MockMLAgentMQTTPublisherInterface{}
		mockedMLAgentMQTTPublisher.On("BuildModelDownloadStatusPayload", u.AppFunctionContext, testModelDeployCommand, false, "error while starting inference container for testalgo").
			Return(testModelDeploymentStatusFailure)

		mockMlStorage := &helpersMock.MockMLStorageInterface{}
		mockMlStorage.On("RemoveModelIgnoreFile").Return(nil)

		mockedContainerManager := &containerMngrMock.MockContainerManager{}
		mockedContainerManager.On("RemoveContainer", mock.Anything).Return(nil)
		mockedContainerManager.On("CreatePredicationContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return("container-id", nil)
		mockedContainerManager.On("CopyModelToContainer", mock.Anything, mock.Anything).Return(nil)
		mockedContainerManager.On("StartContainer", mock.Anything, mock.Anything).Return(errDummy)

		setupTestDirWithConfig(t, testMlModelDir)
		t.Cleanup(func() {
			err := os.RemoveAll(testMlModelDir)
			if err != nil {
				t.Errorf("Failed to clean up test directory: %v", err)
			}
		})

		mlCfg := &mlagent.MLEdgeAgentConfig{
			ContainerManager: mockedContainerManager,
			Subscriber:       mockedMLAgentMQTTPublisher,
			MlStorage:        mockMlStorage,
			LocalMLModelDir:  testMlModelDir,
		}

		result, data := mlCfg.DeployInferenceImplementation(
			u.AppFunctionContext,
			testModelDeployCommand,
		)

		assert.True(t, result)
		updateTimestampIfModelDeploymentStatus(data)
		assert.Equal(t, testModelDeploymentStatusFailure, data)
	})
}

func TestMLEdgeAgentConfig_LoadConfigurations(t *testing.T) {
	t.Run("LoadConfigurations - Passed", func(t *testing.T) {
		mlCfg := &mlagent.MLEdgeAgentConfig{}

		mlCfg.LoadConfigurations(u.AppService)

		dir, _ := filepath.Abs(testMlModelDir)
		assert.Equal(t, "deploy-topic", mlCfg.ModelDeployTopic)
		assert.Equal(t, dir, mlCfg.LocalMLModelDir)
		assert.Equal(t, "download-topic", mlCfg.ModelDownloadTopic)
		assert.Equal(t, "", mlCfg.RemoteNodeId)
		assert.Equal(t, "/reinitialize", mlCfg.ReinitializeEndpoint)
		assert.Equal(t, "/download", mlCfg.ModelDownloadEndpoint)
		assert.Equal(t, "mqtt-user", mlCfg.MqttUserName)
		assert.Equal(t, "mqtt-password", mlCfg.MqttPassword)
		assert.Equal(t, "tcp", mlCfg.MqttScheme)
		assert.Equal(t, "localhost", mlCfg.MqttServer)
		assert.Equal(t, int64(1883), mlCfg.MqttPort)
		assert.False(t, mlCfg.InsecureSkipVerify)
		assert.True(t, mlCfg.ExposePredictContainerPort)
	})
}

func TestMLEdgeAgentConfig_SetDbClient(t *testing.T) {
	t.Run("SetDbClient - Passed", func(t *testing.T) {
		mlAgentConfig := &mlagent.MLEdgeAgentConfig{}
		mockedDBClient := redisMock.MockMLDbInterface{}
		mlAgentConfig.DbClient = &mockedDBClient

		mlAgentConfig.SetDbClient(u.AppService)
		if mlAgentConfig.DbClient != &mockedDBClient {
			t.Error("Expected dbClient to not be initialized again, but it is")
		}
	})
}

func TestMLEdgeAgentConfig_SetHttpSender(t *testing.T) {
	t.Run("SetHttpSender - Passed", func(t *testing.T) {
		mockMLBrokerUrl := "http://mock-ml-broker-url"

		mlCfg := &mlagent.MLEdgeAgentConfig{}

		mlCfg.SetHttpSender(mockMLBrokerUrl)

		if mlCfg.HttpSender == nil {
			t.Errorf("HttpSender was not set")
		}
	})
}

func TestMLEdgeAgentConfig_GetReinitializeEndpointBase(t *testing.T) {
	mlCfg := &mlagent.MLEdgeAgentConfig{
		ReinitializeEndpoint: "/api/v3/ml_broker/reinitialize",
	}
	result := mlCfg.GetReinitializeEndpoint()
	expected := "/api/v3/ml_broker/reinitialize"
	assert.Equal(t, expected, result, "Unexpected result for GetReinitializeEndpoint")
}

func TestMLEdgeAgentConfig_GetModelDownloadEndpoint(t *testing.T) {
	mlCfg := &mlagent.MLEdgeAgentConfig{
		ModelDownloadEndpoint: "/api/v3/ml_broker/download_model",
	}
	result := mlCfg.GetModelDownloadEndpoint()
	expected := "/api/v3/ml_broker/download_model"
	assert.Equal(t, expected, result, "Unexpected result for GetModelDownloadEndpoint")
}

func TestMLEdgeAgentConfig_GetModelDeployTopic(t *testing.T) {
	mlCfg := &mlagent.MLEdgeAgentConfig{
		ModelDeployTopic: "ml_edge_agent_model_deploy",
	}
	result := mlCfg.GetModelDeployTopic()
	expected := "ml_edge_agent_model_deploy"
	assert.Equal(t, expected, result, "Unexpected result for GetModelDeployTopic")
}

func TestMLEdgeAgentConfig_GetModelDownloadTopic(t *testing.T) {
	mlCfg := &mlagent.MLEdgeAgentConfig{
		ModelDownloadTopic: "testDownloadTopic",
	}
	result := mlCfg.GetModelDownloadTopic()
	expected := "testDownloadTopic"
	assert.Equal(t, expected, result, "Unexpected result for GetModelDownloadTopic")
}

func TestMLEdgeAgentConfig_SyncMLEventConfigs(t *testing.T) {
	mockLogger := u.AppService.LoggingClient()

	t.Run("SyncMLEventConfigs - SyncCommand CREATE", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockEventConfig := buildTestMLEventConfigs()[0]

		mockCommand := ml_model.ModelDeployCommand{
			MLEventConfigsToSync: []ml_model.SyncMLEventConfig{
				{SyncCommand: ml_model.CREATE, MLEventConfig: mockEventConfig},
			},
		}

		mockedDBClient.On("AddMLEventConfig", mockEventConfig).Return(mockEventConfig, nil)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			DbClient: &mockedDBClient,
		}

		err := mlCfg.SyncMLEventConfigs(mockCommand, mockLogger)
		assert.NoError(t, err)
	})
	t.Run("SyncMLEventConfigs - SyncCommand UPDATE", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}

		mockOldEventConfig := buildTestMLEventConfigs()[0]
		mockNewEventConfig := buildTestMLEventConfigs()[0]

		mockCommand := ml_model.ModelDeployCommand{
			MLEventConfigsToSync: []ml_model.SyncMLEventConfig{
				{
					SyncCommand:      ml_model.UPDATE,
					OldMLEventConfig: mockOldEventConfig,
					MLEventConfig:    mockNewEventConfig,
				},
			},
		}

		mockedDBClient.On("GetMLEventConfigByName", mockOldEventConfig.MLAlgorithm, mockOldEventConfig.MlModelConfigName, mockOldEventConfig.EventName).
			Return(mockOldEventConfig, nil)
		mockedDBClient.On("UpdateMLEventConfig", mockOldEventConfig, mockNewEventConfig).
			Return(mockNewEventConfig, nil)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			DbClient: &mockedDBClient,
		}

		err := mlCfg.SyncMLEventConfigs(mockCommand, mockLogger)
		assert.NoError(t, err)
	})
	t.Run("SyncMLEventConfigs - SyncCommand UPDATE (non-existing event)", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}

		mockOldEventConfig := buildTestMLEventConfigs()[0]
		mockNewEventConfig := buildTestMLEventConfigs()[0]

		mockCommand := ml_model.ModelDeployCommand{
			MLEventConfigsToSync: []ml_model.SyncMLEventConfig{
				{
					SyncCommand:      ml_model.UPDATE,
					OldMLEventConfig: mockOldEventConfig,
					MLEventConfig:    mockNewEventConfig,
				},
			},
		}

		mockedDBClient.On("GetMLEventConfigByName", mockOldEventConfig.MLAlgorithm, mockOldEventConfig.MlModelConfigName, mockOldEventConfig.EventName).
			Return(mockOldEventConfig, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "Not Found"))
		mockedDBClient.On("AddMLEventConfig", mockNewEventConfig).Return(mockNewEventConfig, nil)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			DbClient: &mockedDBClient,
		}

		err := mlCfg.SyncMLEventConfigs(mockCommand, mockLogger)
		assert.NoError(t, err)
	})
	t.Run("SyncMLEventConfigs - SyncCommand DELETE", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}
		mockEventConfig := buildTestMLEventConfigs()[0]

		mockCommand := ml_model.ModelDeployCommand{
			MLEventConfigsToSync: []ml_model.SyncMLEventConfig{
				{SyncCommand: ml_model.DELETE, MLEventConfig: mockEventConfig},
			},
		}

		mockedDBClient.On("DeleteMLEventConfigByName", mockEventConfig.MLAlgorithm, mockEventConfig.MlModelConfigName, mockEventConfig.EventName).
			Return(nil)

		mlCfg := &mlagent.MLEdgeAgentConfig{
			DbClient: &mockedDBClient,
		}

		err := mlCfg.SyncMLEventConfigs(mockCommand, mockLogger)
		assert.NoError(t, err)
	})
	t.Run("SyncMLEventConfigs - Unrecognized SyncCommand", func(t *testing.T) {
		mockedDBClient := redisMock.MockMLDbInterface{}

		mockCommand := ml_model.ModelDeployCommand{
			MLEventConfigsToSync: []ml_model.SyncMLEventConfig{
				{SyncCommand: "UNKNOWN_COMMAND"},
			},
		}

		mlCfg := &mlagent.MLEdgeAgentConfig{
			DbClient: &mockedDBClient,
		}

		err := mlCfg.SyncMLEventConfigs(mockCommand, mockLogger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unrecognized SyncCommand")
	})
}

func TestGetContainerNameAndPortFromUrl(t *testing.T) {
	t.Run("GetContainerNameAndPortFromUrl - Passed (Valid HTTP URL with port)", func(t *testing.T) {
		input := config.MLAlgorithmDefinition{
			DefaultPredictionEndpointURL: "http://multivariate-deepvar-timeseries:55000/api/v3/predict",
			Name:                         "DeepVarAlgorithm",
		}
		expectedContainer := "multivariate-deepvar-timeseries"
		expectedPort := int64(55000)

		containerName, port := mlagent.GetContainerNameAndPortFromUrl(&input)

		assert.Equal(t, expectedContainer, containerName, "Container name mismatch")
		assert.Equal(t, expectedPort, port, "Port mismatch")
	})
	t.Run(
		"GetContainerNameAndPortFromUrl - Passed (Valid HTTP URL with IP address)",
		func(t *testing.T) {
			input := config.MLAlgorithmDefinition{
				DefaultPredictionEndpointURL: "http://192.168.1.10:8080/api/v3/predict",
				Name:                         "IPAddressAlgorithm",
			}
			expectedContainer := "ipaddressalgorithm"
			expectedPort := int64(8080)

			containerName, port := mlagent.GetContainerNameAndPortFromUrl(&input)

			assert.Equal(t, expectedContainer, containerName, "Container name mismatch")
			assert.Equal(t, expectedPort, port, "Port mismatch")
		},
	)
	t.Run("GetContainerNameAndPortFromUrl - Failed (Invalid port in HTTP URL)", func(t *testing.T) {
		input := config.MLAlgorithmDefinition{
			DefaultPredictionEndpointURL: "http://invalid-port:abc/api/v3/predict",
			Name:                         "InvalidPortAlgorithm",
		}
		expectedContainer := "invalid-port"
		expectedPort := int64(80)

		containerName, port := mlagent.GetContainerNameAndPortFromUrl(&input)

		assert.Equal(t, expectedContainer, containerName, "Container name mismatch")
		assert.Equal(t, expectedPort, port, "Port mismatch")
	})
	t.Run(
		"GetContainerNameAndPortFromUrl - Failed (Non-HTTP URL and default prediction port)",
		func(t *testing.T) {
			input := config.MLAlgorithmDefinition{
				DefaultPredictionEndpointURL: "/api/v3/predict",
				Name:                         "DefaultAlgorithm",
				DefaultPredictionPort:        49000,
			}
			expectedContainer := "defaultalgorithm"
			expectedPort := int64(49000)

			containerName, port := mlagent.GetContainerNameAndPortFromUrl(&input)

			assert.Equal(t, expectedContainer, containerName, "Container name mismatch")
			assert.Equal(t, expectedPort, port, "Port mismatch")
		},
	)
	t.Run(
		"GetContainerNameAndPortFromUrl - Failed (Empty prediction URL with default port)",
		func(t *testing.T) {
			input := config.MLAlgorithmDefinition{
				DefaultPredictionEndpointURL: "",
				Name:                         "EmptyURLAlgorithm",
				DefaultPredictionPort:        49001,
			}
			expectedContainer := "emptyurlalgorithm"
			expectedPort := int64(49001)

			containerName, port := mlagent.GetContainerNameAndPortFromUrl(&input)

			assert.Equal(t, expectedContainer, containerName, "Container name mismatch")
			assert.Equal(t, expectedPort, port, "Port mismatch")
		},
	)
}
