/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"errors"
	"testing"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"hedge/common/client"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	redisMock "hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
)

var (
	appSvcMock *mocks.ApplicationService
	configs    map[string]string
)

func Init() {
	configs = map[string]string{
		"DataStore_Provider":    "Hedge",
		"MetaDataServiceUrl":    "http://localhost:59881",
		"RedisHost":             "edgex-redis",
		"ModelDeployTopic":      "ModelDeployTopic",
		"HedgeAdminURL":         "http://localhost:48098",
		"ModelDownloadEndpoint": "https://clm-aus-vi37qf.bmc.com/hedge/api/v3/ml_management/model/modelFile",
	}
	appSvcMock = utils.NewApplicationServiceMock(map[string]string{}).AppService
}

func TestGetAppService(t *testing.T) {
	Init()
	mockCreator := &svcmocks.MockAppServiceCreator{}
	appServiceCreator = mockCreator
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeMLManagementServiceKey, new(interface{})).Return(appSvcMock, true)

	getAppService()
	assert.Equal(t, appSvcMock, serviceInt, "Service should be assigned correctly")
	serviceInt = nil
}

func TestGetAppService_Failure(t *testing.T) {
	Init()
	mockCreator := new(svcmocks.MockAppServiceCreator)
	mockCreator.On("NewAppServiceWithTargetType", client.HedgeMLManagementServiceKey, new(interface{})).Return(appSvcMock, false)
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
}

func Test_main_Success(t *testing.T) {
	dbClient = &redisMock.MockMLDbInterface{}

	appSvcMock1 := utils.NewApplicationServiceMock(configs).AppService
	/*appSvcMock1.On("ADE_TENANT_URL").Return("ADE_TENANT_URL")
	appSvcMock1.On("ADE_TENANT_ID").Return("ADE_TENANT_ID")
	appSvcMock1.On("ADE_ACCESS_KEY").Return("ADE_ACCESS_KEY")
	appSvcMock1.On("ADE_ACCESS_SECRET_KEY").Return("ADE_ACCESS_SECRET_KEY")*/

	appSvcMock1.On("AddRoute", mock.Anything, mock.AnythingOfType("func(http.ResponseWriter, *http.Request)"), mock.Anything).Return(nil).Once()
	appSvcMock1.On("AddFunctionsPipelineForTopics", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	appSvcMock1.On("Run").Return(nil)
	serviceInt = appSvcMock1

	exitCalled := false
	exitCode := 0
	originalOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	main()
	assert.True(t, exitCalled, "os.Exit should be called on failure")
	assert.Equal(t, 0, exitCode, "os.Exit should be called with 0")
	serviceInt = nil
	osExit = originalOsExit
}

func Test_main_Failure(t *testing.T) {
	dbClient = &redisMock.MockMLDbInterface{}

	appSvcMock1 := utils.NewApplicationServiceMock(configs).AppService
	/*	appSvcMock1.On("ADE_TENANT_URL").Return("ADE_TENANT_URL")
		appSvcMock1.On("ADE_TENANT_ID").Return("ADE_TENANT_ID")
		appSvcMock1.On("ADE_ACCESS_KEY").Return("ADE_ACCESS_KEY")
		appSvcMock1.On("ADE_ACCESS_SECRET_KEY").Return("ADE_ACCESS_SECRET_KEY")*/

	appSvcMock1.On("AddRoute", mock.Anything, mock.AnythingOfType("func(http.ResponseWriter, *http.Request)"), mock.Anything).Return(nil).Once()
	appSvcMock1.On("AddFunctionsPipelineForTopics", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("mocked error"))
	appSvcMock1.On("Run").Return(errors.New("mocked error"))
	serviceInt = appSvcMock1

	exitCalled := false
	exitCode := 0
	originalOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	main()
	assert.True(t, exitCalled, "os.Exit should be called on failure")
	assert.Equal(t, 0, exitCode, "os.Exit should be called with 0")
	serviceInt = nil
	osExit = originalOsExit
}
