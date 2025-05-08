/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	conf "hedge/app-services/hedge-event-publisher/config"
	redis "hedge/app-services/hedge-event-publisher/pkg/db"
	"hedge/common/client"
	bmcmodel "hedge/common/models"
	"hedge/common/service"
	redismock "hedge/mocks/hedge/app-services/hedge-event-publisher/pkg/db"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	"bytes"
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"testing"
)

var (
	appSvcMock   *mocks.ApplicationService
	mockResponse http.Response
)

func Init() {
	dBMockClient := redismock.MockDBClient{}
	redis.DBClientImpl = &dBMockClient
	dBMockClient.On("GetDbClient", mock.Anything).Return(redis.DBClientImpl)

	appSvcMock = utils.NewApplicationServiceMock(map[string]string{}).AppService
	mockResponseBody, _ := json.Marshal(bmcmodel.Node{Name: "nodeName", NodeId: "nodeId"})
	mockResponse = http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(mockResponseBody)),
		Header:     make(http.Header),
	}
}

func TestGetAppService(t *testing.T) {
	Init()
	mockCreator := &svcmocks.MockAppServiceCreator{}
	appServiceCreator = mockCreator
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeEventPublisherServiceKey, &bmcmodel.HedgeEvent{}).Return(appSvcMock, true)

	mockedApp := eventPublisher{}
	mockedApp.getEventPublisher()
	assert.Equal(t, appSvcMock, mockedApp.service, "Service should be assigned correctly")
	appServiceCreator = nil
}

func TestGetAppService_Failure(t *testing.T) {
	Init()
	mockCreator := new(svcmocks.MockAppServiceCreator)
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeEventPublisherServiceKey, &bmcmodel.HedgeEvent{}).Return(appSvcMock, false)
	appServiceCreator = mockCreator

	exitCalled := false
	exitCode := 0
	originalOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	mockedApp := eventPublisher{}
	mockedApp.getEventPublisher()
	assert.True(t, exitCalled, "os.Exit should be called on failure")
	assert.Equal(t, -1, exitCode, "os.Exit should be called with -1")
	osExit = originalOsExit
	appServiceCreator = nil
}

