/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	clientMocks "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"hedge/app-services/meta-sync/internal/state"
	"hedge/common/client"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	svc "hedge/common/service"
	"hedge/mocks/hedge/app-services/hedge-device-extensions/pkg/service"
	redis "hedge/mocks/hedge/app-services/meta-sync/db"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	svcmocks "hedge/mocks/hedge/common/service"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

var (
	appMock          *utils.HedgeMockUtils
	mMetaService     *service.MockMetaDataService
	mDBClient        *redis.MockDBClient
	mockedHttpSender svcmocks.MockHTTPSenderInterface
	mockResponse     http.Response
	validState       state.State
)

type fields struct {
	HAdminUrl  string
	HttpSender svc.HTTPSenderInterface
}

type fields1 struct {
	Service            interfaces.ApplicationService
	MetaService        *service.MockMetaDataService
	stateDataSender    *transforms.MQTTSecretSender
	node               string
	nodeType           string
	stateRequestSender *transforms.MQTTSecretSender
}

func init() {
	appMock = utils.NewApplicationServiceMock(map[string]string{"HedgeAdminURL": "http://hedge-admin:4321"})
	mockedHttpSender = svcmocks.MockHTTPSenderInterface{}
	mockedHttpSender.On("SetURL", mock.Anything).Return()

	mMetaService = &service.MockMetaDataService{}
	mDBClient = &redis.MockDBClient{}

	mockResponseBody, _ := json.Marshal(models.Node{Name: "nodeName", NodeId: "nodeId"})
	mockResponse = http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(mockResponseBody)),
		Header:     make(http.Header),
	}

	dsClient := clientMocks.DeviceServiceClient{}
	dsClient.On("DeleteByName", mock.Anything, mock.Anything).Return(common.BaseResponse{}, nil)
	dsClient.On("DeviceServiceByName", mock.Anything, mock.Anything).Return(responses.DeviceServiceResponse{}, nil)
	dsClient.On("Add", mock.Anything, mock.Anything).Return(nil, nil)
	dsClient.On("Update", mock.Anything, mock.Anything).Return(nil, nil)
	appMock.AppService.On("DeviceServiceClient").Return(&dsClient)

	d1 := dtos.Device{Id: "device-1", Name: "device-1"}
	d2 := dtos.Device{Id: "device-2", Name: "device-2"}
	p1 := dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "profile-1", Name: "profile-1"}}
	p2 := dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "profile-2", Name: "profile-2"}}

	validState = state.State{
		CoreStateVersion: "some-version",
		Node: models.Node{
			NodeId:   "NODE-1",
			HostName: "node-1",
		},
		NodeType:  "NODE",
		Recipient: "NODE-1",
		Devices: map[string]*state.Info{
			"device-1": {
				Sum:    "sum-device-1",
				NodeID: "NODE-1",
				Action: state.CREATE,
			},
			"device-2": {
				Sum:    "sum-device-2",
				NodeID: "NODE-1",
				Action: state.UPDATE,
			},
		},
		Profiles: map[string]*state.Info{
			"profile-1": {
				Sum:    "sum-profile-1",
				Action: state.CREATE,
			},
			"profile-2": {
				Sum:    "sum-profile-2",
				Action: state.UPDATE,
			},
		},
		DeviceServices: map[string]*state.Info{
			"NODE-1__service1": {
				Sum:    "CREATE",
				NodeID: "NODE-1",
				Action: state.CREATE,
			},
			"NODE-1__service2": {
				Sum:    "UPDATE",
				NodeID: "NODE-1",
				Action: state.UPDATE,
			},
		},
		RawDataConfig: map[string]*state.Info{
			"NODE-1": {
				Sum:    "sum-config-1",
				NodeID: "NODE-1",
				Action: state.CREATE,
			},
			"NODE-2": {
				Sum:    "sum-config-2",
				NodeID: "NODE-2",
				Action: state.UPDATE,
			},
		},
		Data: &state.Data{
			Devices: []*dto.DeviceObject{
				{Device: d1},
				{Device: d2},
			},
			Profiles: []*dto.ProfileObject{
				{Profile: p1},
				{Profile: p2},
			},
			DeviceServices: []*dtos.DeviceService{
				{
					Name:        "service1",
					BaseAddress: "http://localhost:48081",
				},
				{
					Name:        "service2",
					BaseAddress: "http://localhost:48081",
				},
			},
			RawConfig: []*dto.NodeRawDataConfig{
				{
					Node: dto.Node{
						NodeID: "NODE-1",
						Host:   "host",
					},
					SendRawData: true,
				},
			},
		},
		Timestamp: time.Now().Unix(),
	}
}

