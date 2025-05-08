/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"fmt"
	"hedge/common/client"
	"hedge/common/config"
	"hedge/common/service"
	"hedge/edge-ml-service/pkg/dto/ml_model"
	ml_agent "hedge/edge-ml-service/pkg/ml-agent"
	"os"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

var (
	serviceInt        interfaces.ApplicationService
	mlAgentConfig     ml_agent.MLEdgeAgentConfigInterface
	appServiceCreator service.AppServiceCreator
	osExit            = os.Exit
)

func setAppService() {
	if appServiceCreator == nil {
		appServiceCreator = &service.AppService{}
	}
	svc, ok := appServiceCreator.NewAppServiceWithTargetType(client.HedgeMLEdgeAgentServiceKey, &ml_model.ModelDeployCommand{})
	if !ok {
		fmt.Errorf("Failed to start App Service: %s\n", client.HedgeMLEdgeAgentServiceKey)
		exitWrapper(-1)
	} else {
		serviceInt = svc
	}
}

func setAgentConfig() {
	agent, err := ml_agent.NewMLEdgeAgentConfig(serviceInt)
	if err != nil {
		fmt.Errorf("Failed to create Ml Agent Config: %s\n", err)
		exitWrapper(-1)
	}
	if mlAgentConfig == nil {
		mlAgentConfig = agent
	}
	mlAgentConfig.SetDbClient(serviceInt)
	ReinitializeEndpoint := mlAgentConfig.GetReinitializeEndpoint()
	mlAgentConfig.SetHttpSender(ReinitializeEndpoint)
}

func main() {

	setAppService()
	setAgentConfig()
	serviceInt.LoggingClient().Infof("Re-initalize Endpoint: %v, modelDownloadEndPoint: %s", mlAgentConfig.GetReinitializeEndpoint(), mlAgentConfig.GetModelDownloadEndpoint())

	// Get the node name
	fqnModelDeployCmdTopicName, err := config.GetNodeTopicName(mlAgentConfig.GetModelDeployTopic(), serviceInt)
	if err != nil {
		serviceInt.LoggingClient().Errorf("node specific topic name required, got error: error:%v", err)
		exitWrapper(-1)
	}

	modelDeployCmdTopics := []string{fqnModelDeployCmdTopicName}
	serviceInt.LoggingClient().Infof("subscribing to modelDeploy topic : %s", fqnModelDeployCmdTopicName)
	err = serviceInt.AddFunctionsPipelineForTopics("ml-edge-model", modelDeployCmdTopics,
		mlAgentConfig.DownloadModel,
		mlAgentConfig.DeployInferenceImplementation,
		mlAgentConfig.ExecuteMLCommands,
	)

	if err != nil {
		serviceInt.LoggingClient().Errorf("AddFunctionsPipelineForTopics returned error: %s", err.Error())
		exitWrapper(-1)
	}

	if err := serviceInt.Run(); err != nil {
		serviceInt.LoggingClient().Errorf("Run returned error: %s", err.Error())
		exitWrapper(-1)
	}
	return

}

func exitWrapper(code int) {
	osExit(code)
}
