/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"hedge/common/client"
	"hedge/common/models"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/stretchr/testify/assert"
	"net/http"
	"reflect"
	"testing"
)

var (
	// GetDoFunc fetches the mock client's `Do` func
	//GetDoFunc func(req *http.Request) (*http.Response, error)
	appServiceMock *utils.HedgeMockUtils
	node2          models.Node
	nodeHttpClient *utils.MockClient
)

func init() {
	nodeHttpClient = utils.NewMockClient()
	//client := Client.(utils.MockClient)
	node2 = models.Node{
		NodeId:           "nodeId02",
		HostName:         "clm-aus-xyzxx",
		IsRemoteHost:     true,
		Name:             "clm-aus-xyzxx",
		RuleEndPoint:     "http://clm-aus-xyzxx:59887",
		WorkFlowEndPoint: "http://clm-aus-xyzxx:1880",
	}
	nodeHttpClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/node/all", http.MethodGet, []models.Node{utils.Node1, node2}, 200, nil)
	nodeHttpClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/node/current", http.MethodGet, utils.Node1, 200, nil)

	// Below step is very important as we override the default implementation of HTTP client with this mock impl
	client.Client = nodeHttpClient

	appServiceMock = utils.NewApplicationServiceMock(map[string]string{"HedgeAdminURL": "http://hedge-admin:48000"})

}

// Test the error scenario when hedge-admin doesn't respond till, this takes few mins,other scenarios will get tested by other test cases
func TestGetHttpResponseWithRetry_adminNotRunning(t *testing.T) {
	nodeHttpClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/error", http.MethodGet, nil, 502, errors.New("connect dial issue"))
	MaxRetryCount = 1
	_, err := GetHttpResponseWithRetry(appServiceMock.AppService, "/api/v3/node_mgmt/error")
	assert.ErrorContainsf(t, err, "connect", err.Error())
}

func TestGetNode(t *testing.T) {
	nodeHttpClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/node/nodeId02", http.MethodGet, node2)
	node, err := GetNode(appServiceMock.AppService, "nodeId02")
	assert.NoError(t, err)
	assert.Equal(t, node2, node)

	// Test for error when HedgeAdminUrl is not set in configuration
	serviceWithNoConfig := utils.NewApplicationServiceMock(nil)
	_, err = GetNode(serviceWithNoConfig.AppService, "nodeId02")
	assert.ErrorContains(t, err, "adminURL has no value")
}

func TestGetCurrentNodeIdAndHost(t *testing.T) {
	nodeHttpClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/node/current", http.MethodGet, utils.Node1)
	nodeId, hostName := GetCurrentNodeIdAndHost(appServiceMock.AppService)
	assert.Equal(t, nodeId, utils.Node1.NodeId)
	assert.Equal(t, hostName, utils.Node1.HostName)
}

func TestBuildTargetNodeTopicName(t *testing.T) {
	nodeHttpClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/node/nodeId02", http.MethodGet, node2)
	topicName, err := BuildTargetNodeTopicName("nodeId02", "hedge/BMCCommands")
	assert.NoError(t, err)
	assert.Equal(t, topicName, "hedge/nodeId02/BMCCommands")

	topicName, err = BuildTargetNodeTopicName("nodeId02", "BMCCommands")
	assert.NoError(t, err)
	assert.Equal(t, topicName, "nodeId02/BMCCommands")

	_, err = BuildTargetNodeTopicName("", "BMCCommands")
	assert.ErrorContains(t, err, "empty")

	_, err = BuildTargetNodeTopicName("nodeId02", "")
	assert.ErrorContains(t, err, "empty")
	//assert.Equal(t, topicName, "nodeId02/BMCCommands")
}

func TestGetAllNodes(t *testing.T) {
	type args struct {
		service interfaces.ApplicationService
	}

	nodes := []models.Node{utils.Node1, node2}

	serviceWithNoConfig := utils.NewApplicationServiceMock(nil)

	tests := []struct {
		name        string
		args        args
		wantedNodes []models.Node
		wantErr     bool
	}{
		{name: "GetAllNodes_ValidAdminUrl", args: args{service: appServiceMock.AppService}, wantedNodes: nodes, wantErr: false},
		{name: "GetAllNodes_MissingAdminUrl", args: args{service: serviceWithNoConfig.AppService}, wantedNodes: nil, wantErr: true},
	}
	///api/v3/node_mgmt/node/all
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := GetAllNodes(tt.args.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(nodes, tt.wantedNodes) {
				t.Errorf("GetAllNodes() got = %v, want %v", nodes, tt.wantedNodes)
			}
		})
	}
}

func TestGetNodeTopicName(t *testing.T) {
	type args struct {
		topicName string
		service   interfaces.ApplicationService
	}

	nodeHttpClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/topic/BMCEvents", http.MethodGet, "BMCEvents")
	nodeHttpClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/topic/BMCEvents/#", http.MethodGet, "BMCEvents/#")

	tests := []struct {
		name      string
		args      args
		topicName string
		wantErr   bool
	}{
		{name: "GetNodeTopicName_TopicNotShared", args: args{service: appServiceMock.AppService, topicName: "BMCEvents"}, topicName: "BMCEvents", wantErr: false},
		{name: "GetNodeTopicName_TopicShared", args: args{service: appServiceMock.AppService, topicName: "BMCEvents/#"}, topicName: "BMCEvents/#", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetNodeTopicName(tt.args.topicName, tt.args.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNodeTopicName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.topicName {
				t.Errorf("GetNodeTopicName() got = %v, want %v", got, tt.topicName)
			}
		})
	}
}