// TestData represents the structure of the JSON data
type TestData struct {
	Core state.State `json:"core"`
	Node state.State `json:"node"`
}

func setupMetaSync(nodeId, nodeType string) *MetaSync {
	// setup for stateManager
	appMock.AppService.On("AddCustomRoute", "/api/v3/state/refresh", interfaces.Unauthenticated, mock.Anything, "POST").
		Return(nil)

	stateManager, _ := state.NewManager(appMock.AppService, mMetaService, mDBClient, models.Node{NodeId: nodeId}, nodeType, 0, 0, false)
	return NewMetaSync(appMock.AppService, mMetaService, stateManager, nil, nil, "", "", false, nodeId, nodeType)
}

func TestSync_NodeToCoreCreate(t *testing.T) {
	sync := setupMetaSync("CORE-1", "CORE")
	registeredNode := models.Node{NodeId: "NODE-1", HostName: "NODE-1"}

	file, err := os.Open("../internal/state/testdata/node-to-core-create.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	currentState, _ := json.Marshal(stateData.Core)

	mMetaService.On("GetAllNodes").Return([]models.Node{registeredNode}, nil).Once()

	mDBClient.On("KnownNode", registeredNode.NodeId).Return(registeredNode.HostName).Once()
	mDBClient.On("GetState").Return(currentState, nil).Once()
	mDBClient.On("GetStateCoreVersion").Return(stateData.Core.CoreStateVersion).Once()

	moveNext, s := sync.State(appMock.AppFunctionContext, stateData.Node)

	assert.Equal(t, true, moveNext)

	rs := s.(*state.Change)
	for _, v := range rs.Devices {
		assert.Equal(t, state.CREATE, v.Action)
	}
	for _, v := range rs.Profiles {
		assert.Equal(t, state.CREATE, v.Action)
	}
	for _, v := range rs.DeviceServices {
		assert.Equal(t, state.CREATE, v.Action)
	}
	for _, v := range rs.RawDataConfig {
		assert.Equal(t, state.CREATE, v.Action)
	}

	assert.Equal(t, 3, len(rs.Devices))
	assert.Equal(t, 12, len(rs.Profiles))
	assert.Equal(t, 2, len(rs.DeviceServices))
	assert.Equal(t, 1, len(rs.RawDataConfig))
}

func TestSync_NodeToCoreUpdate(t *testing.T) {
	sync := setupMetaSync("CORE-1", "CORE")

	registeredNode := models.Node{NodeId: "NODE-1", HostName: "NODE-1"}

	file, err := os.Open("../internal/state/testdata/node-to-core-update.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	currentState, _ := json.Marshal(stateData.Core)

	mMetaService.On("GetAllNodes").Return([]models.Node{registeredNode}, nil).Once()

	mDBClient.On("KnownNode", registeredNode.NodeId).Return(registeredNode.HostName).Once()
	mDBClient.On("GetState").Return(currentState, nil).Once()
	mDBClient.On("GetStateCoreVersion").Return(stateData.Core.CoreStateVersion).Once()

	moveNext, s := sync.State(appMock.AppFunctionContext, stateData.Node)

	assert.Equal(t, true, moveNext)

	rs := s.(*state.Change)
	for _, v := range rs.Devices {
		assert.Equal(t, state.UPDATE, v.Action)
	}
	for _, v := range rs.Profiles {
		assert.Equal(t, state.UPDATE, v.Action)
	}
	for _, v := range rs.DeviceServices {
		assert.Equal(t, state.UPDATE, v.Action)
	}
	for _, v := range rs.RawDataConfig {
		assert.Equal(t, state.UPDATE, v.Action)
	}

	assert.Equal(t, 3, len(rs.Devices))
	assert.Equal(t, 12, len(rs.Profiles))
	assert.Equal(t, 2, len(rs.DeviceServices))
	assert.Equal(t, 1, len(rs.RawDataConfig))
}

func TestSync_NodeToCoreDelete(t *testing.T) {
	sync := setupMetaSync("CORE-1", "CORE")

	registeredNode := models.Node{NodeId: "NODE-1", HostName: "NODE-1"}

	file, err := os.Open("../internal/state/testdata/node-to-core-delete.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	currentState, _ := json.Marshal(stateData.Core)

	mMetaService.On("GetAllNodes").Return([]models.Node{registeredNode}, nil).Once()
	mMetaService.On("DeleteCompleteDevice", mock.Anything, mock.Anything).Return(nil)
	mMetaService.On("DeleteCompleteProfile", mock.Anything).Return(nil)

	mDBClient.On("KnownNode", registeredNode.NodeId).Return(registeredNode.HostName).Once()
	mDBClient.On("GetState").Return(currentState, nil).Once()
	mDBClient.On("GetStateCoreVersion").Return(stateData.Core.CoreStateVersion).Once()

	moveNext, s := sync.State(appMock.AppFunctionContext, stateData.Node)

	assert.Equal(t, false, moveNext)

	rs := s.(*state.Change)
	assert.Equal(t, 0, len(rs.Devices))
	assert.Equal(t, 0, len(rs.Profiles))
	assert.Equal(t, 0, len(rs.DeviceServices))
	assert.Equal(t, 0, len(rs.RawDataConfig))
}

func TestSync_CoreToNodeCreate(t *testing.T) {
	sync := setupMetaSync("NODE-1", "NODE")

	registeredNode := models.Node{NodeId: "CORE-1", HostName: "core-1"}

	file, err := os.Open("../internal/state/testdata/core-to-node-create.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	currentState, _ := json.Marshal(stateData.Node)

	mDBClient.On("KnownNode", registeredNode.NodeId).Return(registeredNode.HostName).Once()
	mDBClient.On("GetState").Return(currentState, nil).Once()
	mDBClient.On("SetStateCoreVersion", stateData.Core.CoreStateVersion).Return(nil).Once()

	moveNext, s := sync.State(appMock.AppFunctionContext, stateData.Core)

	assert.Equal(t, true, moveNext)

	rs := s.(*state.Change)
	for _, v := range rs.Devices {
		assert.Equal(t, state.CREATE, v.Action)
	}
	for _, v := range rs.Profiles {
		assert.Equal(t, state.CREATE, v.Action)
	}
	for _, v := range rs.DeviceServices {
		assert.Equal(t, state.CREATE, v.Action)
	}
	for _, v := range rs.RawDataConfig {
		assert.Equal(t, state.CREATE, v.Action)
	}

	assert.Equal(t, 3, len(rs.Devices))
	assert.Equal(t, 12, len(rs.Profiles))
	assert.Equal(t, 2, len(rs.DeviceServices))
	assert.Equal(t, 1, len(rs.RawDataConfig))
}

func TestSync_CoreToNodeUpdate(t *testing.T) {
	sync := setupMetaSync("NODE-1", "NODE")

	registeredNode := models.Node{NodeId: "CORE-1", HostName: "core-1"}

	file, err := os.Open("../internal/state/testdata/core-to-node-update.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	currentState, _ := json.Marshal(stateData.Node)

	mDBClient.On("KnownNode", registeredNode.NodeId).Return(registeredNode.HostName).Once()
	mDBClient.On("GetState").Return(currentState, nil).Once()
	mDBClient.On("SetStateCoreVersion", stateData.Core.CoreStateVersion).Return(nil).Once()

	moveNext, s := sync.State(appMock.AppFunctionContext, stateData.Core)

	assert.Equal(t, true, moveNext)

	rs := s.(*state.Change)
	for _, v := range rs.Devices {
		assert.Equal(t, state.UPDATE, v.Action)
	}
	for _, v := range rs.Profiles {
		assert.Equal(t, state.UPDATE, v.Action)
	}
	for _, v := range rs.DeviceServices {
		assert.Equal(t, state.UPDATE, v.Action)
	}
	for _, v := range rs.RawDataConfig {
		assert.Equal(t, state.UPDATE, v.Action)
	}

	assert.Equal(t, 3, len(rs.Devices))
	assert.Equal(t, 12, len(rs.Profiles))
	assert.Equal(t, 2, len(rs.DeviceServices))
	assert.Equal(t, 1, len(rs.RawDataConfig))
}

func TestSync_CoreToNodeDelete(t *testing.T) {
	sync := setupMetaSync("NODE-1", "NODE")

	registeredNode := models.Node{NodeId: "CORE-1", HostName: "core-1"}

	file, err := os.Open("../internal/state/testdata/core-to-node-delete.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	currentState, _ := json.Marshal(stateData.Node)

	mMetaService.On("GetAllNodes").Return([]models.Node{registeredNode}, nil).Once()
	mMetaService.On("DeleteCompleteDevice", mock.Anything, mock.Anything).Return(nil)
	mMetaService.On("DeleteCompleteProfile", mock.Anything).Return(nil)

	mDBClient.On("KnownNode", registeredNode.NodeId).Return(registeredNode.HostName).Once()
	mDBClient.On("GetState").Return(currentState, nil).Once()
	mDBClient.On("SetStateCoreVersion", stateData.Core.CoreStateVersion).Return(nil).Once()

	moveNext, s := sync.State(appMock.AppFunctionContext, stateData.Core)

	assert.Equal(t, false, moveNext)

	rs := s.(*state.Change)
	assert.Equal(t, 0, len(rs.Devices))
	assert.Equal(t, 0, len(rs.Profiles))
	assert.Equal(t, 0, len(rs.DeviceServices))
	assert.Equal(t, 0, len(rs.RawDataConfig))
}

func TestMetaSync_RegisterNode(t *testing.T) {
	node := models.Node{
		NodeId:   "NODE-1",
		HostName: "node-1",
	}
	syncObjAsBytes := []byte("sync data")
	jsonData, _ := json.Marshal(node)

	flds := fields{
		HAdminUrl:  "http://localhost:48080",
		HttpSender: &mockedHttpSender,
	}

	type args struct {
		ctx            interfaces.AppFunctionContext
		syncObjAsBytes []byte
		node           models.Node
	}
	args1 := args{
		ctx:            appMock.AppFunctionContext,
		syncObjAsBytes: syncObjAsBytes,
		node:           node,
	}

	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{"RegisterNode - Passed", flds, args1, false, ""},
		{"RegisterNode - Failed", flds, args1, true, "Error registering node"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &MetaSync{
				HAdminUrl:  tt.fields.HAdminUrl,
				HttpSender: tt.fields.HttpSender,
			}
			if tt.wantErr && tt.name == "RegisterNode - Failed" {
				mockedHttpSender.On("HTTPPost", mock.Anything, mock.Anything).Return(false, nil).Once()
			} else {
				mockedHttpSender.On("HTTPPost", mock.Anything, mock.Anything).Return(true, jsonData).Once()
			}

			err := s.registerNode(tt.args.ctx, tt.args.syncObjAsBytes, tt.args.node)
			if tt.wantErr {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestMetaSync_DeleteNode(t *testing.T) {
	nodeID := "NODE-1"
	flds := fields{
		HAdminUrl:  "http://localhost:48080",
		HttpSender: &mockedHttpSender,
	}

	type args struct {
		ctx    interfaces.AppFunctionContext
		nodeID string
	}
	args1 := args{
		ctx:    appMock.AppFunctionContext,
		nodeID: nodeID,
	}

	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{"DeleteNode - Passed", flds, args1, false, ""},
		{"DeleteNode - Failed", flds, args1, true, "Error deleting old node"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &MetaSync{
				HAdminUrl:  tt.fields.HAdminUrl,
				HttpSender: tt.fields.HttpSender,
			}

			url := tt.fields.HAdminUrl + "/api/v3/node_mgmt/node/" + tt.args.nodeID
			mockedHttpSender.On("SetURL", url).Once()

			if tt.wantErr && tt.name == "DeleteNode - Failed" {
				mockedHttpSender.On("HTTPDelete", mock.Anything, nil).Return(false, nil).Once()
			} else {
				mockedHttpSender.On("HTTPDelete", mock.Anything, nil).Return(true, nil).Once()
			}

			err := s.deleteNode(tt.args.ctx, tt.args.nodeID)
			if tt.wantErr {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestMetaSync_StateRequest(t *testing.T) {
	mqttConfig := transforms.MQTTSecretConfig{
		BrokerAddress: "",
		ClientId:      "test_dataEnrich",
		Topic:         "BMCMetrics",
		QoS:           0,
	}

	mockHttpClient := svcmocks.MockHTTPClient{}
	mockHttpClient.On("Do", mock.Anything).Return(&mockResponse, nil)
	client.Client = &mockHttpClient

	mockedMetaService := &service.MockMetaDataService{}
	mockedMetaService1 := &service.MockMetaDataService{}

	stateDataSender := transforms.NewMQTTSecretSender(mqttConfig, false)
	node := "NODE-1"
	d := dtos.Device{Id: "device-1", Name: "device-1"}
	completeDevice := dto.DeviceObject{Device: d}
	p := dtos.DeviceProfile{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "profile-1", Name: "profile-1"}}
	completeProfile := dto.ProfileObject{Profile: p}

	rawConfigs := make(map[string]*dto.NodeRawDataConfig)
	rawConfigs["NODE-1"] = &dto.NodeRawDataConfig{Node: dto.Node{NodeID: "NODE-1"}}

	mockedMetaService.On("GetNodeRawDataConfigs", mock.Anything).Return(rawConfigs, nil)
	mockedMetaService1.On("GetNodeRawDataConfigs", mock.Anything).Return(rawConfigs, nil)
	mockedMetaService.On("GetNode", mock.Anything, "current").Return(models.Node{NodeId: "NODE-1"}, nil)
	mockedMetaService.On("GetCompleteDevice", mock.Anything, mock.Anything, mock.Anything).Return(completeDevice, nil)
	mockedMetaService.On("GetCompleteProfile", mock.Anything).Return(completeProfile, nil)

	flds := fields1{
		Service:         appMock.AppService,
		MetaService:     mockedMetaService,
		stateDataSender: stateDataSender,
		node:            node,
		nodeType:        "NODE",
	}
	flds1 := fields1{
		Service:         appMock.AppService,
		MetaService:     mockedMetaService1,
		stateDataSender: stateDataSender,
		node:            node,
		nodeType:        "NODE",
	}

	invalidState := "invalid state data"

	type args struct {
		ctx  interfaces.AppFunctionContext
		data interface{}
	}
	args1 := args{
		ctx:  appMock.AppFunctionContext,
		data: validState,
	}
	args2 := args{
		ctx:  appMock.AppFunctionContext,
		data: invalidState,
	}

	tests := []struct {
		name    string
		fields  fields1
		args    args
		wantOk  bool
		want    interface{}
		wantErr bool
	}{
		{"StateRequest - Passed", flds, args1, true, validState, false},
		{"StateRequest - Failed Conversion", flds, args2, false, nil, true},
		{"StateRequest - Non-Matching Recipient", flds, args1, false, nil, false},
		{"StateRequest - Failed Device&Profile Fetch", flds1, args1, true, validState, false},
		{"StateRequest - Failed Get Node", flds, args1, false, validState, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &MetaSync{
				Service:            tt.fields.Service,
				MetaService:        tt.fields.MetaService,
				stateDataSender:    tt.fields.stateDataSender,
				node:               tt.fields.node,
				nodeType:           tt.fields.nodeType,
				stateRequestSender: tt.fields.stateRequestSender,
			}

			if tt.name == "StateRequest - Failed Get Node" {
				mockHttpClient1 := svcmocks.MockHTTPClient{}
				mockHttpClient1.On("Do", mock.Anything).Return(nil, errors.New("mocked error"))
				client.Client = &mockHttpClient1
			}

			if tt.name == "StateRequest - Non-Matching Recipient" {
				tt.args.data = state.State{
					Node:      models.Node{NodeId: "NODE-1"},
					Recipient: "DIFFERENT_NODE",
				}
			}

			if tt.name == "StateRequest - Failed Device&Profile Fetch" {
				mockedMetaService1.On("GetCompleteDevice", mock.Anything, mock.Anything, mock.Anything).Return(dto.DeviceObject{}, hedgeErrors.CommonHedgeError{})
				mockedMetaService1.On("GetCompleteProfile", mock.Anything, mock.Anything).Return(dto.ProfileObject{}, hedgeErrors.CommonHedgeError{})
			}

			gotOk, got := s.StateRequest(tt.args.ctx, tt.args.data)
			assert.Equal(t, tt.wantOk, gotOk)

			if tt.wantOk {
				assert.NotNil(t, got)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func TestMetaSync_StateData(t *testing.T) {
	mockedMetaService := &service.MockMetaDataService{}
	node := "NODE-1"

	tests := []struct {
		name       string
		incoming   state.State
		recipient  string
		wantResult bool
	}{
		{"StateData - Passed NodeType NODE", validState, "NODE-1", true},
		{"StateData - Passed NodeType CORE", validState, "NODE-1", true},
		{"StateData - Failed Invalid Data Type", state.State{Node: validState.Node, Recipient: "NODE-2", Data: nil}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedMetaService.On("GetCompleteDevice", mock.Anything, mock.Anything, mock.Anything).Return(dto.DeviceObject{}, nil).Once()
			mockedMetaService.On("CreateCompleteDevice", mock.Anything, mock.Anything).Return("nil", nil).Once()
			mockedMetaService.On("UpdateCompleteDevice", mock.Anything, mock.Anything).Return("nil", nil).Once()

			mockedMetaService.On("GetCompleteProfile", mock.Anything).Return(dto.ProfileObject{}, nil).Once()
			mockedMetaService.On("CreateCompleteProfile", mock.Anything, mock.Anything).Return("nil", nil).Once()
			mockedMetaService.On("UpdateCompleteProfile", mock.Anything, mock.Anything).Return("nil", nil).Once()

			mockedMetaService.On("GetNodeRawDataConfigs", mock.Anything).Return(map[string]dto.NodeRawDataConfig{"node1-config": {}}, nil).Once()
			mockedMetaService.On("UpsertNodeRawDataConfigs", mock.Anything).Return(nil).Once()

			s := &MetaSync{}

			if tt.name == "StateData - Passed NodeType CORE" {
				tt.incoming.NodeType = "CORE"

				s = &MetaSync{
					HAdminUrl:   "http://localhost:48080",
					HttpSender:  &mockedHttpSender,
					Service:     appMock.AppService,
					node:        node,
					nodeType:    "CORE",
					MetaService: mockedMetaService,
				}
			} else {
				s = &MetaSync{
					HAdminUrl:   "http://localhost:48080",
					HttpSender:  &mockedHttpSender,
					Service:     appMock.AppService,
					node:        node,
					nodeType:    "NODE",
					MetaService: mockedMetaService,
				}
			}

			result, _ := s.StateData(appMock.AppFunctionContext, tt.incoming)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}
