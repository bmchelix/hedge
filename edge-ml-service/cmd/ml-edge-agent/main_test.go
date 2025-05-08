/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"hedge/common/client"
	bmcmodel "hedge/common/models"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	"hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	mlagentmocks "hedge/mocks/hedge/edge-ml-service/pkg/ml-agent"
	"io"
	"net/http"
	"testing"
)

var (
	mockedMLAgentService *mocks.ApplicationService
	configs              map[string]string
	mockMlAgentConfig    mlagentmocks.MockMLEdgeAgentConfigInterface
	mockedDBClient       redis.MockMLDbInterface
	testError            = errors.New("dummy error")
)

func init() {
	configs = map[string]string{
		"MetaDataServiceUrl":    "http://localhost:59881",
		"RedisHost":             "edgex-redis",
		"ModelDeployTopic":      "ModelDeployTopic",
		"HedgeAdminURL":         "http://localhost:48098",
		"ModelDownloadEndpoint": "https://clm-aus-vi37qf.bmc.com/hedge/api/v3/ml_management/model/modelFile",
		"ImageRegistry":         "test-registry",
	}

	mockedMLAgentService = utils.NewApplicationServiceMock(configs).AppService

	mockedDBClient = redis.MockMLDbInterface{}
	mockMlAgentConfig = mlagentmocks.MockMLEdgeAgentConfigInterface{}
	mockMlAgentConfig.On("SetHttpClient").Return(&mockedDBClient)
	mockMlAgentConfig.On("SetDbClient", mock.Anything).Return()
	mockMlAgentConfig.On("GetReinitializeEndpoint").Return("http://localhost:48120/api/v3/ml_broker/reinitialize")
	mockMlAgentConfig.On("SetHttpSender", "http://localhost:48120/api/v3/ml_broker/reinitialize").Return()
	mockMlAgentConfig.On("GetModelDeployTopic").Return("ModelDeploy")
	mockMlAgentConfig.On("GetModelDownloadEndpoint").Return("https://clm-aus-vi37qf.bmc.com/hedge/api/v3/ml_management/model/modelFile")
}

func TestMain_setAppService(t *testing.T) {
	t.Run("setAppService - Passed", func(t *testing.T) {
		mockCreator := &svcmocks.MockAppServiceCreator{}
		appServiceCreator = mockCreator
		mockCreator.On("NewAppServiceWithTargetType", client.HedgeMLEdgeAgentServiceKey, &ml_model.ModelDeployCommand{}).Return(mockedMLAgentService, true)

		setAppService()
		assert.Equal(t, mockedMLAgentService, serviceInt, "Service should be assigned correctly")
		serviceInt = nil
	})
	t.Run("setAppService - Failed", func(t *testing.T) {
		mockCreator := new(svcmocks.MockAppServiceCreator)
		mockCreator.On("NewAppServiceWithTargetType", client.HedgeMLEdgeAgentServiceKey, &ml_model.ModelDeployCommand{}).Return(mockedMLAgentService, false)
		appServiceCreator = mockCreator

		exitCalled := false
		exitCode := 0
		originalOsExit := osExit
		osExit = func(code int) {
			exitCalled = true
			exitCode = code
		}

		setAppService()
		assert.True(t, exitCalled, "os.Exit should be called on failure")
		assert.Equal(t, -1, exitCode, "os.Exit should be called with -1")
		serviceInt = nil
		osExit = originalOsExit
	})
}

func TestMain_setAgentConfig(t *testing.T) {
	mockCreator := &svcmocks.MockAppServiceCreator{}
	appServiceCreator = mockCreator
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeMLEdgeAgentServiceKey, &ml_model.ModelDeployCommand{}).Return(mockedMLAgentService, true)
	setAppService()

	mlAgentConfig = &mockMlAgentConfig

	setAgentConfig()
	mockMlAgentConfig.EXPECT().SetHttpSender(mockMlAgentConfig.GetReinitializeEndpoint()).Times(1)
	mockMlAgentConfig.EXPECT().GetReinitializeEndpoint().Return("mockedReinitializeEndpoint").Times(1)
	mockMlAgentConfig.EXPECT().GetModelDownloadEndpoint().Return("mockedModelDownloadEndpoint").Times(1)
	serviceInt = nil
	mlAgentConfig = nil
}

