//
// Copyright (c) 2019 Intel Corporation
// (c) Copyright 2020-2025 BMC Software, Inc.
//
// Contributors: BMC Software, Inc. - BMC Helix Edge
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package main

import (
	conf "hedge/app-services/hedge-event-publisher/config"
	"hedge/app-services/hedge-event-publisher/pkg/functions"
	"hedge/common/client"
	"hedge/common/config"
	"hedge/common/db"
	commModel "hedge/common/models"
	commService "hedge/common/service"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"os"
)

var (
	osExit            = os.Exit
	appServiceCreator commService.AppServiceCreator
)

type eventPublisher struct {
	lc                    logger.LoggingClient
	service               interfaces.ApplicationService
	appServiceCreator     commService.AppServiceCreator
	mqttSender            commService.MqttSender
	storeAndForwardConfig *conf.WritableConfig
}

func (app *eventPublisher) getEventPublisher() {
	if appServiceCreator == nil {
		appServiceCreator = &commService.AppService{}
	}
	svc, ok := appServiceCreator.NewAppServiceWithTargetType(client.HedgeEventPublisherServiceKey, &commModel.HedgeEvent{})
	if !ok {
		//service.LoggingClient().Error(fmt.Sprintf("SDK initialization failed: %v\n", err))
		fmt.Errorf("Failed to start App Service: %s\n", client.HedgeEventPublisherServiceKey)
		exitWrapper(-1)
	} else {
		app.service = svc
	}
}

func main() {
	eventPublisher := eventPublisher{}
	eventPublisher.getEventPublisher()
	eventPublisher.lc = eventPublisher.service.LoggingClient()

	eventTopic, err := eventPublisher.service.GetAppSetting("EventTopic")
	if err != nil {
		eventPublisher.lc.Error(err.Error())
		os.Exit(-1)
	}

	currentNodeId, currentHost := config.GetCurrentNodeIdAndHost(eventPublisher.service)
	if currentNodeId == "" {
		eventPublisher.lc.Error("Error getting current node details from hedge-admin, exiting")
		exitWrapper(-1)
	}

	eventPublisher.storeAndForwardConfig = &conf.WritableConfig{}
	if err := eventPublisher.service.LoadCustomConfig(eventPublisher.storeAndForwardConfig, "Writable/StoreAndForward"); err != nil {
		eventPublisher.lc.Errorf("failed load custom configuration: %s", err.Error())
		exitWrapper(-1)
	}

	mqttConfig, _ := config.BuildMQTTSecretConfig(eventPublisher.service, eventTopic, currentNodeId+"::"+client.HedgeEventPublisherServiceKey)
	mqttConfig.QoS = 0

	eventPublisher.mqttSender = commService.NewMQTTSecretSender(mqttConfig, eventPublisher.storeAndForwardConfig.Writable.StoreAndForward.Enabled)

	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(eventPublisher.service)

	// Listen for StoreAndForward_Enabled changes
	if err := eventPublisher.service.ListenForCustomConfigChanges(eventPublisher.storeAndForwardConfig, "Writable/StoreAndForward", eventPublisher.ProcessAppCustomSettingsConfigUpdates); err != nil {
		eventPublisher.lc.Errorf("unable to watch Writable/StoreAndForward configuration: %s", err.Error())
		exitWrapper(-1)
	}

	// 3) This is our pipeline configuration, the collection of functions to
	// execute every time an event is triggered.
	err = eventPublisher.service.SetDefaultFunctionsPipeline(
		// Check for duplicate against redis db so we donot eventPublisher the same event again, also enrich the event
		functions.NewEdgeEventCoordinator(dbConfig, currentHost).CheckWhetherToPublishEvent,
		eventPublisher.mqttSender.MQTTSend,
	)

	if err != nil {
		eventPublisher.lc.Error("setpipeline returned error: ", err.Error())
		exitWrapper(-1)
	}

	// 4) Lastly, we'll go ahead and tell the SDK to "start" and begin listening for events
	// to trigger the pipeline.
	err = eventPublisher.service.Run()
	if err != nil {
		eventPublisher.lc.Error("Run returned error: ", err.Error())
		exitWrapper(-1)
	}
	// Do any required cleanup here
	return // Return was added so test case for main works ok, otherwise tests panic with os.Exit(0)
	//os.Exit(0)
}

func exitWrapper(code int) {
	osExit(code)
}

func (app *eventPublisher) ProcessAppCustomSettingsConfigUpdates(rawWritableDataConfig interface{}) {
	updated := conf.WritableConfig{}
	if err := app.service.LoadCustomConfig(&updated, "Writable/StoreAndForward"); err != nil {
		app.lc.Errorf("failed load Writable/StoreAndForward configuration from Configuration Provider: %s", err.Error())
		return
	}
	lc := app.lc
	previousConfig := app.storeAndForwardConfig
	app.lc.Infof("previous StoreAndForward_Enabled value: %t", previousConfig.Writable.StoreAndForward.Enabled)
	app.storeAndForwardConfig = &updated

	if previousConfig.Writable.StoreAndForward.Enabled == updated.Writable.StoreAndForward.Enabled {
		lc.Info("no StoreAndForward_Enabled changes detected")
		return
	} else {
		lc.Infof("StoreAndForward_Enabled value changed to %t", updated.Writable.StoreAndForward.Enabled)
	}

	// the next time the pipeline runs, it will use the updated MqttSender
	app.mqttSender.SetPersistOnError(updated.Writable.StoreAndForward.Enabled)
	app.lc.Infof("app.MqttSender.PersistOnError value updated: %t", updated.Writable.StoreAndForward.Enabled)
}
