/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/app-services/hedge-admin/db"
	"hedge/app-services/hedge-admin/internal/config"
	mod "hedge/app-services/hedge-admin/models"
	"hedge/common/client"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	redismocks "hedge/mocks/hedge/app-services/hedge-admin/db"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	mocks3 "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var (
	flds                        fields
	dBMockClient, dBMockClient1 redismocks.MockRedisDbClient
	dsProtocols                 models.DeviceServiceProtocols
	flds4, flds5                fields1
	flds6                       fields2
)

type fields struct {
	service   interfaces.ApplicationService
	client    client.HTTPClient
	dbClient  db.RedisDbClient
	appConfig *config.AppConfig
}

type fields1 struct {
	service  interfaces.ApplicationService
	dbClient db.RedisDbClient
}

type fields2 struct {
	service   interfaces.ApplicationService
	appConfig *config.AppConfig
	client    client.HTTPClient
}

func Init() {
	mockedHttpClient = utils.NewMockClient()
	client.Client = mockedHttpClient

	dBMockClient = redismocks.MockRedisDbClient{}
	dBMockClient1 = redismocks.MockRedisDbClient{}

	flds = fields{
		service:   mockUtils.AppService,
		dbClient:  &dBMockClient,
		appConfig: appConfig,
	}
	flds4 = fields1{
		service:  mockUtils.AppService,
		dbClient: &dBMockClient,
	}
	flds5 = fields1{
		service:  mockUtils.AppService,
		dbClient: &dBMockClient1,
	}
	flds6 = fields2{
		service:   mockUtils.AppService,
		appConfig: appConfig,
	}

	dsProtocols = models.DeviceServiceProtocols{
		"device1": {
			{
				ProtocolName:       "HTTP",
				ProtocolProperties: []string{"Port: 80", "Secure: false"},
			},
		},
		"device2": {
			{
				ProtocolName:       "CoAP",
				ProtocolProperties: []string{"Port: 5683", "Secure: true"},
			},
		},
	}
	Cleanup()
}