func Test_main_Success(t *testing.T) {
	Init()
	configs := map[string]string{
		"EventTopic":    "BMCEvents",
		"HedgeAdminURL": "http://hedge-admin:480111",
		"RedisHost":     "edgex-redis",
		"RedisPort":     "6997",
	}
	appSvcMock1 := utils.NewApplicationServiceMock(configs).AppService

	mockCreator := new(svcmocks.MockAppServiceCreator)
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeEventPublisherServiceKey, &bmcmodel.HedgeEvent{}).Return(appSvcMock1, true)
	appServiceCreator = mockCreator

	dBMockClient := redismock.MockDBClient{}
	redis.DBClientImpl = &dBMockClient
	dBMockClient.On("GetDbClient", mock.Anything).Return(redis.DBClientImpl)

	mockHttpClient := svcmocks.MockHTTPClient{}
	mockHttpClient.On("Do", mock.Anything).Return(&mockResponse, nil)
	client.Client = &mockHttpClient

	appSvcMock1.On("LoadCustomConfig", mock.Anything, mock.Anything).Return(nil)
	appSvcMock1.On("ListenForCustomConfigChanges", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	appSvcMock1.On("SetDefaultFunctionsPipeline", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	appSvcMock1.On("Run").Return(nil)

	main()
	assert.NoError(t, nil)
	appServiceCreator = nil
}

func Test_main_Failure(t *testing.T) {
	Init()
	appSvcMock1 := utils.NewApplicationServiceMock(make(map[string]string)).AppService
	mockCreator := new(svcmocks.MockAppServiceCreator)
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeEventPublisherServiceKey, &bmcmodel.HedgeEvent{}).Return(appSvcMock1, true)
	appServiceCreator = mockCreator

	dBMockClient := redismock.MockDBClient{}
	redis.DBClientImpl = &dBMockClient
	dBMockClient.On("GetDbClient", mock.Anything).Return(redis.DBClientImpl)

	mockHttpClient := utils.NewMockClient()
	client.Client = mockHttpClient

	appSvcMock1.On("LoadCustomConfig", mock.Anything, mock.Anything).Return(nil)
	appSvcMock1.On("ListenForCustomConfigChanges", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	appSvcMock1.On("SetDefaultFunctionsPipeline", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	appSvcMock1.On("Run").Return(errors.New("mocked error"))

	exitCalled := false
	exitCode := 0
	originalOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	main()
	assert.True(t, exitCalled, "os.Exit should be called on failure")
	assert.Equal(t, -1, exitCode, "os.Exit should be called with -1")
	osExit = originalOsExit
	appServiceCreator = nil
}

func Test_main_Failure1(t *testing.T) {
	Init()
	appSvcMock1 := utils.NewApplicationServiceMock(make(map[string]string)).AppService
	mockCreator := new(svcmocks.MockAppServiceCreator)
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeEventPublisherServiceKey, &bmcmodel.HedgeEvent{}).Return(appSvcMock1, true)
	appServiceCreator = mockCreator

	dBMockClient := redismock.MockDBClient{}
	redis.DBClientImpl = &dBMockClient
	dBMockClient.On("GetDbClient", mock.Anything).Return(redis.DBClientImpl)

	mockHttpClient := svcmocks.MockHTTPClient{}
	mockHttpClient.On("Do", mock.Anything).Return(nil, errors.New("error"))
	client.Client = &mockHttpClient

	appSvcMock1.On("LoadCustomConfig", mock.Anything, mock.Anything).Return(errors.New("mocked error"))
	appSvcMock1.On("ListenForCustomConfigChanges", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("mocked error"))
	appSvcMock1.On("SetDefaultFunctionsPipeline", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("mocked error"))
	appSvcMock1.On("Run").Return(nil)

	exitCalled := false
	exitCode := 0
	originalOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	main()
	assert.True(t, exitCalled, "os.Exit should be called on failure")
	assert.Equal(t, -1, exitCode, "os.Exit should be called with -1")
	osExit = originalOsExit
	appServiceCreator = nil
}

func TestProcessAppCustomSettingsConfigUpdates_Failure(t *testing.T) {
	u := utils.NewApplicationServiceMock(map[string]string{
		"EventTopic":    "BMCEvents",
		"HedgeAdminURL": "http://hedge-admin:480111",
		"RedisHost":     "edgex-redis",
		"RedisPort":     "6997"})
	mockedLoggingClient := u.AppService.LoggingClient()
	config := &conf.WritableConfig{}
	config.Writable.StoreAndForward = conf.StoreAndForwardConfig{
		Enabled:       false,
		RetryInterval: "5m",
		MaxRetryCount: 3,
	}
	app := eventPublisher{
		service:               u.AppService,
		lc:                    mockedLoggingClient,
		storeAndForwardConfig: config,
		mqttSender:            &service.MQTTSecretSender{},
	}

	u.AppService.On("LoadCustomConfig", mock.Anything, "Writable/StoreAndForward").Return(errors.New("load error"))
	app.ProcessAppCustomSettingsConfigUpdates(nil)
}

func TestProcessAppCustomSettingsConfigUpdates_Failure1(t *testing.T) {
	u := utils.NewApplicationServiceMock(map[string]string{
		"EventTopic":    "BMCEvents",
		"HedgeAdminURL": "http://hedge-admin:480111",
		"RedisHost":     "edgex-redis",
		"RedisPort":     "6997"})
	mockedLoggingClient := u.AppService.LoggingClient()
	config := &conf.WritableConfig{}
	config.Writable.StoreAndForward = conf.StoreAndForwardConfig{
		Enabled:       false,
		RetryInterval: "",
		MaxRetryCount: 0,
	}

	app := eventPublisher{
		service:               u.AppService,
		lc:                    mockedLoggingClient,
		storeAndForwardConfig: config,
		mqttSender:            &service.MQTTSecretSender{},
	}

	u.AppService.On("LoadCustomConfig", mock.Anything, "Writable/StoreAndForward").Return(nil)
	app.ProcessAppCustomSettingsConfigUpdates(nil)
}

func TestProcessAppCustomSettingsConfigUpdates_Success(t *testing.T) {
	u := utils.NewApplicationServiceMock(map[string]string{
		"EventTopic":    "BMCEvents",
		"HedgeAdminURL": "http://hedge-admin:480111",
		"RedisHost":     "edgex-redis",
		"RedisPort":     "6997"})
	mockedLoggingClient := u.AppService.LoggingClient()
	config := &conf.WritableConfig{}
	config.Writable.StoreAndForward = conf.StoreAndForwardConfig{
		Enabled:       true,
		RetryInterval: "5m",
		MaxRetryCount: 3,
	}

	mqttSender := service.MQTTSecretSender{
		PersistOnError: true,
	}
	app := eventPublisher{
		service:               u.AppService,
		lc:                    mockedLoggingClient,
		storeAndForwardConfig: config,
		mqttSender:            &mqttSender,
	}

	u.AppService.On("LoadCustomConfig", mock.Anything, "Writable/StoreAndForward").Return(nil)
	app.ProcessAppCustomSettingsConfigUpdates(nil)
	assert.Equal(t, false, app.mqttSender.GetPersistOnError())
}
