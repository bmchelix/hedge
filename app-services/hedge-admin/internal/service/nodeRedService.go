/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/app-services/hedge-admin/internal/config"
	"hedge/app-services/hedge-admin/models"
	"hedge/common/client"
	hedgeErrors "hedge/common/errors"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"io"
	"net/http"
	"os"
)

type NodeRedService struct {
	service   interfaces.ApplicationService
	appConfig *config.AppConfig
}

func NewNodeRedService(service interfaces.ApplicationService, appConfig *config.AppConfig) *NodeRedService {
	nodeRedService := new(NodeRedService)
	nodeRedService.service = service
	nodeRedService.appConfig = appConfig
	return nodeRedService
}

func (nodeRedService *NodeRedService) SetHttpClient() {
	if client.Client == nil {
		client.Client = &http.Client{}
	}
}

func (nodeRedService *NodeRedService) ImportNodeRedWorkFlows(contentData models.ContentData, installOpt bool) hedgeErrors.HedgeError {

	mqttCreds, err := nodeRedService.service.SecretProvider().GetSecret("mbconnection")
	if err != nil {
		nodeRedService.service.LoggingClient().Errorf("Failed fetching the mqtt credentials from Vault. Err: %v", err)
	}

	_, ok := mqttCreds["username"]
	if !ok {
		nodeRedService.service.LoggingClient().Warnf("Mqtt username returned empty from Vault")
		mqttCreds["username"] = ""
	}
	_, ok = mqttCreds["password"]
	if !ok {
		nodeRedService.service.LoggingClient().Warnf("Mqtt password returned empty from Vault")
		mqttCreds["password"] = ""
	}

	if contentData.TargetNodes == nil || len(contentData.TargetNodes) == 0 {
		contentData.TargetNodes = []string{"hedge-node-red"}
	}

	hErr := nodeRedService.createWorkFlows(contentData, mqttCreds, installOpt)
	if hErr != nil {
		nodeRedService.service.LoggingClient().Warn(hErr.Error())
		return hErr
	}
	return nil
}

func (nodeRedService *NodeRedService) createWorkFlows(contentData models.ContentData, mqttCreds map[string]string, installOpt bool) hedgeErrors.HedgeError {
	errorMessage := "Error creating workflows"

	nodeRedDir := nodeRedService.appConfig.ContentDir + "/" + "node-red"
	if installOpt {
		nodeRedDir = nodeRedDir + "/workFlows"
	} else {
		// if install optional flows == false, then install the mqtt configure flow only
		nodeRedDir = nodeRedDir + "/configure"
	}

	var finalResult []byte

	dirEntries, err := os.ReadDir(nodeRedDir)
	if err != nil {
		nodeRedService.service.LoggingClient().Warn(err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %s", errorMessage, "Error reading directory"))
	}
	length := len(dirEntries)
	byteComma := []byte(",")

	for index, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			continue
		}
		nodeRedService.service.LoggingClient().Infof("Importing File: %s", dirEntry.Name())
		fileName := nodeRedDir + "/" + dirEntry.Name()
		byteData, err := os.ReadFile(fileName)
		if err != nil {
			nodeRedService.service.LoggingClient().Warnf("Error reading file: %s, error: %v", fileName, err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("%s: %s %s", errorMessage, "Error reading file", fileName))
		}
		// Somehow we need to merge all workflows, otherwise every one overwrites the previous one
		startBracketIndex := bytes.Index(byteData, []byte("["))
		endBracketIndex := bytes.LastIndex(byteData, []byte("]"))
		if index == 0 {
			if index != length-1 {
				byteData = byteData[:endBracketIndex-1]
				byteData = append(byteData, byteComma...)
			}
		} else if index == length-1 {
			byteData = byteData[startBracketIndex+1:]
		} else {
			byteData = byteData[startBracketIndex+1 : endBracketIndex-1]
			byteData = append(byteData, byteComma...)
		}
		finalResult = append(finalResult, byteData...)
	}

	// add Mqtt credentials
	finalResult = nodeRedService.appendMqttCredentials(finalResult, mqttCreds)
	if err := nodeRedService.exportNodeRedWorkflow(contentData, finalResult); err != nil {
		nodeRedService.service.LoggingClient().Errorf("Error creating workflows: export failed: %v", err)
		return err
	}

	return nil
}

func (nodeRedService *NodeRedService) appendMqttCredentials(flowBytes []byte, mqttCreds map[string]string) []byte {
	var flowMaps []map[string]interface{}
	err := json.Unmarshal(flowBytes, &flowMaps)
	if err != nil {
		nodeRedService.service.LoggingClient().Errorf("Failed during unmarshalling of nodered flow. %v", err)
		return flowBytes
	}

	for index, flowMap := range flowMaps {
		if flowMap["type"] == "mqtt-broker" {
			flowMaps[index]["broker"] = nodeRedService.appConfig.MqttURL
			flowMaps[index]["credentials"] = map[string]string{"user": mqttCreds["username"], "password": mqttCreds["password"]}
		}
	}

	retFlowBytes, err := json.Marshal(flowMaps)
	if err != nil {
		nodeRedService.service.LoggingClient().Errorf("Append MQTT credentials failed: Failed during marshalling of nodered flow map. %v", err)
		return flowBytes
	}

	return retFlowBytes
}

func (nodeRedService *NodeRedService) exportNodeRedWorkflow(contentData models.ContentData, workflowBytes []byte) hedgeErrors.HedgeError {
	const mqttNodeId = "bf47c3db.d1bc"

	errorMessage := "Error exporting node red workflow"

	for _, target := range contentData.TargetNodes {
		url := "http://" + target + ":1880" + "/" + "flows"
		nodeRedService.service.LoggingClient().Infof("Node red node URL to import: %s\n", url)
		// First GET flows to check if already imported
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			nodeRedService.service.LoggingClient().Warnf("Error creating http request to import node-red workflows to %s, %v", url, err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		nodeRedService.SetHttpClient()
		res, err := client.Client.Do(req)
		responseBody, err := io.ReadAll(res.Body)
		var mapBody []map[string]string
		json.Unmarshal(responseBody, &mapBody)
		for i := range mapBody {
			for key, val := range mapBody[i] {
				// Don't overwrite the flows if user has created additional flows (in addition to the OOB mqtt-broker flow with id=bf47c3db.d1bc)
				if key == "id" && val == mqttNodeId && len(mapBody) != 1 {
					errStr := "Flow " + mqttNodeId + " already imported and new flows created, exiting without overriding"
					nodeRedService.service.LoggingClient().Warnf("%s: %s", errorMessage, errStr)
					return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("%s: %s", errorMessage, errStr))
				}
			}
		}
		res.Body.Close()

		request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(workflowBytes))
		if err != nil {
			nodeRedService.service.LoggingClient().Warnf("Error creating http request to import node-red workflows to %s, %v", url, err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		}
		request.Header.Set("Content-type", "application/json")
		request.Header.Set("Node-RED-Deployment-Type", "nodes")
		// Refer to https://nodered.org/docs/api/admin/methods/post/flows/ for API details

		resp, err := client.Client.Do(request)

		if err != nil {
			nodeRedService.service.LoggingClient().Warnf("Failed to import node-red workflows with error: %v", err)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
		} else {
			nodeRedService.service.LoggingClient().Infof("Node-red import response: %s\n", resp.Status)
		}

		resp.Body.Close()
	}
	return nil
}
