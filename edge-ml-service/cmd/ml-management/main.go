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
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"hedge/common/client"
	"hedge/common/db"
	"hedge/common/service"
	"hedge/common/telemetry"
	"hedge/edge-ml-service/internal/router"
	"hedge/edge-ml-service/internal/training/hedge"
	"hedge/edge-ml-service/pkg/db/redis"
	DTO "hedge/edge-ml-service/pkg/dto/config"
	mlEdgeService "hedge/edge-ml-service/pkg/helpers"
	"os"
)

var (
	serviceInt        interfaces.ApplicationService
	dbClient          redis.MLDbInterface
	appServiceCreator service.AppServiceCreator
	osExit            = os.Exit
)

func getAppService() {
	if appServiceCreator == nil {
		appServiceCreator = &service.AppService{}
	}
	svc, ok := appServiceCreator.NewAppServiceWithTargetType(client.HedgeMLManagementServiceKey, new(interface{}))
	if !ok {
		//service.LoggingClient().Error(fmt.Sprintf("SDK initialization failed: %v\n", err))
		fmt.Printf("Failed to start App Service: %s\n", client.HedgeMLManagementServiceKey)
		exitWrapper(-1)
	} else {
		serviceInt = svc
	}
}

func main() {
	// 1) First thing to do is to create an instance of the EdgeX SDK and initialize it.
	if serviceInt == nil {
		getAppService()
	}
	svc := serviceInt
	lc := svc.LoggingClient()

	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(svc)
	if dbClient == nil {
		dbClient = redis.NewDBClient(dbConfig)
	}

	appConfig := DTO.NewMLMgmtConfig()
	appConfig.LoadCoreAppConfigurations(svc)

	subscriber := mlEdgeService.NewModelEdgeStatusSubscriber(dbClient)

	var err error
	metricsManager, err := telemetry.NewMetricsManager(svc, client.HedgeMLManagementServiceName)
	if err != nil {
		lc.Error("Failed to create metrics manager. Returned error: ", err.Error())
		exitWrapper(-1)
	}

	if metricsManager != nil { // condition for testing
		router.NewRouter(svc, appConfig, dbClient, client.HedgeMLManagementServiceName, metricsManager.MetricsMgr).AddRoutes()
		metricsManager.Run()
	}

	subscribedTopics, err := svc.GetAppSettingStrings("SubscribeTopics")
	if err != nil {
		lc.Errorf("failed to retrieve subscribedTopic from configuration: %s", err.Error())
		os.Exit(-1)
	}
	lc.Infof("subscribedTopics: %v", subscribedTopics)

	err = svc.AddFunctionsPipelineForTopics("ml-mgmt-train-deploy-status-pipeline", subscribedTopics,
		hedge.HandleTrainingStatusUpdates,
		subscriber.RecordModelSyncNodeStatus,
	)

	if err != nil {
		lc.Errorf("AddFunctionsPipelineForTopics returned error: %s", err.Error())
		exitWrapper(-1)
	}
	// 5) Lastly, we'll go ahead and tell the SDK to "start"
	err = svc.Run()
	if err != nil {
		lc.Error("Run returned error: ", err.Error())
		exitWrapper(-1)
	}

	exitWrapper(0)
}

func exitWrapper(code int) {
	osExit(code)
}
