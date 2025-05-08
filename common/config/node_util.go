/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"hedge/common/client"
	"hedge/common/models"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var MaxRetryCount int

func init() {
	client.Client = &http.Client{}
	// make it 1 for unit test case only
	MaxRetryCount = 12
}

// When we don't know the node-id we call this method
func GetNodeTopicName(topicName string, service interfaces.ApplicationService) (string, error) {
	adminURLs, _ := service.GetAppSettingStrings("HedgeAdminURL")
	adminURL := adminURLs[0]
	if len(adminURL) == 0 {
		service.LoggingClient().Warn("adminURL has no value. defaulting to value" + client.HedgeAdminServiceName)
		adminURL = "http://" + client.HedgeAdminServiceName + ":48098"
	}
	restoreTopicName := false
	if strings.HasSuffix(topicName, "/#") {
		restoreTopicName = true
		topicName = strings.Trim(topicName, "/#")
	}

	topicUrl := adminURL + "/api/v3/node_mgmt/topic/" + topicName
	// Try for few mins to allow hedge-admin service to be up
	resp, err := GetHttpResponseWithRetry(service, topicUrl)
	if err != nil {
		service.LoggingClient().Error("Error getting topicName from hedge-admin")
		return "", err
	}

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var respStr string
	json.Unmarshal(respData, &respStr)
	if restoreTopicName {
		respStr += "/#"
	}
	return respStr, nil
}

func BuildTargetNodeTopicName(nodeId string, targetTopic string) (string, error) {
	if len(nodeId) == 0 {
		return "", fmt.Errorf("NodeId cannot be empty")
	}
	if len(targetTopic) == 0 {
		return "", fmt.Errorf("targetTopic cannot be empty")
	}
	prefix := os.Getenv("MESSAGEBUS_BASETOPICPREFIX")
	if prefix == "" {
		prefix = "hedge"
	}
	// TopicName we get is hedge/ModelDeploy
	// Need to build hedge/nodeId/ModelDeploy
	if strings.HasPrefix(targetTopic, prefix) {
		return strings.Replace(targetTopic, prefix, prefix+"/"+nodeId, 1), nil
	}

	return nodeId + "/" + targetTopic, nil
}

func GetNode(service interfaces.ApplicationService, name string) (models.Node, error) {
	var node models.Node
	adminURL, err := service.GetAppSetting("HedgeAdminURL")
	if err != nil || len(adminURL) == 0 {
		service.LoggingClient().Error("adminURL has no value.")
		return node, errors.New("adminURL has no value")
	}
	nodeUrl := adminURL + "/api/v3/node_mgmt/node/" + name

	resp, err := GetHttpResponseWithRetry(service, nodeUrl)
	if err != nil {
		service.LoggingClient().Error("Error getting node details from hedge-admin")
		return node, err
	}
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		service.LoggingClient().Error("Error parsing response body from hedge-admin")
		return node, err
	}
	resp.Body.Close()
	json.Unmarshal(respData, &node)
	service.LoggingClient().Debugf("Got node %v", node)
	return node, nil
}

func GetCurrentNodeIdAndHost(service interfaces.ApplicationService) (nodeId string, hostName string) {
	node, err := GetNode(service, "current")
	if err != nil {
		service.LoggingClient().Errorf("Error getting current node name %s", err.Error())
		return "", ""
	}

	service.LoggingClient().Infof("Current Node %v\n", node)
	return node.NodeId, node.HostName
}

func GetAllNodes(service interfaces.ApplicationService) ([]models.Node, error) {
	var node []models.Node
	adminURL, err := service.GetAppSetting("HedgeAdminURL")
	if err != nil || len(adminURL) == 0 {
		service.LoggingClient().Error("adminURL has no value.")
		return node, errors.New("adminURL has no value")
	}

	resp, err := Get(adminURL + "/api/v3/node_mgmt/node/all")
	if err != nil {
		service.LoggingClient().Error("Error getting node info from hedge-admin")
		return node, err
	}
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		service.LoggingClient().Error("Error parsing node info from hedge-admin")
		return node, err
	}
	json.Unmarshal(respData, &node)
	service.LoggingClient().Debugf("Got nodes %v", node)
	return node, nil
}

// Given a URL, gets the http get response accouting for the other service not having started, so retries
func GetHttpResponseWithRetry(service interfaces.ApplicationService, url string) (*http.Response, error) {
	adminRunning := false
	var resp *http.Response
	var err error
	for retryCount := 0; !adminRunning && retryCount < MaxRetryCount*5; retryCount++ { // trying 5 mins before exit
		resp, err = Get(url)
		if err != nil {
			if strings.Contains(err.Error(), "connect") {
				// retry after sleep
				service.LoggingClient().Warnf("unable to connect to hedge-admin, will retry  in 5 secs, %v", err)
				time.Sleep(5 * time.Second)
				continue
			} else {
				service.LoggingClient().Error("Error response from hedge-admin, %v", err)
				return nil, err
			}
		} else {
			adminRunning = true
			break
		}
	}

	if !adminRunning {
		service.LoggingClient().Error("Error response from hedge-admin, possibly after multiple retries, %v", err)
		return nil, err
	}
	return resp, err
}

func Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Client.Do(req)
}
