package service

import (
	"hedge/app-services/hedge-admin/internal/config"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
)

var (
	appConfig        *config.AppConfig
	mockedHttpClient *utils.MockClient
	mockUtils        *utils.HedgeMockUtils
)

func init() {
	appConfig = &config.AppConfig{
		ContentDir:                  "./contents",
		NodeId:                      "",
		NodeType:                    "NODE",
		NodeHostName:                "",
		ConsulURL:                   "localhost",
		MqttURL:                     "tcp://localhost:1883",
		DefaultNodeGroupName:        "",
		DefaultNodeGroupDisplayName: "",
		MaxImportExportFileSizeMB:   '5',
	}

	mockUtils = utils.NewApplicationServiceMock(map[string]string{
		"RedisHost":    "localhost",
		"RedisPort":    "6379",
		"RedisName":    "metadata",
		"RedisTimeout": "5000",
		"ConsulURL":    "localhost",
	})
}
