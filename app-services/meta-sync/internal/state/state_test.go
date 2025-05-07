package state

import (
	"context"
	"encoding/json"
	redis3 "hedge/mocks/hedge/app-services/hedge-device-extensions/pkg/db/redis"
	"os"
	"testing"

	"errors"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	clientMocks "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/responses"
	errors1 "github.com/edgexfoundry/go-mod-core-contracts/v3/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"hedge/app-services/metadata-notifier/pkg"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	"hedge/mocks/hedge/app-services/hedge-device-extensions/pkg/service"
	redis "hedge/mocks/hedge/app-services/meta-sync/db"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
)

var (
	appMock             *utils.HedgeMockUtils
	mMetaService        *service.MockMetaDataService
	mDBClient           *redis.MockDBClient
	mDeviceExtClient    *redis3.MockDeviceExtDBClientInterface
	mNotificationClient *clientMocks.NotificationClient
)

func init() {
	appMock = utils.NewApplicationServiceMock(map[string]string{"HedgeAdminURL": "http://hedge-admin:4321"})
	appMock.AppService.On("AddCustomRoute", "/api/v3/state/refresh", interfaces.Unauthenticated, mock.Anything, "POST").
		Return(nil)
	mDeviceExtClient = &redis3.MockDeviceExtDBClientInterface{}
	mDBClient = &redis.MockDBClient{}

	mMetaService = &service.MockMetaDataService{}
	mMetaService.On("GetDbClient").Return(mDeviceExtClient, nil)

	mNotificationClient = &clientMocks.NotificationClient{}
	mNotificationClient.On("SendNotification", mock.Anything, mock.Anything).Return([]common.BaseWithIdResponse{{}}, nil)
	appMock.AppService.On("NotificationClient").Return(mNotificationClient)
	appMock.AppService.On("AppContext").Return(context.Background())
}

// TestData represents the structure of the JSON data
type TestData struct {
	Core State `json:"core"`
	Node State `json:"node"`
}

func setupManager(nodeID, nodeType string, mockerDbClient *redis.MockDBClient) *Manager {
	m, err := NewManager(appMock.AppService, mMetaService, mockerDbClient, models.Node{NodeId: nodeID}, nodeType, 1, 1, false)
	if err != nil {
		panic(err)
	}

	return m
}

