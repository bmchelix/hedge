/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"hedge/app-services/hedge-device-extensions/pkg/db/redis"
	metasvc "hedge/app-services/hedge-device-extensions/pkg/service"
	"hedge/app-services/meta-sync/functions"
	syncRedis "hedge/app-services/meta-sync/internal/db"
	"hedge/app-services/meta-sync/internal/state"
	"hedge/common/client"
	"hedge/common/config"
	"hedge/common/db"
	"os"
	"strconv"
)

var (
	lc          logger.LoggingClient
	service     interfaces.ApplicationService
	metaService *metasvc.MetaService
)

func main() {
	var ok bool
	service, ok = pkg.NewAppServiceWithTargetType(client.HedgeMetaSyncServiceKey, &state.State{})
	if !ok {
		fmt.Errorf("failed to start app service: %s\n", client.HedgeMetaSyncServiceKey)
		os.Exit(-1)
	}

	lc = service.LoggingClient()

	stateIntervalRaw, _ := service.GetAppSetting("StateIntervalMin")
	stateInterval, err := strconv.Atoi(stateIntervalRaw)
	if err != nil {
		lc.Errorf("getting state interval failed: %s", err.Error())
		os.Exit(-1)
	}

	eventIntervalRaw, _ := service.GetAppSetting("StateEventsIntervalSec")
	eventInterval, err := strconv.Atoi(eventIntervalRaw)
	if err != nil {
		lc.Errorf("getting event processing interval failed: %s", err.Error())
		os.Exit(-1)
	}

	nodeType, err := service.GetAppSetting("NodeType")
	if err != nil {
		lc.Errorf("getting node type failed: %s", err.Error())
		os.Exit(-1)
	}

	node, err := config.GetNode(service, "current")
	if err != nil {
		lc.Errorf("error getting current node from hedge-admin: error: %s", err.Error())
		return
	}

	hadminUrl, err := service.GetAppSetting("HedgeAdminURL")
	metadataUrl, err := service.GetAppSetting("MetaDataServiceUrl")
	deviceExtUrl, err := service.GetAppSetting("Device_Extn")

	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(service)
	dbClientExt := redis.NewDBClient(dbConfig, service.LoggingClient())
	dbClientSync := syncRedis.NewDBClient(dbConfig, service.LoggingClient())

	metaService = metasvc.NewMetaService(service, dbConfig, dbClientExt, metadataUrl, node.NodeId, hadminUrl)
	stateManager, err := state.NewManager(service, metaService, dbClientSync, node, nodeType, stateInterval, eventInterval, true)
	if err != nil {
		lc.Errorf("failed while creating state manager: %s", err.Error())
		os.Exit(-1)
	}

	stateSender, requestSender, dataSender, err := getMqttSenders()
	if err != nil {
		lc.Errorf("failed while creating mqtt senders: %s", err.Error())
		os.Exit(-1)
	}

	metaSync := functions.NewMetaSync(service, metaService, stateManager, requestSender, dataSender, hadminUrl, deviceExtUrl,
		config.GetPersistOnError(service), node.NodeId, nodeType)

	if err := service.AddFunctionsPipelineForTopics("meta-sync state", []string{"meta-sync-state"},
		metaSync.State, // check if message recipient match and validate if relevant
	); err != nil {
		lc.Errorf("meta-sync state pipeline setup failed: %v", err)
		os.Exit(-1)
	}

	if err := service.AddFunctionsPipelineForTopics("meta-sync request", []string{"meta-sync-request"},
		metaSync.StateRequest, // send requested missing data
	); err != nil {
		lc.Errorf("meta-sync request pipeline setup failed: %v", err)
		os.Exit(-1)
	}

	if err := service.AddFunctionsPipelineForTopics("meta-sync data", []string{"meta-sync-data"},
		metaSync.StateData, // update missing/updated state
	); err != nil {
		lc.Errorf("meta-sync data pipeline setup failed: %v", err)
		os.Exit(-1)
	}

	go broadcastState(stateManager, stateSender)

	err = service.Run()
	if err != nil {
		lc.Error("Run returned error: ", err.Error())
		os.Exit(-1)
	}
	os.Exit(0)
}

// send state for sync once trigger activated
func broadcastState(stateManager *state.Manager, stateSender *transforms.MQTTSecretSender) {
	notify := stateManager.Listen()
	for {
		select {
		case s := <-notify:
			stateSender.MQTTSend(service.BuildContext("meta-sync-correlation-id", "application/json"), s)
		}
	}
}

func getMqttSenders() (*transforms.MQTTSecretSender, *transforms.MQTTSecretSender, *transforms.MQTTSecretSender, error) {
	mqttConfig, err := config.BuildMQTTSecretConfig(service, "meta-sync-state", client.HedgeMetaSyncServiceKey)
	if err != nil {
		lc.Errorf("Exiting failed to fetch mgtt config: %s", err)
		return nil, nil, nil, err
	}

	stateSender := transforms.NewMQTTSecretSender(mqttConfig, false)

	mqttConfig, err = config.BuildMQTTSecretConfig(service, "meta-sync-request", client.HedgeMetaSyncServiceKey)
	if err != nil {
		lc.Errorf("failed to fetch mgtt config: %s", err.Error())
		return nil, nil, nil, err
	}

	requestSender := transforms.NewMQTTSecretSender(mqttConfig, false)

	mqttConfig, err = config.BuildMQTTSecretConfig(service, "meta-sync-data", client.HedgeMetaSyncServiceKey)
	if err != nil {
		lc.Errorf("Failed to fetch mgtt config: %s", err)
		return nil, nil, nil, err
	}

	dataSender := transforms.NewMQTTSecretSender(mqttConfig, false)

	return stateSender, requestSender, dataSender, nil
}
