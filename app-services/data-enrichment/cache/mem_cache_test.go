package cache

import (
	"hedge/common/dto"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	mockClients "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
	"time"
)

var (
	mService      *mocks.ApplicationService
	mLogger       *logger.MockLogger
	mClient       *utils.MockClient
	mDeviceClient mockClients.DeviceClient
)

func setup() {
	mLogger = new(logger.MockLogger)
	mClient = utils.NewMockClient()

	mService = new(mocks.ApplicationService)
	mService.On("GetAppSetting", "CacheRefreshIntervalSecs").Return("1000", nil).Once()
	mService.On("GetAppSetting", "Device_Extn").Return("", nil).Once()
	mService.On("GetAppSetting", "MetaDataServiceUrl").Return("", nil).Once()
	mService.On("LoggingClient").Return(mLogger)

	mDeviceClient = mockClients.DeviceClient{}
	mDeviceClient.On("DeviceByName", mock.Anything, mock.Anything).Return(responses.DeviceResponse{}, nil)
	mService.On("DeviceClient").Return(&mDeviceClient)
}

func TestPubSub_FetchDeviceTags(t *testing.T) {
	devices := []map[string]interface{}{
		{
			"id":             "55b68fcf-0fd2-445a-9fae-670b37fb9274",
			"name":           "some_device",
			"description":    "Example of Device Virtual",
			"created":        1600927134931,
			"modified":       1600927134931,
			"adminState":     "UNLOCKED",
			"operatingState": "UP",
			"lastConnected":  0,
			"lastReported":   0,
			"labels":         []string{"device-virtual-example"},
			"location":       "",
			"serviceName":    "device-virtual",
			"profileName":    "Random-Boolean-Device",
			"autoEvents": map[string]interface{}{
				"interval":   "10s",
				"onChange":   false,
				"sourceName": "Bool",
			},
			"protocols": map[string]interface{}{
				"other": map[string]interface{}{
					"Address": "device-virtual-bool-01",
					"Port":    "300",
				},
			},
		},
	}

	configureMockServer := func(deviceMetadata map[string]string,
		deviceMetadataStatusCode int,
		deviceExtensionMetadata []map[string]interface{},
		deviceExtensionMetadataStatusCode int,
		deviceAllInfo map[string]interface{},
		deviceAllInfoStatusCode int) {
		mClient.RegisterExternalMockRestCall("/api/v3/metadata/device/biz/some_device", "GET", deviceMetadata, deviceMetadataStatusCode, nil)
		mClient.RegisterExternalMockRestCall("/api/v3/metadata/device/extn/some_device", "GET", deviceExtensionMetadata, deviceExtensionMetadataStatusCode, nil)
		mClient.RegisterExternalMockRestCall("/api/v3/device/all", "GET", deviceAllInfo, deviceAllInfoStatusCode, nil)
	}

	deviceMetadataFull := map[string]string{
		"devicename": "some_device",
		"foo":        "bar",
	}
	deviceMetadaEmpty := map[string]string{}

	deviceExtensionMetadataFull := []map[string]interface{}{{
		"field": "extension_field",
		"value": "extenstion_value",
	},
		{"field": "extension_field_2",
			"default": "default_value",
		}}
	deviceExtensionMetadataEmpty := []map[string]interface{}{}

	deviceAllInfoFull := map[string]interface{}{
		"apiVersion": "v2",
		"requestId":  "479439fa-0148-11eb-adc1-0242ac120002",
		"statusCode": 200,
		"totalCount": 3,
		"devices":    devices,
	}
	deviceAllInfoEmpty := deviceAllInfoFull
	deviceAllInfoEmpty["devices"] = []map[string]interface{}{}

	wantFull := map[string]interface{}{
		"devicename":        "some_device",
		"foo":               "bar",
		"extension_field":   "extenstion_value",
		"extension_field_2": "default_value",
	}
	wantEmpty := map[string]interface{}{}

	type resp struct {
		deviceMetadata          map[string]string
		deviceExtensionMetadata []map[string]interface{}
		deviceAllInfo           map[string]interface{}
	}

	type testStruct struct {
		resp
		name string
		want map[string]interface{}
	}

	tests := []testStruct{
		{
			name: "all requests return filled out data - resulting map should be found in cache - PASS",
			resp: resp{
				deviceMetadata:          deviceMetadataFull,
				deviceExtensionMetadata: deviceExtensionMetadataFull,
				deviceAllInfo:           deviceAllInfoFull,
			},
			want: wantFull,
		},
		{
			name: "all requests return empty data - resulting map should be found in cache - PASS",
			resp: resp{
				deviceMetadata:          deviceMetadaEmpty,
				deviceExtensionMetadata: deviceExtensionMetadataEmpty,
				deviceAllInfo:           deviceAllInfoEmpty},
			want: wantEmpty,
		},
		{
			name: "all requests return nil data - resulting map should be found in cache - PASS",
			resp: resp{
				deviceMetadata:          nil,
				deviceExtensionMetadata: nil,
				deviceAllInfo:           deviceAllInfoEmpty,
			},
			want: wantEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			configureMockServer(
				tt.resp.deviceMetadata, 200,
				tt.resp.deviceExtensionMetadata, 200,
				tt.resp.deviceAllInfo, 200)

			mc, _ := NewMemCache(mService, mClient, "test-node")
			resultMap, _ := mc.FetchDeviceTags("some_device")

			if !reflect.DeepEqual(resultMap, tt.want) {
				t.Errorf("resulting structure in cache = %v, want %v", resultMap, tt.want)
			}
		})
	}

	testsWithRequestsErrors := []testStruct{
		{
			name: "No API call is working - the function complete - no entry to be found in cache - PASS",
			resp: resp{
				deviceMetadata:          deviceMetadaEmpty,
				deviceExtensionMetadata: deviceExtensionMetadataEmpty,
				deviceAllInfo:           deviceAllInfoEmpty},
		},
	}

	for _, tt := range testsWithRequestsErrors {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			configureMockServer(
				deviceMetadaEmpty, 400,
				deviceExtensionMetadataEmpty, 400,
				deviceAllInfoEmpty, 200)

			mc, _ := NewMemCache(mService, mClient, "test-node")
			resultMap, _ := mc.FetchDeviceTags("some_device")
			assert.Empty(t, resultMap)
		})
	}
}