func TestChange_NodeToCoreCreate(t *testing.T) {
	manager := setupManager("CORE-1", "CORE", mDBClient)

	file, err := os.Open("testdata/node-to-core-create.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	mDBClient.On("GetStateCoreVersion").Return(stateData.Core.CoreStateVersion).Once()

	changes := manager.Changes(&stateData.Core, &stateData.Node, true)

	for _, v := range changes.Devices {
		assert.Equal(t, CREATE, v.Action)
	}
	for _, v := range changes.Profiles {
		assert.Equal(t, CREATE, v.Action)
	}
	for _, v := range changes.DeviceServices {
		assert.Equal(t, CREATE, v.Action)
	}
	for _, v := range changes.RawDataConfig {
		assert.Equal(t, CREATE, v.Action)
	}

	// Assert the results
	assert.Equal(t, 3, len(changes.Devices))
	assert.Equal(t, 12, len(changes.Profiles))
	assert.Equal(t, 2, len(changes.DeviceServices))
	assert.Equal(t, 1, len(changes.RawDataConfig))
}

func TestChange_CoreToNodeCreate(t *testing.T) {
	manager := setupManager("NODE-1", "NODE", mDBClient)

	file, err := os.Open("testdata/core-to-node-create.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	mDBClient.On("SetStateCoreVersion", stateData.Core.CoreStateVersion).Return(nil).Once()

	changes := manager.Changes(&stateData.Node, &stateData.Core, true)

	for _, v := range changes.Devices {
		assert.Equal(t, CREATE, v.Action)
	}
	for _, v := range changes.Profiles {
		assert.Equal(t, CREATE, v.Action)
	}
	for _, v := range changes.DeviceServices {
		assert.Equal(t, CREATE, v.Action)
	}
	for _, v := range changes.RawDataConfig {
		assert.Equal(t, CREATE, v.Action)
	}

	// Assert the results
	assert.Equal(t, 3, len(changes.Devices))
	assert.Equal(t, 12, len(changes.Profiles))
	assert.Equal(t, 2, len(changes.DeviceServices))
	assert.Equal(t, 1, len(changes.RawDataConfig))
}

func TestChange_CoreToNodeUpdate(t *testing.T) {
	manager := setupManager("NODE-1", "NODE", mDBClient)

	file, err := os.Open("testdata/core-to-node-update.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	mDBClient.On("SetStateCoreVersion", stateData.Core.CoreStateVersion).Return(nil).Once()

	changes := manager.Changes(&stateData.Node, &stateData.Core, true)

	// Assert the results
	for _, v := range changes.Devices {
		assert.Equal(t, UPDATE, v.Action)
	}
	for _, v := range changes.Profiles {
		assert.Equal(t, UPDATE, v.Action)
	}
	for _, v := range changes.DeviceServices {
		assert.Equal(t, UPDATE, v.Action)
	}
	// Assert the results
	assert.Equal(t, 3, len(changes.Devices))
	assert.Equal(t, 12, len(changes.Profiles))
	assert.Equal(t, 2, len(changes.DeviceServices))
	assert.Equal(t, 1, len(changes.RawDataConfig))
}

func TestChange_NodeToCoreUpdate(t *testing.T) {
	manager := setupManager("CORE-1", "CORE", mDBClient)

	file, err := os.Open("testdata/node-to-core-update.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	mDBClient.On("GetStateCoreVersion").Return(stateData.Core.CoreStateVersion).Once()

	changes := manager.Changes(&stateData.Core, &stateData.Node, true)

	// Assert the results
	for _, v := range changes.Devices {
		assert.Equal(t, UPDATE, v.Action)
	}
	for _, v := range changes.Profiles {
		assert.Equal(t, UPDATE, v.Action)
	}
	for _, v := range changes.DeviceServices {
		assert.Equal(t, UPDATE, v.Action)
	}
	// Assert the results
	assert.Equal(t, 3, len(changes.Devices))
	assert.Equal(t, 12, len(changes.Profiles))
	assert.Equal(t, 2, len(changes.DeviceServices))
	assert.Equal(t, 1, len(changes.RawDataConfig))
}

func TestChange_NodeToCoreDelete(t *testing.T) {
	manager := setupManager("CORE-1", "CORE", mDBClient)

	file, err := os.Open("testdata/node-to-core-delete.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	mDBClient.On("GetStateCoreVersion").Return(stateData.Core.CoreStateVersion).Once()

	changes := manager.Changes(&stateData.Core, &stateData.Node, false)

	// Assert the results
	for _, v := range changes.Devices {
		assert.Equal(t, DELETE, v.Action)
	}
	for _, v := range changes.Profiles {
		assert.Equal(t, DELETE, v.Action)
	}
	for _, v := range changes.DeviceServices {
		assert.Equal(t, DELETE, v.Action)
	}
	// Assert the results
	assert.Equal(t, 3, len(changes.Devices))
	assert.Equal(t, 12, len(changes.Profiles))
	assert.Equal(t, 2, len(changes.DeviceServices))
}

func TestChange_CoreToNodeDelete(t *testing.T) {
	manager := setupManager("NODE-1", "NODE", mDBClient)

	file, err := os.Open("testdata/core-to-node-delete.json")
	if err != nil {
		t.Fatalf("failed to open json state file: %v", err)
	}
	defer file.Close()

	var stateData TestData
	err = json.NewDecoder(file).Decode(&stateData)
	if err != nil {
		t.Fatalf("failed to decode json state: %v", err)
	}

	mDBClient.On("SetStateCoreVersion", stateData.Core.CoreStateVersion).Return(nil).Once()

	changes := manager.Changes(&stateData.Node, &stateData.Core, false)

	// Assert the results
	for _, v := range changes.Devices {
		assert.Equal(t, DELETE, v.Action)
	}
	for _, v := range changes.Profiles {
		assert.Equal(t, DELETE, v.Action)
	}
	for _, v := range changes.DeviceServices {
		assert.Equal(t, DELETE, v.Action)
	}
	// Assert the results
	assert.Equal(t, 3, len(changes.Devices))
	assert.Equal(t, 12, len(changes.Profiles))
	assert.Equal(t, 2, len(changes.DeviceServices))
	assert.Equal(t, 0, len(changes.RawDataConfig))
}

func TestListen_CalculateState_Node(t *testing.T) {
	manager := setupManager("NODE-1", "NODE", mDBClient)

	devices := []dtos.Device{
		{Id: "device-1", Name: "device-1"},
		{Id: "device-2", Name: "device-2"},
		{Id: "device-3", Name: "device-3"},
		{Id: "device-4", Name: "device-4"},
		{Id: "device-5", Name: "device-5"},
	}

	var completeDevices []dto.DeviceObject
	for _, d := range devices {
		completeDevices = append(completeDevices, dto.DeviceObject{Device: d})
	}

	profiles := []dtos.DeviceProfile{
		{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "profile-1", Name: "profile-1"}},
		{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "profile-2", Name: "profile-2"}},
		{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "profile-3", Name: "profile-3"}},
		{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "profile-4", Name: "profile-4"}},
		{DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{Id: "profile-5", Name: "profile-5"}},
	}

	var completeProfiles []dto.ProfileObject
	for _, p := range profiles {
		completeProfiles = append(completeProfiles, dto.ProfileObject{Profile: p})
	}

	services := []dtos.DeviceService{
		{Id: "service-1", Name: "service-1"},
		{Id: "service-2", Name: "service-2"},
	}

	rawConfig := make(map[string]*dto.NodeRawDataConfig)
	rawConfig["NODE-1"] = &dto.NodeRawDataConfig{Node: dto.Node{NodeID: "NODE-1"}}

	mMetaService.On("GetNodeRawDataConfigs", mock.AnythingOfType("[]string")).Return(rawConfig, nil).Once()

	deviceClient := clientMocks.DeviceClient{}
	deviceClient.On("AllDevices", mock.Anything, mock.AnythingOfType("[]string"), mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return(responses.MultiDevicesResponse{Devices: devices}, nil)

	profileClient := clientMocks.DeviceProfileClient{}
	profileClient.On("AllDeviceProfiles", mock.Anything, mock.AnythingOfType("[]string"), mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return(responses.MultiDeviceProfilesResponse{Profiles: profiles}, nil)

	serviceClient := clientMocks.DeviceServiceClient{}
	serviceClient.On("AllDeviceServices", mock.Anything, mock.AnythingOfType("[]string"), mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return(responses.MultiDeviceServicesResponse{Services: services}, nil)

	appMock.AppService.On("DeviceClient").Return(&deviceClient)
	appMock.AppService.On("DeviceProfileClient").Return(&profileClient)
	appMock.AppService.On("DeviceServiceClient").Return(&serviceClient)

	mDBClient.On("GetStateCoreVersion").Return("").Once()
	mDBClient.On("SetState", mock.AnythingOfType("[]uint8")).Return(nil).Once()

	mDeviceExtClient.On("GetSceneIds").Return([]string{}, nil)
	mDeviceExtClient.On("GetImageIds").Return([]string{}, nil)

	for _, d := range completeDevices {
		mMetaService.On("GetCompleteDevice", d.Device.Name, "", mock.Anything).Return(d, nil)
	}

	for _, p := range completeProfiles {
		mMetaService.On("GetCompleteProfile", p.Profile.Name, mock.Anything).Return(p, nil)
	}

	notify := manager.Listen()

	s := <-notify
	assert.Equal(t, 5, len(s.Devices))
	assert.Equal(t, 5, len(s.Profiles))
	assert.Equal(t, 2, len(s.DeviceServices))
	assert.Equal(t, 1, len(s.RawDataConfig))
}

func TestManager_GetState(t *testing.T) {
	manager := setupManager("NODE-1", "NODE", mDBClient)

	validState := State{
		CoreStateVersion: "some-version",
		Node:             models.Node{NodeId: "", HostName: "", IsRemoteHost: false, Name: "", RuleEndPoint: "", WorkFlowEndPoint: ""},
		NodeType:         "",
		Recipient:        "",
		Devices:          map[string]*Info(nil),
		Profiles:         map[string]*Info(nil),
		DeviceServices:   map[string]*Info(nil),
		RawDataConfig:    map[string]*Info(nil),
		Data:             (*Data)(nil),
		Timestamp:        1724274019,
	}

	validStateBytes, _ := json.Marshal(validState)

	tests := []struct {
		name          string
		expectedError hedgeErrors.HedgeError
		wantState     *State
		wantErr       bool
	}{
		{"GetState - Passed", nil, &validState, false},
		{"GetState - Failed DB Error", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error getting state"), nil, true},
		{"GetState - Failed Unmarshal Error", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error getting state"), nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "GetState - Failed Unmarshal Error" {
				mDBClient1 := &redis.MockDBClient{}
				mDBClient1.On("GetState").Return([]byte("invalid json"), nil)
				manager = setupManager("NODE-1", "NODE", mDBClient1)
			} else if tt.name == "GetState - Failed DB Error" {
				mDBClient1 := &redis.MockDBClient{}
				mDBClient1.On("GetState").Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error getting state"))
				manager = setupManager("NODE-1", "NODE", mDBClient1)
			} else {
				mDBClient.On("GetState").Return(validStateBytes, nil)
			}

			gotState, err := manager.GetState()

			if tt.wantErr {
				assert.Nil(t, gotState)
				assert.NotNil(t, err)
				assert.Equal(t, err, tt.expectedError)
			} else {
				assert.NotNil(t, gotState)
				assert.Nil(t, err)
				assert.Equal(t, tt.wantState, gotState)
			}
		})
	}
}

func TestManager_KnownNode(t *testing.T) {
	manager := setupManager("NODE-1", "NODE", mDBClient)
	node := models.Node{
		NodeId:   "NODE-1",
		HostName: "node-1",
	}

	tests := []struct {
		name        string
		mockKnown   string
		mockSetErr  error
		expectedRes bool
	}{
		{"KnownNode - Passed", "node-1", nil, true},
		{"KnownNode - Failed", "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mDBClient.On("KnownNode", node.NodeId).Return(tt.mockKnown).Once()
			if tt.mockKnown == "" {
				mDBClient.On("SetKnownNode", node.NodeId, node.HostName).Return(tt.mockSetErr).Once()
			}

			result := manager.KnownNode(node)
			assert.Equal(t, tt.expectedRes, result)
		})
	}
}

func TestManager_subscribeNotifications(t *testing.T) {
	mMetaService = &service.MockMetaDataService{}
	mockSubscriptionClient := clientMocks.SubscriptionClient{}
	manager := setupManager("NODE-1", "NODE", mDBClient)
	subResp1 := []common.BaseWithIdResponse{
		{
			BaseResponse: common.BaseResponse{
				StatusCode: 200,
			},
		},
	}
	subResp2 := []common.BaseWithIdResponse{
		{
			BaseResponse: common.BaseResponse{
				StatusCode: 500,
			},
		},
	}

	tests := []struct {
		name         string
		subResponses []common.BaseWithIdResponse
		subErr       errors1.EdgeX
		expectedErr  hedgeErrors.HedgeError
	}{
		{"SubscribeNotifications - Passed", subResp1, nil, nil},
		{"SubscribeNotifications - Failed Subscription Setup", nil, errors1.NewCommonEdgeXWrapper(errors.New("subscription setup failed")), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Failed to subscribe to notifications")},
		{"SubscribeNotifications - Failed Status Code", subResp2, nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Failed to subscribe to notifications")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSubscriptionClient.On("Add", mock.Anything, mock.Anything).Return(tt.subResponses, tt.subErr).Once()
			appMock.AppService.On("SubscriptionClient").Return(&mockSubscriptionClient).Once()

			err := manager.subscribeNotifications()

			if tt.expectedErr != nil {
				assert.NotNil(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestManager_GetStateCoreVersion(t *testing.T) {
	expectedVersion := "v1.0.0"
	mDBClient.On("GetStateCoreVersion").Return(expectedVersion)
	manager := setupManager("NODE-1", "NODE", mDBClient)

	version := manager.GetStateCoreVersion()

	assert.Equal(t, expectedVersion, version)
	mDBClient.AssertExpectations(t)
}

func TestManager_SetStateCoreVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr hedgeErrors.HedgeError
	}{
		{"SetStateCoreVersion - Passed", "v1.0.0", nil},
		{"SetStateCoreVersion - Failed", "v1.0.0", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "DB error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mDBClient.On("SetStateCoreVersion", mock.Anything).Return(nil)
			manager := setupManager("NODE-1", "NODE", mDBClient)

			if tt.name == "SetStateCoreVersion - Failed" {
				mDBClient1 := &redis.MockDBClient{}
				mDBClient1.On("SetStateCoreVersion", mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "DB error"))
				manager = setupManager("NODE-1", "NODE", mDBClient1)
			}

			err := manager.SetStateCoreVersion(tt.version)

			assert.Equal(t, tt.wantErr, err)
			mDBClient.AssertExpectations(t)
		})
	}
}

func TestManager_notifyExternal(t *testing.T) {
	manager := setupManager("NODE-1", "NODE", mDBClient)

	validEvent := &pkg.MetaEvent{
		Type: "Node",
		Name: "Node1",
	}

	invalidEvent := &pkg.MetaEvent{}

	tests := []struct {
		name          string
		events        []*pkg.MetaEvent
		expectedError bool
	}{
		{
			name:          "Valid event - Successfully sent",
			events:        []*pkg.MetaEvent{validEvent},
			expectedError: false,
		},
		{
			name:          "Invalid event - Skipped",
			events:        []*pkg.MetaEvent{invalidEvent},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expectedError {
				mNotificationClient.On("SendNotification", mock.Anything, mock.Anything).Return([]common.BaseResponse{{StatusCode: 201}}, nil)
			}

			err := manager.notifyExternal(tt.events)

			if tt.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