func TestMain_main(t *testing.T) {
	mockCreator := &svcmocks.MockAppServiceCreator{}
	appServiceCreator = mockCreator
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeMLEdgeAgentServiceKey, &ml_model.ModelDeployCommand{}).Return(mockedMLAgentService, true)
	serviceInt = mockedMLAgentService

	t.Run("main - Passed", func(t *testing.T) {
		mlAgentConfig = &mockMlAgentConfig

		testResponseBody, _ := json.Marshal(bmcmodel.Node{Name: "nodeName", NodeId: "nodeId"})
		testResponse := http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(testResponseBody)),
			Header:     make(http.Header),
		}
		mockHttpClient := svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.Anything).Return(&testResponse, nil)
		client.Client = &mockHttpClient

		mockedMLAgentService.On("AddFunctionsPipelineForTopics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		mockedMLAgentService.On("Run").Return(nil).Once()

		t.Cleanup(func() {
			serviceInt = nil
			mlAgentConfig = nil
		})

		main()
		assert.NoError(t, nil)
	})
	t.Run("main - Failed (GetNodeTopicName() failure)", func(t *testing.T) {
		mlAgentConfig = &mockMlAgentConfig

		mockHttpClient := svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.Anything).Return(nil, testError)
		client.Client = &mockHttpClient

		mockedMLAgentService.On("AddFunctionsPipelineForTopics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		mockedMLAgentService.On("Run").Return(nil).Once()

		exitCalled := false
		exitCode := 0
		originalOsExit := osExit
		osExit = func(code int) {
			exitCalled = true
			exitCode = code
		}

		t.Cleanup(func() {
			serviceInt = nil
			mlAgentConfig = nil
			osExit = originalOsExit
		})

		main()
		assert.True(t, exitCalled, "os.Exit should be called on failure")
		assert.Equal(t, -1, exitCode, "os.Exit should be called with -1")
	})
	t.Run("main - Failed (AddFunctionsPipelineForTopics() failure)", func(t *testing.T) {
		mlAgentConfig = &mockMlAgentConfig

		testResponseBody, _ := json.Marshal(bmcmodel.Node{Name: "nodeName", NodeId: "nodeId"})
		testResponse := http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(testResponseBody)),
			Header:     make(http.Header),
		}
		mockHttpClient := svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.Anything).Return(&testResponse, nil)
		client.Client = &mockHttpClient

		mockedMLAgentService.On("AddFunctionsPipelineForTopics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(testError).Once()
		mockedMLAgentService.On("Run").Return(nil).Once()

		exitCalled := false
		exitCode := 0
		originalOsExit := osExit
		osExit = func(code int) {
			exitCalled = true
			exitCode = code
		}

		t.Cleanup(func() {
			serviceInt = nil
			mlAgentConfig = nil
			osExit = originalOsExit
		})

		main()
		assert.True(t, exitCalled, "os.Exit should be called on failure")
		assert.Equal(t, -1, exitCode, "os.Exit should be called with -1")
	})
	t.Run("main - Failed (Run() failure)", func(t *testing.T) {
		mlAgentConfig = &mockMlAgentConfig

		testResponseBody, _ := json.Marshal(bmcmodel.Node{Name: "nodeName", NodeId: "nodeId"})
		testResponse := http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(testResponseBody)),
			Header:     make(http.Header),
		}
		mockHttpClient := svcmocks.MockHTTPClient{}
		mockHttpClient.On("Do", mock.Anything).Return(&testResponse, nil)
		client.Client = &mockHttpClient

		mockedMLAgentService.On("AddFunctionsPipelineForTopics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()
		mockedMLAgentService.On("Run").Return(testError).Once()

		exitCalled := false
		exitCode := 0
		originalOsExit := osExit
		osExit = func(code int) {
			exitCalled = true
			exitCode = code
		}

		t.Cleanup(func() {
			serviceInt = nil
			mlAgentConfig = nil
			osExit = originalOsExit
		})

		main()
		assert.True(t, exitCalled, "os.Exit should be called on failure")
		assert.Equal(t, -1, exitCode, "os.Exit should be called with -1")
	})
}