func TestPubSub_FetchProfileTags(t *testing.T) {
	configureMockServer := func(deviceMetadata map[string]map[string]string, deviceMetadataStatusCode int) {
		mClient.RegisterExternalMockRestCall("/api/v3/deviceprofile/name/some_profile", "GET", deviceMetadata, deviceMetadataStatusCode, nil)
	}

	profileFull := map[string]map[string]string{
		"profile": {
			"model":        "some_model",
			"manufacturer": "bar",
		},
	}

	profileEmpty := map[string]map[string]string{}

	wantFull := map[string]interface{}{
		"model":        "some_model",
		"manufacturer": "bar",
	}
	wantEmpty := map[string]interface{}{
		"model":        "",
		"manufacturer": "",
	}

	type resp struct {
		profile map[string]map[string]string
	}

	type testStruct struct {
		resp
		name string
		want map[string]interface{}
	}

	tests := []testStruct{
		{
			name: "all requests return filled out data - resulting map should be found in cache - PASS",
			resp: resp{
				profile: profileFull,
			},
			want: wantFull,
		},
		{
			name: "all requests return empty data - resulting map should be found in cache - PASS",
			resp: resp{
				profile: profileEmpty,
			},
			want: wantEmpty,
		},
		{
			name: "all requests return nil data - resulting map should be found in cache - PASS",
			resp: resp{
				profile: profileEmpty,
			},
			want: wantEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			configureMockServer(tt.resp.profile, 200)

			mc, _ := NewMemCache(mService, mClient, "test-node")
			resultMap, _ := mc.FetchProfileTags("some_profile")

			if !reflect.DeepEqual(resultMap, tt.want) {
				t.Errorf("resulting structure in cache = %v, want %v", resultMap, tt.want)
			}
		})
	}

	testsWithRequestsErrors := []testStruct{
		{
			name: "No API call is working - the function complete - no entry to be found in cache - PASS",
			resp: resp{
				profile: profileEmpty,
			},
		},
	}

	for _, tt := range testsWithRequestsErrors {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			configureMockServer(profileEmpty, 400)

			mc, _ := NewMemCache(mService, mClient, "test-node")
			resultMap, _ := mc.FetchDeviceTags("some_device")
			assert.Empty(t, resultMap)
		})
	}
}

