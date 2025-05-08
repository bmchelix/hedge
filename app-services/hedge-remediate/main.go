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
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"hedge/app-services/hedge-remediate/functions"
	"hedge/common/client"
	"hedge/common/config"
	comService "hedge/common/config"
	"hedge/common/models"
	"hedge/common/telemetry"
	"os"
)

func main() {

	service, ok := pkg.NewAppServiceWithTargetType(client.HedgeRemediateServiceKey, &models.Command{}) // Key used by Registry (Aka Consul))
	if !ok {
		os.Exit(-1)
	}
	lc := service.LoggingClient()

	subscribeTopic := "commands"
	node, err := comService.GetNode(service, "current")
	if err != nil {
		lc.Errorf("Could not get the node details and hence the nodeType, exiting")
		os.Exit(-1)
	}
	if !node.IsRemoteHost {
		subscribeTopic, err = comService.GetNodeTopicName("commands", service)
		if err != nil {
			lc.Errorf("Could not get topic name, exiting")
			os.Exit(-1)
		}
	}
	lc.Infof("Subscribing to topic: %s", subscribeTopic)

	hedge_event_publisher_trigger_url, err := service.GetAppSetting("EventPipelineTriggerURL")
	if err != nil {
		lc.Errorf("EventPipelineTriggerURL not configured, exiting")
		os.Exit(-1)
	}

	metricsManager, err := telemetry.NewMetricsManager(service, client.HedgeRemediateServiceName)
	if err != nil {
		lc.Error("Failed to create metrics manager. Returned error: ", err.Error())
		os.Exit(-1)
	}

	cmdExecutionService := functions.NewCommandExecutionService(service, client.HedgeRemediateServiceName, metricsManager.MetricsMgr, node.HostName)
	err = service.AddFunctionsPipelineForTopics("hedge-remediate-pipeline", []string{subscribeTopic, "commands"},
		cmdExecutionService.IdentifyCommandAndExecute,
		transforms.NewHTTPSender(hedge_event_publisher_trigger_url, "application/json", config.GetPersistOnError(service)).HTTPPost,
	)

	if err != nil {
		lc.Error("hedge-remediate functions pipeline failed: " + err.Error())
		os.Exit(-1)
	}

	metricsManager.Run()

	// Check and if required restore it: Probably only for testing, it sends the command to command topic
	/*	service.AddRoute("/api/v1/helix/remediation", func(writer http.ResponseWriter, req *http.Request) {
		// Direct API call for Command , Device Service publish to MQTT topic BMCCommandLog
		mqttService.RestRemediation(writer, req)
	}, http.MethodPost)*/

	err = service.Run()
	if err != nil {
		lc.Error("Run returned error: ", err.Error())
		os.Exit(-1)
	}

	os.Exit(0)
}
