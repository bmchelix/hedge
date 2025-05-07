/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package db

import (
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

// VictoriaMetricsURL is not required for all scenarios as of now
type DatabaseConfig struct {
	RedisHost          string
	RedisPort          string
	RedisName          string
	RedisUsername      string
	RedisPassword      string
	VictoriaMetricsURL string
}

func NewDatabaseConfig() *DatabaseConfig {
	appConfig := new(DatabaseConfig)
	return appConfig
}

// Loads only database confgirations of redis db and Victoria db
func (dbConfig *DatabaseConfig) LoadAppConfigurations(service interfaces.ApplicationService) {

	redisHost, err := service.GetAppSetting("RedisHost")
	lc := service.LoggingClient()
	if err != nil {
		lc.Error(err.Error())
	}
	lc.Infof("RedisHost %s\n", redisHost)

	redisPort, err := service.GetAppSetting("RedisPort")
	if err != nil {
		lc.Error(err.Error())
	}
	lc.Infof("RedisPort %s\n", redisPort)

	redisName, err := service.GetAppSetting("RedisName")
	if err != nil {
		lc.Error(err.Error())
	}

	lc.Infof("RedisName %v, will read redisdb secret now", redisName)
	redisSecrets, err := service.SecretProvider().GetSecret("redisdb", "username", "password")
	// Refer to https://docs.edgexfoundry.org/2.0/microservices/application/AdvancedTopics/#secrets for more details
	// Also and example @ https://github.com/edgexfoundry/edgex-examples/tree/main/application-services/custom/secrets
	//redisSecrets, err  := service.GetSecret("secret/edgex/hedge-device-extensions", "redisdb")

	if err == nil {
		lc.Infof("Got the redisdb secret\n")
		dbConfig.RedisUsername = redisSecrets["username"]
		dbConfig.RedisPassword = redisSecrets["password"]
	} else {
		lc.Error(err.Error())
	}

	lc.Infof("RedisUserName %s\n", dbConfig.RedisUsername)

	victoriaMetricsURL, err := service.GetAppSetting("VictoriaMetricsURL")
	if err != nil {
		lc.Info("VictoriaMetricsURL config not found or it is not applicable")
	} else {
		lc.Infof("VictoriaMetricsURL %s\n", victoriaMetricsURL)
	}

	dbConfig.RedisHost = redisHost
	dbConfig.RedisPort = redisPort
	dbConfig.RedisName = redisName

	if victoriaMetricsURL != "" {
		dbConfig.VictoriaMetricsURL = victoriaMetricsURL
	}
}