func TestPubSub_FetchDownSamplingConfiguration(t *testing.T) {
	configureMockServer := func(downSamplingConf *dto.DownsamplingConfig, status int) {
		mClient.RegisterExternalMockRestCall("/api/v3/metadata/deviceprofile/some_profile/downsampling/config", "GET",
			downSamplingConf, status, nil)
	}

	type resp struct {
		downSampling *dto.DownsamplingConfig
	}

	downSamplingCfg := &dto.DownsamplingConfig{
		DefaultDataCollectionIntervalSecs: 2,
		DefaultDownsamplingIntervalSecs:   2,
		Aggregates: []dto.Aggregate{{
			FunctionName:         "avg",
			GroupBy:              "deviceName",
			SamplingIntervalSecs: 2,
		},
			{
				FunctionName:         "max",
				GroupBy:              "deviceName",
				SamplingIntervalSecs: 2,
			},
			{
				FunctionName:         "max",
				GroupBy:              "deviceName",
				SamplingIntervalSecs: 2,
			},
		},
	}

	wantFull := downSamplingCfg
	wantEmpty := &dto.DownsamplingConfig{}

	type testStruct struct {
		resp
		name string
		want *dto.DownsamplingConfig
	}

	tests := []testStruct{
		{
			name: "all requests return filled out data - resulting map should be found in cache - PASS",
			resp: resp{
				downSampling: downSamplingCfg,
			},
			want: wantFull,
		},
		{
			name: "all requests return empty data - resulting map should be found in cache - PASS",
			resp: resp{
				downSampling: &dto.DownsamplingConfig{},
			},
			want: wantEmpty,
		},
		{
			name: "all requests return nil data - resulting map should be found in cache - PASS",
			resp: resp{
				downSampling: nil,
			},
			want: wantEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			configureMockServer(tt.resp.downSampling, 200)

			mc, _ := NewMemCache(mService, mClient, "test-node")
			resultMap, _ := mc.FetchDownSamplingConfiguration("some_profile")

			assert.Equal(t, tt.want.DefaultDownsamplingIntervalSecs, resultMap.(dto.DownsamplingConfig).DefaultDownsamplingIntervalSecs)
			assert.Equal(t, tt.want.DefaultDataCollectionIntervalSecs, resultMap.(dto.DownsamplingConfig).DefaultDataCollectionIntervalSecs)
			assert.Equal(t, len(tt.want.Aggregates), len(resultMap.(dto.DownsamplingConfig).Aggregates))
		})
	}

	testsWithRequestsErrors := []testStruct{
		{
			name: "No API call is working - the function complete - no entry to be found in cache - PASS",
			resp: resp{
				downSampling: nil,
			},
		},
	}

	for _, tt := range testsWithRequestsErrors {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			configureMockServer(tt.resp.downSampling, 400)

			mc, _ := NewMemCache(mService, mClient, "test-node")
			resultMap, _ := mc.FetchProfileTags("some_profile")
			assert.Empty(t, resultMap)
		})
	}
}

func TestPubSub_FetchRawDataConfiguration(t *testing.T) {
	configureMockServer := func(rawDataConf map[string]dto.NodeRawDataConfig, status int) {
		mClient.RegisterExternalMockRestCall("/api/v3/node_mgmt/downsampling/rawdata", "GET",
			rawDataConf, status, nil)
	}

	type resp struct {
		rawDataConf map[string]dto.NodeRawDataConfig
	}

	nodeRawDataConfig := &dto.NodeRawDataConfig{
		SendRawData: true,
		StartTime:   0,
		EndTime:     time.Now().Unix() + 60*60, // 1-hour pause
		Node: dto.Node{
			NodeID: "node10",
			Host:   "node1.bmc.com",
		},
	}

	wantFull := nodeRawDataConfig
	wantEmpty := &dto.NodeRawDataConfig{}

	type testStruct struct {
		resp
		name string
		want *dto.NodeRawDataConfig
	}

	tests := []testStruct{
		{
			name: "all requests return filled out data - resulting map should be found in cache - PASS",
			resp: resp{
				rawDataConf: map[string]dto.NodeRawDataConfig{"node10": *nodeRawDataConfig},
			},
			want: wantFull,
		},
		{
			name: "all requests return empty data - resulting map should be found in cache - PASS",
			resp: resp{
				rawDataConf: map[string]dto.NodeRawDataConfig{"node10": dto.NodeRawDataConfig{}},
			},
			want: wantEmpty,
		},
		{
			name: "all requests return nil data - resulting map should be found in cache - PASS",
			resp: resp{
				rawDataConf: nil,
			},
			want: wantEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			configureMockServer(tt.resp.rawDataConf, 200)

			mc, _ := NewMemCache(mService, mClient, "node10")
			resultMap, _ := mc.FetchRawDataConfiguration()

			//convert to milliseconds
			tt.want.EndTime *= 1000000000
			tt.want.StartTime *= 1000000000

			assert.Equal(t, tt.want.SendRawData, resultMap.(dto.NodeRawDataConfig).SendRawData)
			assert.Equal(t, tt.want.EndTime, resultMap.(dto.NodeRawDataConfig).EndTime)
			assert.Equal(t, tt.want.StartTime, resultMap.(dto.NodeRawDataConfig).StartTime)
			assert.Equal(t, tt.want.Node, resultMap.(dto.NodeRawDataConfig).Node)
		})
	}

	testsWithRequestsErrors := []testStruct{
		{
			name: "No API call is working - the function complete - no entry to be found in cache - PASS",
			resp: resp{
				rawDataConf: map[string]dto.NodeRawDataConfig{"node10": dto.NodeRawDataConfig{}},
			},
		},
	}

	for _, tt := range testsWithRequestsErrors {
		t.Run(tt.name, func(t *testing.T) {
			setup()
			configureMockServer(tt.resp.rawDataConf, 400)

			mc, _ := NewMemCache(mService, mClient, "test-node")
			resultMap, _ := mc.FetchProfileTags("some_profile")
			assert.Empty(t, resultMap)
		})
	}
}
