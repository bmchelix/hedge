package utils

import (
	"context"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	mocks3 "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces/mocks"
	mocks2 "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	hedgeErrors "hedge/common/errors"
	"strings"
)

type HedgeMockUtils struct {
	AppService         *mocks.ApplicationService
	AppSettings        map[string]string
	AppFunctionContext *mocks.AppFunctionContext
}

func NewApplicationServiceMock(appSettings map[string]string) *HedgeMockUtils {

	//{"nodeId":"Node-01","hostName":"Host-Name","isRemoteHost":false,"name":"Host","ruleEndPoint":"","workflowEndPoint":""}
	//{"nodeId":"Node-01","hostName":"Host-Name","isRemoteHost":false,"name":"Host","ruleEndPoint":"","workflowEndPoint":""}
	hedgeMockUtils := new(HedgeMockUtils)
	// Add mock for logs
	mockLogger := &mocks2.LoggingClient{}
	mockLogger.On("Debug", mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Debugf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Errorf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Errorf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Errorf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Errorf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Warnf", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Infof", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Infof", mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Infof", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Infof", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Trace", mock.Anything).Return()
	mockLogger.On("Tracef", mock.Anything, mock.Anything, mock.Anything).Return()

	mockAppService := &mocks.ApplicationService{}
	hedgeMockUtils.AppService = mockAppService
	mockAppService.On("LoggingClient").Return(mockLogger)
	mockAppService.On("AppContext").Return(context.Background())

	if appSettings == nil {
		hedgeMockUtils.AppSettings = make(map[string]string)
		hedgeMockUtils.AppService.On("GetAppSettingStrings", mock.Anything).Return([]string{}, errors.New("not found"))
	} else {
		for k, v := range appSettings {
			if strings.HasPrefix(v, "ERR:") {
				e := errors.New(v)
				hedgeMockUtils.AppService.On("GetAppSetting", k).Return("", e)
				hedgeMockUtils.AppService.On("GetAppSettingStrings", k).Return([]string{}, e)
			} else {
				hedgeMockUtils.AppService.On("GetAppSetting", k).Return(v, nil)
				hedgeMockUtils.AppService.On("GetAppSettingStrings", k).Return([]string{v}, nil)
			}
		}
	}

	hedgeMockUtils.AppService.On("GetAppSetting", mock.Anything).Return("", nil)
	hedgeMockUtils.AppService.On("GetAppSettingStrings", mock.Anything).Return([]string{}, nil)
	//hedgeMockUtils.AppService.On("SecretProvider", mock.Anything).Return()
	hedgeMockUtils.AppService.On("GetSecret", "redisdb", "username", "password").Return(map[string]string{"username": "username", "password": "password"}, nil)
	hedgeMockUtils.AppService.On("GetSecret", "dbconnection", "password").Return(map[string]string{"password": "password"}, nil)
	//config.Client = &MockClient{}, cannot do this otherwise we get into circular dependency

	// Also create AppFunctionContext
	ctx := &mocks.AppFunctionContext{}
	ctx.On("LoggingClient").Return(mockLogger)
	ctx.On("PipelineId").Return("erty-876trfv-dsdf")
	ctx.On("CorrelationID").Return("erty-876trfv-dsdf2")
	//ctx.On("ApplyValues",mock.Anything, mock.Anything).Return("anystring", nil)

	hedgeMockUtils.AppFunctionContext = ctx

	mockSecretProvider := mocks3.SecretProvider{}
	mockSecretProvider.On("GetSecret", mock.Anything, "username", "password").Return(map[string]string{"username": "username", "password": "password"}, nil)
	mockSecretProvider.On("GetSecret", "mbconnection").Return(map[string]string{}, nil)
	mockSecretProvider.On("GetSecret", "dbconnection", "password").Return(map[string]string{"password": "password"}, nil)
	mockSecretProvider.On("GetSecret", "vaultconnection").Return(make(map[string]string), nil)
	mockSecretProvider.On("GetSecret", "vaultconnectionOneKeyMap").Return(map[string]string{"secret_key": "secret_value"}, nil)
	mockSecretProvider.On("GetSecret", "vaultconnectionThreeKeysMap").Return(map[string]string{"key1": "unsealKey1", "key2": "unsealKey2", "key3": "unsealKey3"}, nil)
	mockSecretProvider.On("GetSecret", "vaultconnectionThreeKeysMap").Return(map[string]string{"key1": "unsealKey1", "key2": "unsealKey2", "key3": "unsealKey3"}, nil)
	mockSecretProvider.On("GetSecret", "vaultconnectionTwoKeysMap").Return(map[string]string{"key1": "unsealKey1", "key3": "unsealKey3"}, nil)
	mockSecretProvider.On("GetSecret", "vaultconnectionerror").Return(make(map[string]string), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	hedgeMockUtils.AppService.On("SecretProvider").Return(&mockSecretProvider)

	return hedgeMockUtils
}

func (m *HedgeMockUtils) InitMQTTSettings() {
	appSettings := make(map[string]string)
	appSettings["scheme"] = "tcp"
	appSettings["MqttServer"] = "clm-pun-xxxx"
	appSettings["MqttPort"] = "1883"
	appSettings["MqttAuthMode"] = "usernamepassword"
	appSettings["QoS"] = "0"
	for k, v := range appSettings {
		m.AppService.On("GetAppSetting", k).Return(v, nil)
	}
	m.AppService.On("GetAppSetting", "MqttSecretPath").Return("", errors.New("path error"))
}