func TestDeviceService_getAllServices(t *testing.T) {
	Init()
	tests := []struct {
		name    string
		fields  fields
		want    []string
		wantErr bool
	}{
		{"getAllServices - Failed1", flds, nil, true},
		{"getAllServices - Failed2", flds, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if strings.Contains(tt.name, "Failed2") {
				tokenFilePath := "test_token_file.txt"
				os.Setenv("HEDGE_CONSUL_TOKEN_FILE", tokenFilePath)
				ioutil.WriteFile("test_token_file.txt", []byte("12345678-1234-1234-1234-123456789012"), 0644)

				mockedHttpClient.RegisterExternalMockRestCall("", "GET", []byte("]"), 200, nil)
			}

			deviceService := DeviceService{
				service:   tt.fields.service,
				dbClient:  tt.fields.dbClient,
				appConfig: tt.fields.appConfig,
			}
			got, err := deviceService.getAllServices()
			if (err != nil) != tt.wantErr {
				t.Errorf("getAllServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getAllServices() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceService_GetServiceProtocolsFromConsul(t *testing.T) {
	Init()
	tokenFilePath := "test_token_file.txt"
	os.Setenv("HEDGE_CONSUL_TOKEN_FILE", tokenFilePath)
	ioutil.WriteFile("test_token_file.txt", []byte("12345678-1234-1234-1234-123456789012"), 0644)

	consulKVs := []models.ConsulKV{
		{
			Key:         "edgex/devices/2.0/device-virtual/Hedge/Protocols/0/ProtocolName",
			Value:       base64.StdEncoding.EncodeToString([]byte("Hello, base64!")),
			LockIndex:   0,
			Flags:       0,
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}

	type args struct {
		devServiceName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    models.DeviceProtocols
		wantErr bool
	}{
		{"GetServiceProtocolsFromConsul - Passed", flds, args{"test-devService-name"}, models.DeviceProtocols{{ProtocolName: "Hello, base64!"}}, false},
		{"GetServiceProtocolsFromConsul - Failed1", flds, args{"test-devService-name"}, models.DeviceProtocols{}, true},
		{"GetServiceProtocolsFromConsul - Failed2", flds, args{"test-devService-name"}, models.DeviceProtocols{}, true},
		{"GetServiceProtocolsFromConsul - Failed3", flds, args{"test-devService-name"}, models.DeviceProtocols{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if strings.Contains(tt.name, "Passed") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", consulKVs, 404, nil)
			}
			if strings.Contains(tt.name, "Failed1") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", []byte{}, 400, nil)
			}
			if strings.Contains(tt.name, "Failed2") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", consulKVs, 400, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
			}
			if strings.Contains(tt.name, "Failed3") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", []byte("]"), 404, nil)
			}

			deviceService := DeviceService{
				service:   tt.fields.service,
				dbClient:  tt.fields.dbClient,
				appConfig: tt.fields.appConfig,
			}
			got, err := deviceService.GetServiceProtocolsFromConsul(tt.args.devServiceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetServiceProtocolsFromConsul() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetServiceProtocolsFromConsul() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceService_InitializeDeviceServices(t *testing.T) {
	Init()
	tokenFilePath := "test_token_file.txt"
	os.Setenv("HEDGE_CONSUL_TOKEN_FILE", tokenFilePath)
	ioutil.WriteFile("test_token_file.txt", []byte("12345678-1234-1234-1234-123456789012"), 0644)

	consulKVs := []models.ConsulKV{
		{
			Key:         "key1/path",
			Value:       "value1",
			LockIndex:   0,
			Flags:       0,
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}
	consulKVs1 := []models.ConsulKV{
		{
			Key:         "edgex/devices/2.0/device-virtual/Hedge/Protocols/0/ProtocolName",
			Value:       base64.StdEncoding.EncodeToString([]byte("Hello, base64!")),
			LockIndex:   0,
			Flags:       0,
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}
	dBMockClient.On("SaveProtocols", mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError,
		"Error saving protocol"))

	type args struct {
		inServiceName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    models.DeviceServiceProtocols
		wantErr bool
	}{
		{"InitializeDeviceServices - Passed1", flds, args{"test-inService-name"}, models.DeviceServiceProtocols{}, false},
		{"InitializeDeviceServices - Passed2", flds, args{""}, models.DeviceServiceProtocols{}, false},
		{"InitializeDeviceServices - Passed3", flds, args{"test/inService/name"}, models.DeviceServiceProtocols{}, false},
		{"InitializeDeviceServices - Failed1", flds, args{""}, nil, true},
		{"InitializeDeviceServices - Failed2", flds, args{"test-inService-name"}, nil, true},
		{"InitializeDeviceServices - Failed3", flds, args{"test-inService-name"}, nil, true},
		{"InitializeDeviceServices - Failed4", flds, args{""}, nil, true},
		{"InitializeDeviceServices - Failed4", flds, args{"test/name"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if strings.Contains(tt.name, "Passed1") || strings.Contains(tt.name, "Passed2") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", []string{}, 404, nil)
			}
			if strings.Contains(tt.name, "Passed3") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", consulKVs1, 200, nil)
			}
			if strings.Contains(tt.name, "Failed1") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", []byte{}, 400, nil)
			}
			if strings.Contains(tt.name, "Failed2") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", dsProtocols, 200, nil)
			}
			if strings.Contains(tt.name, "Failed3") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", consulKVs, 200, nil)
			}
			if strings.Contains(tt.name, "Failed4") {
				mockedHttpClient.RegisterExternalMockRestCall("", "GET", []byte{}, 400, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
			}

			deviceService := DeviceService{
				service:   tt.fields.service,
				dbClient:  tt.fields.dbClient,
				appConfig: tt.fields.appConfig,
			}
			got, err := deviceService.InitializeDeviceServices(tt.args.inServiceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitializeDeviceServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InitializeDeviceServices() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeviceService_RegisterServices(t *testing.T) {
	Init()
	tokenFilePath := "test_token_file.txt"
	os.Setenv("HEDGE_CONSUL_TOKEN_FILE", tokenFilePath)
	ioutil.WriteFile("test_token_file.txt", []byte("12345678-1234-1234-1234-123456789012"), 0644)

	type args struct {
		dsProtocols models.DeviceServiceProtocols
	}
	args1 := args{
		dsProtocols: dsProtocols,
	}
	args2 := args{
		dsProtocols: models.DeviceServiceProtocols{
			"device1": {
				{
					ProtocolName:       "",
					ProtocolProperties: []string{},
				},
			},
		},
	}
	args3 := args{
		dsProtocols: models.DeviceServiceProtocols{
			"device1": {
				{
					ProtocolName:       "HTTP",
					ProtocolProperties: nil,
				},
			},
		},
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"RegisterServices - Passed", flds, args1, false},
		{"RegisterServices - Failed1", flds, args2, true},
		{"RegisterServices - Failed2", flds, args1, true},
		{"RegisterServices - Failed3", flds, args1, true},
		{"RegisterServices - Failed4", flds, args3, true},
		{"RegisterServices - Failed5", flds, args1, true},
		{"RegisterServices - Failed6", flds, args1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if strings.Contains(tt.name, "Passed") || strings.Contains(tt.name, "Failed1") {
				mockedHttpClient.RegisterExternalMockRestCall("", "PUT", []byte("mocked response"), 200, nil)
			}
			if strings.Contains(tt.name, "Failed2") {
				mockedHttpClient.RegisterExternalMockRestCall("", "PUT", []byte("mocked response"), 400, nil)
			}
			if strings.Contains(tt.name, "Failed3") {
				mockedHttpClient.RegisterExternalMockRestCall("", "PUT", nil, 200, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
			}
			if strings.Contains(tt.name, "Failed4") || strings.Contains(tt.name, "Failed5") {
				mockedHttpClient.RegisterExternalMockRestCall("http://localhost/v1/kv/edgex/devices/2.0/device1/Hedge/Protocols/0/ProtocolName", "PUT", []byte("mocked response"), 200, nil)
				mockedHttpClient.RegisterExternalMockRestCall("http://localhost/v1/kv/edgex/devices/2.0/device1/Hedge/Protocols/0/ProtocolProperties", "PUT", nil, 400, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
			}
			if strings.Contains(tt.name, "Failed6") || strings.Contains(tt.name, "Failed7") {
				mockedHttpClient.RegisterExternalMockRestCall("http://localhost/v1/kv/edgex/devices/2.0/device1/Hedge/Protocols/0/ProtocolName", "PUT", []byte("mocked response"), 200, nil)
				mockedHttpClient.RegisterExternalMockRestCall("http://localhost/v1/kv/edgex/devices/2.0/device1/Hedge/Protocols/0/ProtocolProperties", "PUT", []byte("mocked response"), 400, nil)
			}

			deviceService := DeviceService{
				service:   tt.fields.service,
				dbClient:  tt.fields.dbClient,
				appConfig: tt.fields.appConfig,
			}
			if err := deviceService.RegisterServices(tt.args.dsProtocols); (err != nil) != tt.wantErr {
				t.Errorf("RegisterServices() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewDeviceService(t *testing.T) {
	//Init()
	//deviceService := NewDeviceService(u.AppService, appConfig)
	//if deviceService == nil {
	//	t.Error("Expected non-nil DeviceService, got nil")
	//	return
	//}
}

func TestNewNodeGroupService(t *testing.T) {
	//Init()
	//nodeGroupService := NewNodeGroupService(u.AppService)
	//if nodeGroupService == nil {
	//	t.Error("Expected non-nil NodeGroupService, got nil")
	//	return
	//}
	//if nodeGroupService.service != u.AppService {
	//	t.Error("Expected service to be set correctly in NodeGroupService")
	//	return
	//}
}

func TestNewNodeRedService(t *testing.T) {
	nodeRedService := NewNodeRedService(mockUtils.AppService, appConfig)
	if nodeRedService == nil {
		t.Error("Expected non-nil NodeRedService, got nil")
		return
	}
}

func TestNewRuleService(t *testing.T) {
	Init()
	NewRuleService(mockUtils.AppService, appConfig)
}

func TestNodeGroupService_AddNodeToDefaultGroup(t *testing.T) {
	Init()
	dBMockClient.On("FindNodeKey", mock.Anything).Return("parentGroupHashKey", nil)
	dBMockClient.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{}, nil)
	dBMockClient.On("SaveNodeGroup", mock.Anything, mock.Anything).Return("", nil)

	dBMockClient1.On("FindNodeKey", mock.Anything).Return("parentGroupHashKey", nil)
	dBMockClient1.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{}, nil)
	dBMockClient1.On("SaveNodeGroup", mock.Anything, mock.Anything).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "find node group failed"))

	dBMockClient2 := redismocks.MockRedisDbClient{}
	dBMockClient2.On("FindNodeKey", mock.Anything).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "find node group failed"))

	flds6 := fields1{
		service:  mockUtils.AppService,
		dbClient: &dBMockClient2,
	}

	type args struct {
		groupName        string
		groupDisplayName string
		node             *models.Node
	}
	args1 := args{
		groupName:        "Group-1",
		groupDisplayName: "Group Display Name",
		node: &models.Node{
			NodeId:       "123",
			HostName:     "host-1",
			Name:         "Node-1",
			IsRemoteHost: false,
		},
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		{"AddNodeToDefaultGroup - Passed", flds4, args1, false},
		{"AddNodeToDefaultGroup - Failed", flds5, args1, true},
		{"AddNodeToDefaultGroup - Failed1", flds6, args1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			if err := ng.AddNodeToDefaultGroup(tt.args.groupName, tt.args.groupDisplayName, tt.args.node); (err != nil) != tt.wantErr {
				t.Errorf("AddNodeToDefaultGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeGroupService_DeleteNode(t *testing.T) {
	Init()
	dBMockClient.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{}, nil)
	dBMockClient.On("DeleteNode", mock.Anything, mock.Anything).Return(nil)

	dBMockClient1.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "mocked error"))

	type args struct {
		nodeId string
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		{"DeleteNode - Passed", flds4, args{"123"}, false},
		{"DeleteNode - Failed", flds5, args{"123"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			if err := ng.DeleteNode(tt.args.nodeId); (err != nil) != tt.wantErr {
				t.Errorf("DeleteNode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeGroupService_DeleteNodeGroup(t *testing.T) {
	Init()
	dBMockClient.On("FindNodeKey", mock.Anything).Return("parentGroupHashKey", nil)
	dBMockClient.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{}, nil)
	dBMockClient.On("DeleteNodeGroup", mock.Anything, mock.Anything).Return(nil)

	dBMockClient1.On("FindNodeKey", mock.Anything).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	dBMockClient2 := redismocks.MockRedisDbClient{}
	dBMockClient2.On("FindNodeKey", mock.Anything).Return("parentGroupHashKey", nil)
	dBMockClient2.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	dBMockClient3 := redismocks.MockRedisDbClient{}
	dBMockClient3.On("FindNodeKey", mock.Anything).Return("parentGroupHashKey", nil)
	dBMockClient3.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{}, nil)
	dBMockClient3.On("DeleteNodeGroup", mock.Anything, mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	dBMockClient4 := redismocks.MockRedisDbClient{}
	dBMockClient4.On("FindNodeKey", mock.Anything).Return("", nil)

	dBMockClient5 := redismocks.MockRedisDbClient{}
	dBMockClient5.On("FindNodeKey", mock.Anything).Return("parentGroupHashKey", nil)
	dBMockClient5.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{{NodeId: "123"}}, nil)

	flds6 := fields1{
		service:  mockUtils.AppService,
		dbClient: &dBMockClient2,
	}
	flds7 := fields1{
		service:  mockUtils.AppService,
		dbClient: &dBMockClient3,
	}
	flds8 := fields1{
		service:  mockUtils.AppService,
		dbClient: &dBMockClient4,
	}
	flds9 := fields1{
		service:  mockUtils.AppService,
		dbClient: &dBMockClient5,
	}
	type args struct {
		groupName string
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantErr bool
	}{
		{"DeleteNodeGroup - Passed", flds4, args{"Group-1"}, false},
		{"DeleteNodeGroup - Failed", flds5, args{"Group-1"}, true},
		{"DeleteNodeGroup - Failed1", flds6, args{"Group-1"}, true},
		{"DeleteNodeGroup - Failed2", flds7, args{"Group-1"}, true},
		{"DeleteNodeGroup - Failed3", flds8, args{"Group-1"}, true},
		{"DeleteNodeGroup - Failed4", flds9, args{"Group-1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			if err := ng.DeleteNodeGroup(tt.args.groupName); (err != nil) != tt.wantErr {
				t.Errorf("DeleteNodeGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeGroupService_GetNode(t *testing.T) {
	Init()
	dBMockClient.On("GetNode", mock.Anything).Return(&models.Node{}, nil)
	dBMockClient1.On("GetNode", mock.Anything).Return(&models.Node{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	type args struct {
		nodeName string
	}
	want1 := &models.Node{
		IsRemoteHost:     false,
		RuleEndPoint:     "/hedge/api/v3/rules/edgex-kuiper/",
		WorkFlowEndPoint: "/hedge/hedge-node-red/hedge-node-red/",
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    *models.Node
		wantErr bool
	}{
		{"GetNode - Passed", flds4, args{"Node-1"}, want1, false},
		{"GetNode - Failed", flds5, args{"Node-1"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			got, err := ng.GetNode(tt.args.nodeName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNode() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeGroupService_GetNodeGroups(t *testing.T) {
	Init()
	dBMockClient.On("FindNodeKey", mock.Anything).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("GetDBNodeGroupMembers", mock.Anything).Return([]models.DBNodeGroup{}, nil)

	dBMockClient1.On("FindNodeKey", mock.Anything).Return("parentGroupHashKey", nil)
	dBMockClient1.On("GetDBNodeGroupMembers", mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	node := &models.Node{
		HostName:     "host-1",
		Name:         "Node-1",
		IsRemoteHost: true,
		NodeId:       "123",
	}
	dBMockClient2 := redismocks.MockRedisDbClient{}
	dBMockClient2.On("FindNodeKey", mock.Anything).Return("parentGroupHashKey", nil)
	dBMockClient2.On("GetDBNodeGroupMembers", "parentGroupHashKey").Return([]models.DBNodeGroup{{NodeId: "123"}}, nil)
	dBMockClient2.On("GetDBNodeGroupMembers", mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient2.On("GetNode", "123").Return(node, nil)

	flds6 := fields1{
		service:  mockUtils.AppService,
		dbClient: &dBMockClient2,
	}
	type args struct {
		parentGroupName string
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    []models.NodeGroup
		wantErr bool
	}{
		{"GetNodeGroups - Failed", flds4, args{"parentGroupName"}, nil, true},
		{"GetNodeGroups - Failed1", flds5, args{"parentGroupName"}, nil, true},
		{"GetNodeGroups - Failed2", flds6, args{"parentGroupName"}, []models.NodeGroup{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			got, err := ng.GetNodeGroups(tt.args.parentGroupName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNodeGroups() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNodeGroups() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeGroupService_GetNodes(t *testing.T) {
	Init()
	node := models.Node{
		IsRemoteHost:     false,
		RuleEndPoint:     "/hedge/api/v3/rules/edgex-kuiper/",
		WorkFlowEndPoint: "/hedge/hedge-node-red/hedge-node-red/",
	}
	dBMockClient.On("GetAllNodes").Return([]models.Node{node}, nil)

	tests := []struct {
		name    string
		fields  fields1
		want    []models.Node
		wantErr bool
	}{
		{"GetNodes - Passed", flds4, []models.Node{node}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			got, err := ng.GetNodes()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNodes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeGroupService_SaveNode(t *testing.T) {
	Init()
	dBMockClient.On("SaveNode", mock.Anything).Return("key", nil)

	type args struct {
		node *models.Node
	}
	args1 := args{
		&models.Node{
			HostName:     "host-1",
			Name:         "Node-1",
			IsRemoteHost: false,
		},
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    string
		wantErr bool
	}{
		{"GetNode - Passed", flds4, args1, "key", false},
		{"GetNode - Failed", flds4, args{&models.Node{}}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			got, err := ng.SaveNode(tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SaveNode() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeGroupService_getNodeGroupHashKeysMatchingANode(t *testing.T) {
	Init()
	dBMockClient.On("GetNode", mock.Anything).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dbNodeGroups := []models.DBNodeGroup{{
		NodeId:      "123",
		Name:        "NodeGroup-1",
		DisplayName: "Display Node Group Name"},
	}
	dBMockClient.On("GetDBNodeGroupMembers", mock.Anything).Return(dbNodeGroups, nil)

	node := &models.Node{
		HostName:     "host-1",
		Name:         "Node-1",
		IsRemoteHost: true,
		NodeId:       "123",
	}
	dBMockClient1.On("GetNode", mock.Anything).Return(node, nil)
	dBMockClient1.On("GetDBNodeGroupMembers", "hashKey").Return(dbNodeGroups, nil)
	dBMockClient1.On("GetDBNodeGroupMembers", "hashKey:NodeGroup-1").Return(dbNodeGroups, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))

	type args struct {
		hashKey string
		nodeId  string
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    []mod.KeyFieldTuple
		wantErr bool
	}{
		{"getNodeGroupHashKeysMatchingANode - Passed", flds4, args{"hashKey", "123"}, []mod.KeyFieldTuple{}, false},
		{"getNodeGroupHashKeysMatchingANode - Passed1", flds5, args{"hashKey", "123"}, []mod.KeyFieldTuple{{"hashKey", "NodeGroup-1"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			got, err := ng.getNodeGroupHashKeysMatchingANode(tt.args.hashKey, tt.args.nodeId)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodeGroupHashKeysMatchingANode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNodeGroupHashKeysMatchingANode() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeGroupService_getNodeGroups(t *testing.T) {
	Init()
	node := &models.Node{
		HostName:     "host-1",
		Name:         "Node-1",
		IsRemoteHost: true,
	}
	dbNodeGroups := []models.DBNodeGroup{{
		NodeId:      "123",
		Name:        "NodeGroup-1",
		DisplayName: "Display Node Group Name"},
	}
	dBMockClient.On("GetNode", mock.Anything).Return(node, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	dBMockClient.On("GetDBNodeGroupMembers", mock.Anything).Return(dbNodeGroups, nil)

	type args struct {
		hashKey string
	}
	tests := []struct {
		name    string
		fields  fields1
		args    args
		want    []models.NodeGroup
		wantErr bool
	}{
		{"getNodeGroups - Passed", flds4, args{"parentGroupName"}, []models.NodeGroup{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ng := &NodeGroupService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
			}
			got, err := ng.getNodeGroups(tt.args.hashKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodeGroups() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNodeGroups() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeRedService_ImportNodeRedWorkFlows(t *testing.T) {
	Init()
	u1 := utils.NewApplicationServiceMock(map[string]string{})
	mockSecretProvider := mocks3.SecretProvider{}
	mockSecretProvider.On("GetSecret", "mbconnection").Return(make(map[string]string), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
	u1.AppService.On("SecretProvider").Return(&mockSecretProvider)

	flds7 := fields2{
		service:   u1.AppService,
		appConfig: appConfig,
	}
	type args struct {
		contentData mod.ContentData
		installOpt  bool
	}
	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"ImportNodeRedWorkFlows with demo data - Failed", flds6, args{mod.ContentData{}, true}, true},
		{"ImportNodeRedWorkFlows with demo data - Failed1", flds7, args{mod.ContentData{}, true}, true},
		{"ImportNodeRedWorkFlows without demo data - Failed", flds6, args{mod.ContentData{}, false}, true},
		{"ImportNodeRedWorkFlows without demo data - Failed1", flds7, args{mod.ContentData{}, false}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeRedService := NodeRedService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := nodeRedService.ImportNodeRedWorkFlows(tt.args.contentData, tt.args.installOpt); (err != nil) != tt.wantErr {
				t.Errorf("ImportNodeRedWorkFlows() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeRedService_appendMqttCredentials(t *testing.T) {
	Init()
	type args struct {
		flowBytes []byte
		mqttCreds map[string]string
	}
	args1 := args{
		flowBytes: []byte(`[{"type": "mqtt-broker", "otherField": "value1"}]`),
		mqttCreds: map[string]string{
			"username": "your_username",
			"password": "your_password",
		},
	}
	want1 := []byte(`[{"broker":"tcp://localhost:1883","credentials":{"password":"your_password","user":"your_username"},"otherField":"value1","type":"mqtt-broker"}]`)
	args2 := args{
		flowBytes: make([]byte, 0),
		mqttCreds: make(map[string]string),
	}
	tests := []struct {
		name   string
		fields fields2
		args   args
		want   []byte
	}{
		{"appendMqttCredentials - Passed", flds6, args1, want1},
		{"appendMqttCredentials - Failed", flds6, args2, []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeRedService := NodeRedService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if got := nodeRedService.appendMqttCredentials(tt.args.flowBytes, tt.args.mqttCreds); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("appendMqttCredentials() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeRedService_createWorkFlows(t *testing.T) {
	Init()
	nodeRedDir := "./contents/node-red/workFlows"
	os.MkdirAll(nodeRedDir, 0755)
	nodeRedFlowFile1 := filepath.Join(nodeRedDir, "nodeRedFlow-1.json")
	ioutil.WriteFile(nodeRedFlowFile1, []byte("[testdata]"), 0644)
	nodeRedFlowFile2 := filepath.Join(nodeRedDir, "nodeRedFlow-2.json")
	ioutil.WriteFile(nodeRedFlowFile2, []byte("[testdata]"), 0644)

	nodeRedDir = "./contents/node-red/configure"
	os.MkdirAll(nodeRedDir, 0755)
	nodeRedFlowFile1 = filepath.Join(nodeRedDir, "mqtt_node.json")
	ioutil.WriteFile(nodeRedFlowFile1, []byte("[testdata]"), 0644)

	mockedHttpClient.RegisterExternalMockRestCall("", "GET", []byte("mocked response"), 200, nil)
	type args struct {
		contentData mod.ContentData
		mqttCreds   map[string]string
		installOpt  bool
	}
	args1 := args{
		contentData: mod.ContentData{
			TargetNodes: []string{"localhost"},
		},
		mqttCreds: map[string]string{
			"username": "your_username",
			"password": "your_password",
		},
		installOpt: true,
	}
	args2 := args1
	args2.installOpt = false //change installOpt to false

	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"createWorkFlows with demo data- Passed", flds6, args1, false},
		{"createWorkFlows without demo data - Passed", flds6, args2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if strings.Contains(tt.name, "Passed") {
				mockedHttpClient.RegisterExternalMockRestCall("http://localhost:1880/flows", "POST", []byte("mocked response"), 200, nil)
			}

			nodeRedService := NodeRedService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := nodeRedService.createWorkFlows(tt.args.contentData, tt.args.mqttCreds, tt.args.installOpt); (err != nil) != tt.wantErr {
				t.Errorf("createWorkFlows() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeRedService_exportNodeRedWorkflow(t *testing.T) {
	Init()
	type args struct {
		contentData   mod.ContentData
		workflowBytes []byte
	}
	args1 := args{
		contentData: mod.ContentData{
			TargetNodes: []string{"localhost"},
		},
		workflowBytes: make([]byte, 0),
	}
	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"exportNodeRedWorkflow - Passed1", flds6, args{mod.ContentData{}, nil}, false},
		{"exportNodeRedWorkflow - Passed2", flds6, args1, false},
		{"exportNodeRedWorkflow - Failed", flds6, args1, true},
		{"exportNodeRedWorkflow - Failed1", flds6, args1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHttpClient1 := &svcmocks.MockHTTPClient{}
			data := []map[string]string{
				{"key1": "value1"},
				{"id": "bf47c3db.d1bc"},
			}
			jsonData, _ := json.Marshal(data)

			mockResponse := &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("mocked response"))),
			}
			mockResponse1 := &http.Response{
				Status:     "400 Bad Request",
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(bytes.NewReader([]byte("failed request"))),
			}
			mockResponse2 := &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(jsonData)),
			}
			client.Client = mockedHttpClient1

			if tt.name == "exportNodeRedWorkflow - Passed1" || tt.name == "exportNodeRedWorkflow - Passed2" {
				mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet
				})).Return(mockResponse, nil)

				mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodPost
				})).Return(mockResponse1, nil)
			}
			if tt.name == "exportNodeRedWorkflow - Failed" {
				mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet
				})).Return(mockResponse, nil)

				mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodPost
				})).Return(mockResponse1, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
			}
			if tt.name == "exportNodeRedWorkflow - Failed1" {
				mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet
				})).Return(mockResponse2, nil)

				mockedHttpClient1.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodPost
				})).Return(mockResponse1, nil)
			}

			nodeRedService := NodeRedService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := nodeRedService.exportNodeRedWorkflow(tt.args.contentData, tt.args.workflowBytes); (err != nil) != tt.wantErr {
				t.Errorf("exportNodeRedWorkflow() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleService_ImportStreamsAndRules(t *testing.T) {
	Init()
	streamsDir := "./contents/rules/nodeType/streams"
	os.MkdirAll(streamsDir, 0755)
	nodeRedFlowFile1 := filepath.Join(streamsDir, "stream-1.json")
	ioutil.WriteFile(nodeRedFlowFile1, []byte("[testdata]"), 0644)

	type args struct {
		contentData mod.ContentData
	}
	args1 := args{
		contentData: mod.ContentData{
			TargetNodes: []string{"localhost"},
			NodeType:    "nodeType",
		},
	}
	args2 := args{
		contentData: mod.ContentData{
			TargetNodes: []string{},
			NodeType:    "nodeType",
		},
	}
	args3 := args{
		contentData: mod.ContentData{
			TargetNodes: []string{"localhost"},
			NodeType:    "nodeType",
			ContentDir:  []string{"contentDir"},
		},
	}
	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"ImportStreamsAndRules - Passed", flds6, args1, false},
		{"ImportStreamsAndRules - Failed1", flds6, args2, true},
		{"ImportStreamsAndRules - Failed2", flds6, args3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if strings.Contains(tt.name, "Passed") || strings.Contains(tt.name, "Failed2") {
				mockedHttpClient.RegisterExternalMockRestCall("http://localhost:59720/streams", "POST", []byte("mocked response"), 200, nil)
			}
			if strings.Contains(tt.name, "Failed1") {
				mockedHttpClient.RegisterExternalMockRestCall("http://localhost:59720/streams", "POST", []byte("mocked response"), 200, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
			}

			ruleService := RuleService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := ruleService.ImportStreamsAndRules(tt.args.contentData); (err != nil) != tt.wantErr {
				t.Errorf("ImportStreamsAndRules() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleService_create(t *testing.T) {
	Init()
	type args struct {
		contentData mod.ContentData
		fileName    string
		pathType    string
	}
	args1 := args{
		contentData: mod.ContentData{
			TargetNodes: []string{"localhost"},
		},
	}
	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"create - Failed", flds6, args1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleService := RuleService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := ruleService.create(tt.args.contentData, tt.args.fileName, tt.args.pathType); (err != nil) != tt.wantErr {
				t.Errorf("create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleService_createRules(t *testing.T) {
	Init()
	type args struct {
		contentData mod.ContentData
	}
	args1 := args{
		contentData: mod.ContentData{
			TargetNodes: []string{"localhost"},
			NodeType:    "nodeType",
			ContentDir:  []string{"contentDir"},
		},
	}
	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"createRules - Failed", flds6, args1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleService := RuleService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := ruleService.createRules(tt.args.contentData); (err != nil) != tt.wantErr {
				t.Errorf("createRules() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleService_readDirectoryAndCreate(t *testing.T) {
	Init()
	type args struct {
		dir         string
		contentData mod.ContentData
		pathType    string
	}
	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"readDirectoryAndCreate - Failed", flds6, args{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleService := RuleService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := ruleService.readDirectoryAndCreate(tt.args.dir, tt.args.contentData, tt.args.pathType); (err != nil) != tt.wantErr {
				t.Errorf("readDirectoryAndCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_toNodeGroup(t *testing.T) {
	dbNodeGroup := models.DBNodeGroup{
		Name:        "test-group",
		DisplayName: "Test Group",
	}
	nodeGroup := toNodeGroup(dbNodeGroup)
	if nodeGroup.Name != dbNodeGroup.Name {
		t.Errorf("Expected Name to be %s, got %s", dbNodeGroup.Name, nodeGroup.Name)
	}
	if nodeGroup.DisplayName != dbNodeGroup.DisplayName {
		t.Errorf("Expected DisplayName to be %s, got %s", dbNodeGroup.DisplayName, nodeGroup.DisplayName)
	}
}

func TestNewElasticService(t *testing.T) {
	Init()
	NewElasticService(mockUtils.AppService, appConfig)
}

func TestElasticService_SeedElasticData(t *testing.T) {
	Init()
	dataDir := "./contents/elasticsearch/demo-seed-data"
	os.MkdirAll(dataDir, 0755)
	nodeRedFlowFile1 := filepath.Join(dataDir, "data-1.json")
	jsonArr := []deviceMap{
		{
			Id:         "1",
			DeviceName: "Device1",
			Profile:    "Profile1",
			Region:     "Region1",
			Latitude:   40.7128,
			Longitude:  -74.0060,
			Custom1:    map[string]string{"key1": "value1"},
			Custom2:    map[string]string{"key2": "value2"},
			Custom3:    map[string]string{"key3": "value3"},
			Custom4:    map[string]string{"key4": "value4"},
			Custom5:    map[string]string{"key5": "value5"},
			Created:    0,
		},
	}
	fileContent, _ := json.Marshal(jsonArr)
	ioutil.WriteFile(nodeRedFlowFile1, fileContent, 0644)

	type args struct {
		contentData mod.ContentData
	}
	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"SeedElasticData - Passed", flds6, args{}, false},
		{"SeedElasticData - Failed1", flds6, args{}, true},
		{"SeedElasticData - Failed2", flds6, args{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if strings.Contains(tt.name, "Passed") {
				mockedHttpClient.RegisterExternalMockRestCall("http://hedge-elasticsearch:9200/devicemap_index/_doc/1?refresh", "PUT", []byte("mocked response"), 200, nil)
			}
			if strings.Contains(tt.name, "Failed1") {
				mockedHttpClient.RegisterExternalMockRestCall("http://hedge-elasticsearch:9200/devicemap_index/_doc/1?refresh", "PUT", []byte("mocked response"), 200, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mocked error"))
			}
			if strings.Contains(tt.name, "Failed2") {
				nodeRedFlowFile2 := filepath.Join(dataDir, "data-2.txt")
				ioutil.WriteFile(nodeRedFlowFile2, fileContent, 0644)

				mockedHttpClient.RegisterExternalMockRestCall("http://hedge-elasticsearch:9200/devicemap_index/_doc/1?refresh", "PUT", []byte("mocked response"), 200, nil)
			}

			elasticService := ElasticService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := elasticService.SeedElasticData(tt.args.contentData); (err != nil) != tt.wantErr {
				t.Errorf("SeedElasticData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestElasticService_seedElasticData(t *testing.T) {
	Init()
	type args struct {
		contentData mod.ContentData
	}
	tests := []struct {
		name    string
		fields  fields2
		args    args
		wantErr bool
	}{
		{"seedElasticData - Failed", flds6, args{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elasticService := ElasticService{
				service:   tt.fields.service,
				appConfig: tt.fields.appConfig,
			}
			if err := elasticService.seedElasticData(tt.args.contentData); (err != nil) != tt.wantErr {
				t.Errorf("seedElasticData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestElasticService_SetHttpClient(t *testing.T) {
	client.Client = nil
	elasticService := &ElasticService{
		service:   mockUtils.AppService,
		appConfig: appConfig,
	}
	elasticService.SetHttpClient()

	if client.Client == nil {
		t.Error("Expected non-nil HTTP client, got nil")
		return
	}
}

func TestNodeRedService_SetHttpClient(t *testing.T) {
	nodeRedService := &NodeRedService{}
	client.Client = nil
	nodeRedService.SetHttpClient()

	if client.Client == nil {
		t.Error("Expected non-nil HTTP client, got nil")
		return
	}
}

func TestRuleService_SetHttpClient(t *testing.T) {
	ruleService := &RuleService{}
	client.Client = nil
	ruleService.SetHttpClient()

	if client.Client == nil {
		t.Error("Expected non-nil HTTP client, got nil")
		return
	}
}

func TestDeviceService_SetHttpClient(t *testing.T) {
	deviceService := &DeviceService{}
	client.Client = nil
	deviceService.SetHttpClient()

	if client.Client == nil {
		t.Error("Expected non-nil HTTP client, got nil")
		return
	}
}

func TestDeviceService_fetchConsulToken(t *testing.T) {
	Init()
	deviceService := &DeviceService{
		service: mockUtils.AppService,
	}
	validUUID := uuid.New().String()

	setup1 := func() {
		tempFile, err := os.CreateTemp("", "consul_token_*.txt")
		assert.NoError(t, err)
		defer tempFile.Close()

		_, err = tempFile.WriteString(validUUID)
		assert.NoError(t, err)

		os.Setenv("HEDGE_CONSUL_TOKEN_FILE", tempFile.Name())
	}
	setup2 := func() {
		os.Unsetenv("HEDGE_CONSUL_TOKEN_FILE")
	}
	setup3 := func() {
		os.Setenv("HEDGE_CONSUL_TOKEN_FILE", "/path/to/nonexistent/file.txt")
	}
	setup4 := func() {
		invalidUUID := "invalid-uuid-string"
		tempFile, err := os.CreateTemp("", "consul_token_*.txt")
		assert.NoError(t, err)
		defer tempFile.Close()

		_, err = tempFile.WriteString(invalidUUID)
		assert.NoError(t, err)

		os.Setenv("HEDGE_CONSUL_TOKEN_FILE", tempFile.Name())
	}

	tests := []struct {
		name          string
		setup         func() // setup function to prepare the environment
		expectedToken string
		expectedError error
	}{
		{"fetchConsulToken - Passed", setup1, validUUID, nil},
		{"fetchConsulToken - Failed Environment Variable Missing", setup2, "", errors.New("Invalid consul token found")},
		{"fetchConsulToken - Failed File Read Error", setup3, "", errors.New("Failed reading the consul token")},
		{"fetchConsulToken - Failed Invalid UUID in File", setup4, "", errors.New("Invalid consul token found")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			token, err := deviceService.fetchConsulToken()

			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedToken, token)

			// Reset the environment after each test
			os.Unsetenv("HEDGE_CONSUL_TOKEN_FILE")
		})
	}
}

func Cleanup() {
	os.RemoveAll("./contents")
	os.Unsetenv("HEDGE_CONSUL_TOKEN_FILE")
	os.Remove("test_token_file.txt")
}
