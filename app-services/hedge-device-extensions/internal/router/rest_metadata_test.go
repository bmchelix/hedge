/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package router

import (
	"hedge/app-services/hedge-device-extensions/internal/util"
	redis2 "hedge/app-services/hedge-device-extensions/pkg/db/redis"
	"hedge/common/client"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	bmcmodel "hedge/common/models"
	"hedge/common/telemetry"
	"hedge/mocks/hedge/app-services/hedge-device-extensions/pkg/db/redis"
	mds "hedge/mocks/hedge/app-services/hedge-device-extensions/pkg/service"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	mservice "hedge/mocks/hedge/common/service"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	inter "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/go-redsync/redsync/v4"
	"github.com/labstack/echo/v4"
	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	ExampleUUID           = "82eb2e26-0f24-48aa-ae4c-de9dac3fb9bc"
	TestDeviceProfileName = "TestDeviceProfileName"
	TestDeviceName        = "TestDevice"
	TestDeviceServiceName = "TestDeviceServiceName"
)

var mockMetaService = &mds.MockMetaDataService{}
var dsi = &mservice.MockDeviceServiceInter{}
var metricsManager *telemetry.MetricsManager
var router *Router
var router2 *Router
var router3 *Router
var u *utils.HedgeMockUtils
var dbc redis.MockDeviceExtDBClientInterface
var nodeHttpClient *utils.MockClient
var dev = map[string]interface{}{
	"id":             "55b68fcf-0fd2-445a-9fae-670b37fb9274",
	"name":           TestDeviceName,
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
	"profileName":    TestDeviceProfileName,
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
}
var devices []interface{}
var devserv = map[string]interface{}{
	"apiVersion": "v2",
	"requestId":  "479439fa-0148-11eb-adc1-0242ac120002",
	"statusCode": 200,
	"totalCount": 3,
	"devices":    "",
}
var serv = map[string]interface{}{
	"id":            "1ff7762f-c432-4af0-9a5d-756bbc92744b",
	"name":          "device-virtual",
	"created":       1600927134890,
	"modified":      1600927134890,
	"description":   "Example",
	"lastConnected": 0,
	"lastReported":  0,
	"adminState":    "UNLOCKED",
	"labels":        []string{"virtual"},
	"baseAddress":   "http://edgex-device-virtual:59990",
}
var servarray []interface{}
var services = map[string]interface{}{
	"apiVersion": "v2",
	"requestId":  "4c7c47a7-10e0-4489-99c5-f639b7d7eb5c",
	"statusCode": 200,
	"totalCount": 3,
	"services":   "",
}
var tel = &Telemetry{
	contextualDataRequests: nil,
	contextualDataSize:     nil,
}

func init() {
	devices = append(devices, dev)
	devserv["devices"] = devices
	servarray = append(servarray, serv)
	services["services"] = servarray
	nodeHttpClient = utils.NewMockClient()
	client.Client = nodeHttpClient
	nodeHttpClient.RegisterExternalMockRestCall("MockMetaDataServiceUrl/api/v3/device/all", http.MethodGet, &devserv, 200, nil)
	nodeHttpClient.RegisterExternalMockRestCall("MockMetaDataServiceUrl/api/v3/deviceservice/all", http.MethodGet, &services, 200, nil)
	nodeHttpClient.RegisterExternalMockRestCall("MockMetaDataServiceUrl/api/v3/deviceservice/name/dev", http.MethodGet, &services, 200, nil)
	nodeHttpClient.RegisterExternalMockRestCall("mockUrl/api/v3/deviceservice/all", http.MethodGet, &services, 400, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	redis2.DBClientImpl = &dbc
	dbc.On("GetDbClient", mock.Anything, mock.Anything).Return(redis2.DBClientImpl)
	dbc.On("GetMetricCounter", mock.Anything).Return(int64(0), nil)
	dbc.On("SetMetricCounter", mock.Anything, mock.Anything).Return(nil)
	dbc.On("IncrMetricCounterBy", mock.Anything, mock.Anything).Return(int64(11), nil)
	dbc.On("AcquireRedisLock", mock.Anything).Return(&redsync.Mutex{}, nil)

	u = utils.NewApplicationServiceMock(map[string]string{"HedgeAdminURL": "http://hedge-admin:4321", "MetricReportInterval": "1", "MetricPublishTopicPrefix": "SomeTopic",
		"MetaDataServiceUrl": "MockMetaDataServiceUrl", "MetaSyncUrl": "MockMetaSyncUrl"})
	metricsManager, _ = telemetry.NewMetricsManager(u.AppService, client.HedgeDeviceExtnsServiceName)
	router = NewRouter(u.AppService, "TestService", metricsManager.MetricsMgr)
	router2 = NewRouter(u.AppService, "TestService", metricsManager.MetricsMgr)
	router3 = NewRouter(u.AppService, "TestService", metricsManager.MetricsMgr)
	router.metaService = mockMetaService
	//router.deviceInfoService = dsi.GetDeviceInfoService("localhost")
	router2.metaDataServiceURL = "mockUrl"
	router3.currentNodeId = "localhost"
	g := metrics.NewCounter()
	tel.contextualDataRequests = g
	tel.contextualDataSize = g
	router.telemetry = tel
}

// Gen test device
func buildTestDeviceRequest() dto.DeviceObject {
	var testAutoEvents = []dtos.AutoEvent{
		{SourceName: "TestResource", Interval: "300ms", OnChange: true},
	}
	var testProtocols = map[string]dtos.ProtocolProperties{
		"modbus-ip": {
			"Address": "localhost",
			"Port":    "1502",
			"UnitID":  "1",
		},
	}
	var Device = dtos.Device{
		Id:             ExampleUUID,
		Name:           TestDeviceName,
		ServiceName:    TestDeviceServiceName,
		ProfileName:    TestDeviceProfileName,
		AdminState:     models.Locked,
		OperatingState: models.Up,
		Labels:         []string{"MODBUS", "TEMP"},
		Location:       "{40lat;45long}",
		AutoEvents:     testAutoEvents,
		Protocols:      testProtocols,
	}
	var device = dto.DeviceObject{
		ApiVersion:           "V2",
		Device:               Device,
		Node:                 bmcmodel.Node{},
		Associations:         nil,
		Extensions:           nil,
		ContextualAttributes: nil,
	}
	return device
}

// Gen test profile
func buildTestDeviceProfileRequest() dto.ProfileObject {
	var Profile = dtos.DeviceProfile{
		DBTimestamp: dtos.DBTimestamp{},
		DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
			Id:           "abc123",
			Name:         "Random-Boolean-Device",
			Manufacturer: "The Machine",
			Description:  "Test profile",
			Model:        "F-16",
			Labels:       nil,
		},
		DeviceResources: []dtos.DeviceResource{
			{
				Name: "device_resource_1",
			},
		},
		DeviceCommands: nil,
	}
	var profile = dto.ProfileObject{
		ApiVersion: "V2",
		Profile:    Profile,
		ProfileExtension: dto.ProfileExtension{
			DeviceAttributes: []dto.DeviceExtension{{
				Field:       "some field",
				Default:     "default field",
				IsMandatory: false,
			}},
			ContextualAttributes: nil,
		},
	}
	return profile
}

func buildTestDeviceProfileRequestForDeleteExt() dto.ProfileObject {
	var Profile = dtos.DeviceProfile{
		DBTimestamp: dtos.DBTimestamp{},
		DeviceProfileBasicInfo: dtos.DeviceProfileBasicInfo{
			Id:           "abc123",
			Name:         TestDeviceProfileName,
			Manufacturer: "The Machine",
			Description:  "Test profile",
			Model:        "F-16",
			Labels:       nil,
		},
		DeviceResources: []dtos.DeviceResource{
			{
				Name: "device_resource_1",
			},
		},
		DeviceCommands: nil,
	}
	var profile = dto.ProfileObject{
		ApiVersion: "V2",
		Profile:    Profile,
		ProfileExtension: dto.ProfileExtension{
			DeviceAttributes: []dto.DeviceExtension{
				{
					Field:       "att1",
					Default:     "default field",
					IsMandatory: false,
				},
				{
					Field:       "att2",
					Default:     "default field",
					IsMandatory: false,
				},
			},
			ContextualAttributes: nil,
		},
	}
	return profile
}

func Test_deleteDeviceContextualData(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.Header.Set("Content-Type", "application/json")

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("DeleteDeviceContextualAttributes", "device").Return(nil)
	mockMetaService.On("DeleteDeviceContextualAttributes", "Test").Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "error"))

	tests := []struct {
		name       string
		deviceName string
		wantErr    bool
		errMessage string
	}{
		{"DeleteDeviceContextualData - Passed", deviceName, false, ""},
		{"DeleteDeviceContextualData - Failed", "Test", true, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/:device")
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)

			err := deleteDeviceContextualData(c, mockRouter)
			if (err != nil) != tt.wantErr {
				t.Errorf("deleteDeviceBusinessData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_deleteProfileContextualData(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	req.Header.Set("Content-Type", "application/json")

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("DeleteProfileContextualAttributes", profileName).Return(nil)
	mockMetaService.On("DeleteProfileContextualAttributes", "Test").Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeUnknown, ""))

	tests := []struct {
		name        string
		profileName string
		wantErr     bool
		errMessage  string
	}{
		{"DeleteProfileContextualAttrb - Passed", profileName, false, ""},
		{"DeleteProfileContextualAttrb - Failed", "Test", true, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodDelete, "/", nil), rec)
			c.SetPath("/:profile")
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			if err := deleteProfileContextualData(c, mockRouter); (err != nil) != tt.wantErr {
				t.Errorf("deleteProfileContextualData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_filterByNodeId(t *testing.T) {
	type args struct {
		arr  []interface{}
		node string
	}
	arr := []interface{}{map[string]interface{}{"name": "test"}}
	arg := args{
		arr:  arr,
		node: "test",
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"filterByNode - Passed", arg, []string{"test"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterByNodeId(tt.args.arr, tt.args.node); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterByNodeId() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getAttributesGroupedByProfiles(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("GetAttributesGroupedByProfiles", []string{profileName}).Return(map[string][]string{}, nil)
	mockMetaService.On("GetAttributesGroupedByProfiles", []string{"test1", "test2"}).Return(map[string][]string{}, nil)
	mockMetaService.On("GetAttributesGroupedByProfiles", []string{"test3", "test4"}).Return(map[string][]string{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "error"))

	tests := []struct {
		name       string
		queryParam string
		wantErr    bool
	}{
		{"getAttributesGroupedByProfiles - Passed", profileName, false},
		{"getAttributesGroupedByProfiles - Passed2", "test1,test2", false},
		{"getAttributesGroupedByProfiles - Failed", "test3,test4", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?profiles="+tt.queryParam, nil)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := getAttributesGroupedByProfiles(c, mockRouter); (err != nil) != tt.wantErr {
				t.Errorf("getAttributesGroupedByProfiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getDeviceContextualData(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	bizData := map[string]interface{}{"Field1": deviceName, "Field2": TestDeviceName}
	mockMetaService.On("GetDeviceContextualAttributes", deviceName).Return(bizData, nil)
	mockMetaService.On("GetDeviceContextualAttributes", "Test").Return(map[string]interface{}{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name       string
		deviceName string
		wantErr    bool
	}{
		{"GetDeviceContextualData - Passed", deviceName, false},
		{"GetDeviceContextualData - Failed", "Test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?device=%s", tt.deviceName), nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/:device")
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)

			err := getDeviceContextualData(c, mockRouter)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDeviceContextualData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getDeviceServiceProtocols(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	protocols := map[string]string{"prot1": "someProtocol"}
	mockMetaService.On("GetProtocolsForService", serviceName).Return(protocols, nil)
	mockMetaService.On("GetProtocolsForService", "Test").Return(map[string]string{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name        string
		serviceName string
		wantErr     bool
	}{
		{"getDeviceServiceProtocols - Passed", serviceName, false},
		{"getDeviceServiceProtocols - Failed", "Test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?service=%s", tt.serviceName), nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/:service")
			c.SetParamNames("service")
			c.SetParamValues(tt.serviceName)

			if err := getDeviceServiceProtocols(c, mockRouter); (err != nil) != tt.wantErr {
				t.Errorf("getDeviceServiceProtocols() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getDeviceServices(t *testing.T) {
	type args struct {
		r      Router
		nodeId string
	}
	var emptyArray []string
	var servicearray []string
	servicearray = append(servicearray, "device-virtual")
	var devserv *mservice.MockDeviceServiceInter
	arg := args{
		r:      *router,
		nodeId: router.currentNodeId,
	}
	arg2 := args{
		r:      *router2,
		nodeId: router2.currentNodeId,
	}
	arg3 := args{
		r:      *router3,
		nodeId: "",
	}
	arg4 := args{
		r:      *router3,
		nodeId: "test",
	}
	dsi.On("GetDeviceService", router.metaDataServiceURL).Return(devserv)
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{"getDeviceService - Passed", arg, servicearray, false},
		{"getDeviceService - Failed", arg2, emptyArray, true},
		{"getDeviceService - Failed", arg3, servicearray, false},
		{"getDeviceService - Failed", arg4, []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDeviceServices(tt.args.r, tt.args.nodeId)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDeviceServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDeviceServices() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDeviceService(t *testing.T) {
	e := echo.New()
	mockRouter := *router
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"getDeviceService - Passed", "1ff7762f-c432-4af0-9a5d-756bbc92744b", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?edgeNode=dev", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			res, err := getDeviceService(c, mockRouter, "dev")
			if (err != nil) != tt.wantErr {
				t.Errorf("getDeviceServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			got := res.([]interface{})
			got0 := got[0].(map[string]interface{})
			id := got0["id"]

			if id != tt.want {
				t.Errorf("getDeviceServices() id = %v, want %v", id, tt.want)
			}
		})
	}
}

func Test_getDevicesSummary(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	page1 := bmcmodel.Page{
		Size:   5,
		Number: 1,
		Total:  0,
	}
	page2 := bmcmodel.Page{
		Size:   15,
		Number: 1,
		Total:  0,
	}
	var devsum = make([]bmcmodel.DeviceSummary, 0)

	tests := []struct {
		name       string
		edgeNodeID string
		wantErr    hedgeErrors.HedgeError
		page       bmcmodel.Page
		summary    []bmcmodel.DeviceSummary
	}{
		{"getDevicesSummary - Passed", router.currentNodeId, nil, page1, devsum},
		{"getDevicesSummary - different page - Passed", router.currentNodeId, nil, page2, devsum},
		{"getDevicesSummary - Failed", router.currentNodeId, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Internal Server Error"), page1, devsum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMetaService.On("GetDevices", mock.AnythingOfType("*models.Query")).Return(tt.summary, tt.page, tt.wantErr).Once()

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?edgeNode=%s", tt.edgeNodeID), nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			if err := getDevicesSummary(c, mockRouter); err != nil {
				if reflect.DeepEqual(err, tt.wantErr.ConvertToHTTPError()) {
					return
				}

				t.Errorf("getDevicesSummary() error = %v, wantErr %v", err, tt.wantErr)
				t.Fail()
			}

			response := rec.Body.Bytes()
			responseMap := make(map[string]interface{})
			json.Unmarshal(response, &responseMap)

			responseDataValue, _ := json.Marshal(responseMap["data"])
			responsePageValue, _ := json.Marshal(responseMap["page"])

			gotDeviceSummary := make([]bmcmodel.DeviceSummary, 0)
			var gotPage bmcmodel.Page

			json.Unmarshal(responseDataValue, &gotDeviceSummary)
			json.Unmarshal(responsePageValue, &gotPage)

			if !reflect.DeepEqual(gotDeviceSummary, tt.summary) {
				t.Errorf("getDevicesSummary() got = %v, want %v", gotDeviceSummary, tt.summary)
			}

			if !reflect.DeepEqual(gotPage, tt.page) {
				t.Errorf("getDevicesSummary() got = %v, want %v", gotPage, tt.page)
			}
		})
	}
}

func Test_getGivenDevicesSummary(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	devList := []string{deviceName}
	device := dtos.Device{
		Name: "Device",
	}
	mockMetaService.On("GetDeviceDetails", deviceName).Return(device, "", nil)
	mockMetaService.On("GetDeviceDetails", "Test").Return(dtos.Device{}, "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name    string
		devList []string
		wantErr bool
	}{
		{"GetGivenDevSum - Passed", devList, false},
		{"GetGivenDevSum - Failed", []string{"Test"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonByte, _ := json.Marshal(tt.devList)
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(jsonByte))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := getGivenDevicesSummary(c, mockRouter); (err != nil) != tt.wantErr {
				t.Errorf("getGivenDevicesSummary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getProfileBusinessData(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	bizAttr := []string{"attrb1", "attrb2", "attrb3"}
	mockMetaService.On("GetProfileContextualAttributes", profileName).Return(bizAttr, nil)
	mockMetaService.On("GetProfileContextualAttributes", "Test").Return([]string{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name        string
		profileName string
		wantErr     bool
	}{
		{"GetProfileContextualAttrb - Passed", profileName, false},
		{"GetProfileContextualAttrb - Failed", "Test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?profile=%s", tt.profileName), nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/:profile")
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			if err := getProfileContextualData(c, mockRouter); (err != nil) != tt.wantErr {
				t.Errorf("getProfileContextualData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getProfileMetaDataSummary(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	arrprofsum := []dto.ProfileSummary{{
		Name:                 "",
		Description:          "",
		MetricNames:          nil,
		DeviceAttributes:     nil,
		ContextualAttributes: nil,
	}}
	mockMetaService.On("GetProfileMetaDataSummary", []string{profileName}).Return(arrprofsum, nil)
	mockMetaService.On("GetProfileMetaDataSummary", []string{"test"}).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name         string
		profileQuery string
		wantErr      bool
	}{
		{"GetProfileMetaDataSummary - Passed", profileName, false},
		{"GetProfileMetaDataSummary - Failed", "test", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/?profiles=%s", tt.profileQuery), nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := getProfileMetaDataSummary(c, mockRouter); (err != nil) != tt.wantErr {
				t.Errorf("getProfileMetaDataSummary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getRelatedProfiles(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	profArray := []string{profileName}
	mockMetaService.On("GetRelatedProfiles", []string{profileName}).Return(profArray, nil)
	mockMetaService.On("GetRelatedProfiles", []string{"test"}).Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name         string
		profileQuery string
		wantErr      bool
	}{
		{"GetRelatedProfiles - Passed", profileName, false},
		{"GetRelatedProfiles - Failed", "test", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?profiles=%s", tt.profileQuery), nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := getRelatedProfiles(c, mockRouter); (err != nil) != tt.wantErr {
				t.Errorf("getRelatedProfiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_restAddAssociations(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	associatedNodes := []dto.AssociationNode{{NodeType: deviceName, NodeName: TestDeviceName}}
	mockMetaService.On("AddAssociation", "TestName", deviceName, associatedNodes).Return("lkj-87h-sd7", nil)
	mockMetaService.On("DeleteAssociation", "", deviceName).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "error"))
	mockMetaService.On("DeleteAssociation", "DeletePassed", deviceName).Return(nil)

	tests := []struct {
		name       string
		nodeAType  string
		nodeAName  string
		payload    interface{}
		wantErr    bool
		wantStatus int
	}{
		{"AddAssociations - Passed", deviceName, "TestName", associatedNodes, false, http.StatusOK},
		{"AddAssociations - Failed", deviceName, "", nil, true, http.StatusBadRequest},
		{"AddAssociations - Passed2", deviceName, "DeletePassed", nil, false, http.StatusOK},
		{"AddAssociations - Failed2", deviceName, "", nil, true, http.StatusBadRequest},
		{"AddAssociations - Failed3", "Test", "", nil, true, http.StatusNotImplemented},
		{"AddAssociations - Failed4", deviceName, "TestName1", associatedNodes, true, http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody io.Reader
			if tt.payload != nil {
				b, _ := json.Marshal(tt.payload)
				reqBody = bytes.NewReader(b)
			}

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/?nodeAType=%s&nodeAName=%s", tt.nodeAType, tt.nodeAName), reqBody)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("nodeAType", "nodeAName")
			c.SetParamValues(tt.nodeAType, tt.nodeAName)

			if strings.Contains(tt.name, "AddAssociations - Failed4") {
				mockMetaService1 := &mds.MockMetaDataService{}
				mockRouter.metaService = mockMetaService1
				mockMetaService1.On("AddAssociation", "TestName1", deviceName, associatedNodes).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))
			}

			if err := restAddAssociations(c, mockRouter); err != nil {
				if !tt.wantErr {
					t.Errorf("restAddAssociations() error = %v, wantErr %v", err, tt.wantErr)
				}

				if err.Code != tt.wantStatus {
					t.Errorf("restAddAssociations() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_restAddDeviceExt(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	deviceExt := []dto.DeviceExt{{Field: deviceName, Value: TestDeviceName}}
	jsonByte, _ := json.Marshal(deviceExt)
	payload := strings.NewReader(string(jsonByte))
	mockMetaService.On("AddDeviceExtension", deviceName, deviceExt).Return("someID", nil)

	tests := []struct {
		name       string
		deviceName string
		payload    io.Reader
		wantErr    bool
		wantStatus int
	}{
		{"AddDeviceExtensions - Passed", deviceName, payload, false, http.StatusOK},
		{"AddDeviceExtensions - Failed", deviceName, io.NopCloser(ErrorReader{}), true, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", tt.payload)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)

			if err := restAddDeviceExt(c, mockRouter); err != nil {
				if !tt.wantErr {
					t.Errorf("restAddDeviceExt() error = %v, wantErr %v", err, tt.wantErr)
				}

				if err.Code != tt.wantStatus {
					t.Errorf("restAddDeviceExt() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_restAddDvcProfileExt(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	profileExt := []dto.DeviceExtension{{Field: profileName, Default: "testDefault"}}
	profileExt1 := []dto.DeviceExtension{{Field: "Test", Default: "testDefault"}}
	jsonByte, _ := json.Marshal(profileExt)
	jsonByte1, _ := json.Marshal(profileExt1)
	payload := strings.NewReader(string(jsonByte))
	payload1 := strings.NewReader(string(jsonByte1))

	mockMetaService.On("AddDeviceExtensionsInProfile", profileName, profileExt).Return("someID", nil)
	mockMetaService.On("AddDeviceExtensionsInProfile", "Test", profileExt1).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, ""))

	tests := []struct {
		name        string
		profileName string
		payload     io.Reader
		wantErr     bool
		wantStatus  int
	}{
		{"AddDeviceExtensionsInProfile - Passed", profileName, payload, false, http.StatusOK},
		{"AddDeviceExtensionsInProfile - Failed", "Test", payload1, true, http.StatusNotFound},
		{"AddDeviceExtensionsInProfile - Failed1", "Test", io.NopCloser(ErrorReader{}), true, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", tt.payload)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			if err := restAddDvcProfileExt(c, mockRouter); (err != nil) != tt.wantErr {
				if !tt.wantErr {
					t.Errorf("restAddDvcProfileExt() error = %v, wantErr %v", err, tt.wantErr)

					if err.Code != tt.wantStatus {
						t.Errorf("restAddDvcProfileExt() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
					}
				}
			}
		})
	}
}

func Test_restCreateCompleteDevice(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	device := buildTestDeviceRequest()
	jsonByte, _ := json.Marshal(device)
	payload := strings.NewReader(string(jsonByte))
	mockMetaService.On("CreateCompleteDevice", device.Device.Name, mock.AnythingOfType("dto.DeviceObject")).Return("ukwhdf-435-fds4-sdf9", nil)

	tests := []struct {
		name       string
		deviceName string
		payload    io.Reader
		wantErr    bool
		wantStatus int
	}{
		{"CreateDevice - Passed", device.Device.Name, payload, false, http.StatusOK},
		{"CreateDevice - FailedMissingDeviceName", "", payload, true, http.StatusBadRequest},
		{"CreateDevice - FailedInvalidDeviceName", "Invalid Name!", nil, true, http.StatusBadRequest},
		{"CreateDevice - FailedServiceError", "test", payload, true, http.StatusBadRequest},
		{"CreateDevice - FailedServiceError1", "test1", io.NopCloser(ErrorReader{}), true, http.StatusBadRequest},
		{"CreateDevice - FailedDeviceNameExceededLimit", "123456789012345678901234567890", payload, true, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", tt.payload)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)

			if err := restCreateCompleteDevice(c, mockRouter); (err != nil) != tt.wantErr {
				if !tt.wantErr {
					t.Errorf("restCreateCompleteDevice() error = %v, wantErr %v", err, tt.wantErr)
				}

				if err.Code != tt.wantStatus {
					t.Errorf("restCreateCompleteDevice() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_restCreateCompleteProfile(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	profile := buildTestDeviceProfileRequest()
	profile1 := buildTestDeviceProfileRequest()
	profile1.Profile.DeviceResources[0].Name = "1234567890123456789012345678901234567890"
	jsonByte, _ := json.Marshal(profile)
	jsonByte1, _ := json.Marshal(profile1)
	mockMetaService.On("CreateCompleteProfile", profile.Profile.Name, mock.AnythingOfType("dto.ProfileObject")).Return("wer-45jhds-435kjhij", nil)
	mockMetaService.On("CreateCompleteProfile", "test", mock.Anything).Return("", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name        string
		profileName string
		payload     io.Reader
		wantErr     bool
		wantStatus  int
		wantID      string
	}{
		{"CreateProfile - Passed", profile.Profile.Name, bytes.NewReader(jsonByte), false, http.StatusOK, "wer-45jhds-435kjhij"},
		{"CreateProfile - Failed", "test", bytes.NewReader(jsonByte), true, http.StatusConflict, ""},
		{"CreateProfile - FailedMissingBody", profile.Profile.Name, io.NopCloser(ErrorReader{}), true, http.StatusBadRequest, ""},
		{"CreateProfile - FailedDeviceResourceNameExceededLimit", profile.Profile.Name, bytes.NewReader(jsonByte1), true, http.StatusBadRequest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", tt.payload)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			if err := restCreateCompleteProfile(c, mockRouter); err != nil {
				if !tt.wantErr {
					t.Errorf("restCreateCompleteProfile() error = %v, wantErr %v", err, tt.wantErr)
				}

				if err.Code != tt.wantStatus {
					t.Errorf("restCreateCompleteProfile() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
			if tt.wantID != "" {
				var result map[string]string
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result["id"] != tt.wantID {
					t.Errorf("restCreateCompleteProfile() got ID = %v, wantID %v", result["id"], tt.wantID)
				}
			}
		})
	}
}

func Test_restDeleteAssociations(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("DeleteAssociation", "TestName", deviceName).Return(nil)
	mockMetaService.On("DeleteAssociation", "Other", deviceName).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))
	mockMetaService.On("DeleteAssociation", "Other", "test").Return(nil)

	tests := []struct {
		name       string
		nodeAType  string
		nodeAName  string
		wantStatus int
	}{
		{"DeleteAssociations - Passed", deviceName, "TestName", http.StatusOK},
		{"DeleteAssociations - Failed", deviceName, "Other", http.StatusConflict},
		{"DeleteAssociations - NotImplemented", "test", "Other", http.StatusNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/?nodeAType=%s&nodeAName=%s", tt.nodeAType, tt.nodeAName), nil)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("nodeAType", "nodeAName")
			c.SetParamValues(tt.nodeAType, tt.nodeAName)

			if err := restDeleteAssociations(c, mockRouter); err != nil {
				if tt.wantStatus == 200 {
					t.Errorf("restDeleteAssociations() error = %v", err)
				}

				if err.Code != tt.wantStatus {
					t.Errorf("restDeleteAssociations() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_restDeleteCompleteDevice(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("DeleteCompleteDevice", "TestDevice").Return(nil)
	mockMetaService.On("DeleteCompleteDevice", "fail").Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name       string
		deviceName string
		edgeNode   string
		wantStatus int
		wantErr    bool
	}{
		{"DeleteDevice - Passed", "TestDevice", "node-test-001", http.StatusOK, false},
		{"DeleteDevice - Failed", "fail", "", http.StatusConflict, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/?device=%s&edgeNode=%s", tt.deviceName, tt.edgeNode), nil)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("device", "edgeNode")
			c.SetParamValues(tt.deviceName, tt.edgeNode)

			if err := restDeleteCompleteDevice(c, mockRouter); err != nil {
				if !tt.wantErr {
					t.Errorf("restDeleteCompleteDevice() error = %v, wantErr %v", err, tt.wantErr)
				}
				if err.Code != tt.wantStatus {
					t.Errorf("restDeleteCompleteDevice() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_restDeleteCompleteProfile(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("DeleteCompleteProfile", "TestProfile").Return(nil)
	mockMetaService.On("DeleteCompleteProfile", "Test").Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, ""))

	tests := []struct {
		name        string
		profileName string
		wantStatus  int
		wantErr     bool
	}{
		{"DeleteProfile - Passed", "TestProfile", http.StatusOK, false},
		{"DeleteProfile - Failed", "Test", http.StatusBadRequest, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/", nil)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			if err := restDeleteCompleteProfile(c, mockRouter); err != nil {
				if !tt.wantErr {
					t.Errorf("restDeleteCompleteProfile() error = %v, wantErr %v", err, tt.wantErr)
				}

				if err.Code != tt.wantStatus {
					t.Errorf("restDeleteCompleteProfile() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_restDeleteAllDeviceExt(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("DeleteAllDeviceExtension", deviceName).Return(nil)
	mockMetaService.On("DeleteAllDeviceExtension", "test").Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name       string
		deviceName string
		wantStatus int
		wantErr    bool
	}{
		{"DeleteAllDeviceExtensions - Passed", deviceName, http.StatusOK, false},
		{"DeleteAllDeviceExtensions - Failed", "test", http.StatusConflict, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/"+tt.deviceName, nil)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)

			if err := restDeleteAllDeviceExt(c, mockRouter); err != nil {
				if !tt.wantErr {
					t.Errorf("restDeleteAllDeviceExt() error = %v, wantErr %v", err, tt.wantErr)
				}
				if err.Code != tt.wantStatus {
					t.Errorf("restDeleteAllDeviceExt() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_restDeleteDvcProfileExt(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("DeleteDeviceExtensionInProfile", profileName).Return(nil)
	mockMetaService.On("DeleteDeviceExtensionInProfile", "Test").Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "Not Found"))
	dsi.On("GetDevicesByProfile", profileName).Return([]string{})
	dsi.On("GetDevicesByProfile", "Test").Return([]string{"device1", "device2"})

	tests := []struct {
		name        string
		profileName string
		wantStatus  int
		wantErr     bool
	}{
		{"DeleteProfiles - Passed", profileName, http.StatusOK, false},
		{"DeleteProfiles - Failed", "Test", http.StatusNotFound, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/%s", tt.profileName), nil)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			if err := restDeleteDvcProfileExt(c, mockRouter); err != nil {
				if !tt.wantErr {
					t.Errorf("restDeleteDvcProfileExt() error = %v, wantErr %v", err, tt.wantErr)
				}

				if err.Code != tt.wantStatus {
					t.Errorf("restDeleteDvcProfileExt() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_restGetAssociations(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	associatedNodes := []dto.AssociationNode{{NodeType: deviceName, NodeName: TestDeviceName}}
	mockMetaService.On("GetAssociation", "TestName").Return(associatedNodes, nil)
	mockMetaService.On("GetAssociation", "OtherDevice").Return(nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name       string
		nodeAType  string
		nodeAName  string
		wantStatus int
		wantBody   interface{}
		wantErr    bool
	}{
		{"GetAssociations - Passed", deviceName, "TestName", http.StatusOK, associatedNodes, false},
		{"GetAssociations - Failed", deviceName, "OtherDevice", http.StatusBadRequest, nil, true},
		{"GetAssociations - NotImplemented", "test", "Device", http.StatusNotImplemented, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/?nodeAType=%s&nodeAName=%s", tt.nodeAType, tt.nodeAName), nil)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("nodeAType", "nodeAName")
			c.SetParamValues(tt.nodeAType, tt.nodeAName)

			if err := restGetAssociations(c, mockRouter); (err != nil) != tt.wantErr {
				if !tt.wantErr {
					t.Errorf("restGetAssociations() error = %v, wantErr %v", err, tt.wantErr)
				}

				if err.Code != tt.wantStatus {
					t.Errorf("restGetAssociations() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}

			if tt.wantBody != nil {
				var gotBody []dto.AssociationNode
				if err := json.Unmarshal(rec.Body.Bytes(), &gotBody); err != nil {
					t.Errorf("restGetAssociations() unable to parse response body")
				}
				if !reflect.DeepEqual(gotBody, tt.wantBody) {
					t.Errorf("restGetAssociations() gotBody = %v, wantBody %v", gotBody, tt.wantBody)
				}
			}
		})
	}
}

func Test_restGetCompleteDevice(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	device := buildTestDeviceRequest()
	tests := []struct {
		name             string
		deviceName       string
		expectStatusCode int
		expectedResponse dto.DeviceObject
		returnError      error
	}{
		{"GetCompleteDevice - Passed", device.Device.Name, http.StatusOK, device, nil},
		{"GetCompleteDevice - Failed", "test", http.StatusBadRequest, dto.DeviceObject{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)
			mockMetaService.On("GetCompleteDevice", tt.deviceName, "", router.service).Return(tt.expectedResponse, tt.returnError)

			if err := restGetCompleteDevice(c, mockRouter); err != nil {
				assert.Equal(t, tt.expectStatusCode, err.Code)
				if tt.expectStatusCode == http.StatusOK {
					var got dto.DeviceObject
					json.Unmarshal(rec.Body.Bytes(), &got)
					assert.Equal(t, tt.expectedResponse, got)
				}
			}
		})
	}
}

func Test_restGetCompleteProfile(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	profile := buildTestDeviceProfileRequest()

	tests := []struct {
		name             string
		profileName      string
		expectStatusCode int
		expectedResponse dto.ProfileObject
		returnError      error
	}{
		{"GetCompleteProfile - Passed", profile.Profile.Name, http.StatusOK, profile, nil},
		{"GetCompleteProfile - Failed", "Test", http.StatusBadRequest, dto.ProfileObject{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)
			mockMetaService.On("GetCompleteProfile", tt.profileName).Return(tt.expectedResponse, tt.returnError)

			if err := restGetCompleteProfile(c, mockRouter); err != nil {
				assert.Equal(t, tt.expectStatusCode, err.Code)
				if tt.expectStatusCode == http.StatusOK {
					var got dto.ProfileObject
					json.Unmarshal(rec.Body.Bytes(), &got)
					assert.Equal(t, tt.expectedResponse, got)
				}
			}
		})
	}
}

func Test_restGetDeviceExt(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	deviceExt := []dto.DeviceExtResp{
		{Field: deviceName, Value: TestDeviceName},
	}
	tests := []struct {
		name             string
		deviceName       string
		expectStatusCode int
		expectedResponse []dto.DeviceExtResp
		returnError      error
	}{
		{"GetDeviceExt - Passed", deviceName, http.StatusOK, deviceExt, nil},
		{"GetDeviceExt - Failed", "Test", http.StatusNotFound, nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)
			mockMetaService.On("GetDeviceExtension", tt.deviceName).Return(tt.expectedResponse, tt.returnError)

			if err := restGetDeviceExt(c, mockRouter); err != nil {
				assert.Equal(t, tt.expectStatusCode, err.Code)
				if tt.expectStatusCode == http.StatusOK {
					var got []dto.DeviceExtResp
					json.Unmarshal(rec.Body.Bytes(), &got)
					assert.Equal(t, tt.expectedResponse, got)
				}
			}
		})
	}
}

func Test_restGetDvcProfileExt(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	profDeviceExtArr := []dto.DeviceExtension{
		{
			Field:       "device",
			Default:     "test",
			IsMandatory: false,
		},
	}
	tests := []struct {
		name             string
		profileName      string
		expectStatusCode int
		expectedResponse []dto.DeviceExtension
		returnError      error
	}{
		{"GetDvcProfileExt - Passed", profileName, http.StatusOK, profDeviceExtArr, nil},
		{"GetDvcProfileExt - Failed", "Test", http.StatusNotFound, nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)
			mockMetaService.On("GetDeviceExtensionInProfile", tt.profileName).Return(tt.expectedResponse, tt.returnError)

			if err := restGetDvcProfileExt(c, mockRouter); err != nil {
				assert.Equal(t, tt.expectStatusCode, err.Code)
				if tt.expectStatusCode == http.StatusOK {
					var got []dto.DeviceExtension
					json.Unmarshal(rec.Body.Bytes(), &got)
					assert.Equal(t, tt.expectedResponse, got)
				}
			}
		})
	}
}

func Test_restUpdateAssociations(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	associatedNodes := []dto.AssociationNode{{NodeType: deviceName, NodeName: TestDeviceName}}
	jsonByte, _ := json.Marshal(associatedNodes)

	tests := []struct {
		name             string
		nodeAName        string
		nodeAType        string
		requestBody      io.Reader
		expectStatusCode int
		mockError        error
	}{
		{"UpdateAssociations - Passed", "TestName", deviceName, bytes.NewReader(jsonByte), http.StatusOK, nil},
		{"UpdateAssociations - Failed", "Device", deviceName, bytes.NewReader(jsonByte), http.StatusBadRequest, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "error")},
		{"UpdateAssociations - Passed2", "Other", deviceName, nil, http.StatusConflict, nil},
		{"UpdateAssociations - Failed2", "Other", "test", bytes.NewReader(jsonByte), http.StatusNotImplemented, nil},
		{"UpdateAssociations - Failed3", "Other1", deviceName, io.NopCloser(ErrorReader{}), http.StatusBadRequest, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s", tt.nodeAType, tt.nodeAName), tt.requestBody)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetPath("/:nodeAType/:nodeAName")
			c.SetParamNames("nodeAType", "nodeAName")
			c.SetParamValues(tt.nodeAType, tt.nodeAName)

			if tt.mockError == nil {
				mockMetaService.On("UpdateAssociation", tt.nodeAName, tt.nodeAType, mock.AnythingOfType("[]dto.AssociationNode"), false).Return("updatedAssociationID", nil)
				mockMetaService.On("DeleteAssociation", tt.nodeAName, tt.nodeAType, true).Return(nil)
			} else {
				mockMetaService.On("UpdateAssociation", tt.nodeAName, tt.nodeAType, mock.AnythingOfType("[]dto.AssociationNode"), false).Return("", tt.mockError)
				mockMetaService.On("DeleteAssociation", tt.nodeAName, tt.nodeAType, true).Return(tt.mockError)
			}

			if err := restUpdateAssociations(c, mockRouter); err != nil {
				assert.Equal(t, tt.expectStatusCode, err.Code)
			}
		})
	}
}

func Test_restUpdateCompleteDevice(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	device := buildTestDeviceRequest()
	jsonByte, _ := json.Marshal(device)

	tests := []struct {
		name             string
		deviceName       string
		requestBody      io.Reader
		expectStatusCode int
		mockError        error
	}{
		{"UpdateDevice - Passed", device.Device.Name, bytes.NewReader(jsonByte), http.StatusOK, nil},
		{"UpdateDevice - Failed", "test", bytes.NewReader(jsonByte), http.StatusBadRequest, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "error")},
		{"UpdateDevice - Failed1", "test", io.NopCloser(ErrorReader{}), http.StatusBadRequest, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/devices/%s", tt.deviceName), tt.requestBody)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetPath("/devices/:device")
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)

			if tt.mockError == nil {
				mockMetaService.On("UpdateCompleteDevice", tt.deviceName, mock.AnythingOfType("dto.DeviceObject")).Return("updatedDeviceID", nil)
			} else {
				mockMetaService.On("UpdateCompleteDevice", tt.deviceName, mock.AnythingOfType("dto.DeviceObject")).Return("", tt.mockError)
			}

			if err := restUpdateCompleteDevice(c, mockRouter); err != nil {
				assert.Equal(t, tt.expectStatusCode, err.Code)
			}
		})
	}
}

func Test_restUpdateCompleteProfile(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	profile := buildTestDeviceProfileRequest()
	profile1 := buildTestDeviceProfileRequest()
	profile1.Profile.DeviceResources[0].Name = "1234567890123456789012345678901234567890"
	jsonByte, _ := json.Marshal(profile)
	jsonByte1, _ := json.Marshal(profile1)

	tests := []struct {
		name             string
		profileName      string
		requestBody      io.Reader
		expectStatusCode int
		mockErrorUpdate  hedgeErrors.HedgeError
		mockErrorGet     hedgeErrors.HedgeError
		mockDeviceList   []string
	}{
		{"UpdateProfile - Passed", "Random-Boolean-Device", bytes.NewReader(jsonByte), http.StatusOK, nil, nil, []string{"testDevice1", "testDevice2"}},
		{"UpdateProfile - FailedWithUpdateError", "Test", bytes.NewReader(jsonByte), http.StatusBadRequest, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "update error"), nil, []string{"testDevice1", "testDevice2"}},
		{"UpdateProfile - FailedWithGetError", "Random-Boolean-Device", bytes.NewReader(jsonByte), http.StatusBadRequest, nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "get error"), []string{"testDevice1", "testDevice2"}},
		{"UpdateProfile - FailedWithGetError", "Random-Boolean-Device", io.NopCloser(ErrorReader{}), http.StatusBadRequest, nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "custom error"), []string{"testDevice1", "testDevice2"}},
		{"UpdateProfile - FailedDeviceResourceNameExceededLimit", profile.Profile.Name, bytes.NewReader(jsonByte1), http.StatusBadRequest, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Device resource name should not exceed 35 characters in length, failed on name: 1234567890123456789012345678901234567890"), nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/profiles/:profile", tt.requestBody)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			dsi.On("GetDevicesByProfile", tt.profileName).Return(tt.mockDeviceList, nil)
			if tt.mockErrorGet == nil {
				mockMetaService.On("GetCompleteProfile", tt.profileName).Return(profile, nil)
			} else {
				mockMetaService.On("GetCompleteProfile", tt.profileName).Return(nil, tt.mockErrorGet)
			}
			if tt.mockErrorUpdate == nil && tt.mockErrorGet == nil {
				mockMetaService.On("UpdateCompleteProfile", tt.profileName, mock.AnythingOfType("dto.ProfileObject")).Return("updatedProfileID", nil)
			} else {
				mockMetaService.On("UpdateCompleteProfile", tt.profileName, mock.AnythingOfType("dto.ProfileObject")).Return("", tt.mockErrorUpdate)
			}

			if err := restUpdateCompleteProfile(c, mockRouter); err != nil {
				assert.Equal(t, tt.expectStatusCode, err.Code)
			}
		})
	}
}

func Test_restUpdateCompleteProfile_SucceededToDeleteDeviceExtensions_Passed(t *testing.T) {
	e := echo.New()

	mockMetaService := &mds.MockMetaDataService{}

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService

	profileBeforeDeletion := buildTestDeviceProfileRequestForDeleteExt()

	profileAfterDeletion := buildTestDeviceProfileRequestForDeleteExt()
	profileAfterDeletion.ProfileExtension = dto.ProfileExtension{
		DeviceAttributes: []dto.DeviceExtension{
			{
				Field:       "att1",
				Default:     "default_value",
				IsMandatory: false,
			},
		},
		ContextualAttributes: nil,
	}

	deviceList := []string{"TestDevice"}
	jsonByte, _ := json.Marshal(profileAfterDeletion)

	requestBody := bytes.NewReader(jsonByte)
	missingAttribute := "att2"

	req := httptest.NewRequest(http.MethodPut, "/profiles/:profile", requestBody)
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	c.SetParamNames("profile")
	c.SetParamValues(profileAfterDeletion.Profile.Name)

	dsi.On("GetDevicesByProfile", TestDeviceProfileName).Return(deviceList, nil)
	mockMetaService.On("GetCompleteProfile", TestDeviceProfileName).Return(profileBeforeDeletion, nil)
	mockMetaService.On("UpdateCompleteProfile", TestDeviceProfileName, mock.AnythingOfType("dto.ProfileObject")).Return("updatedProfileID", nil)
	mockMetaService.On("DeleteDeviceExtension", TestDeviceName, []string{missingAttribute}).Return(nil)

	err1 := restUpdateCompleteProfile(c, mockRouter)

	assert.Empty(t, err1)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]string
	err2 := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err2)
	assert.Contains(t, response, "id")
	assert.Equal(t, "updatedProfileID", response["id"])
}

func Test_restUpdateCompleteProfile_FailedToDeleteDeviceExtensions_ReturnedError(t *testing.T) {
	e := echo.New()

	mockMetaService := &mds.MockMetaDataService{}

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService

	profileBeforeDeletion := buildTestDeviceProfileRequestForDeleteExt()

	profileAfterDeletion := buildTestDeviceProfileRequestForDeleteExt()
	profileAfterDeletion.ProfileExtension = dto.ProfileExtension{
		DeviceAttributes: []dto.DeviceExtension{
			{
				Field:       "att1",
				Default:     "default_value",
				IsMandatory: false,
			},
		},
		ContextualAttributes: nil,
	}

	deviceList := []string{"TestDevice"}
	jsonByte, _ := json.Marshal(profileAfterDeletion)

	requestBody := bytes.NewReader(jsonByte)
	missingAttribute := "att2"

	req := httptest.NewRequest(http.MethodPut, "/profiles/:profile", requestBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	c.SetParamNames("profile")
	c.SetParamValues(profileAfterDeletion.Profile.Name)

	dsi.On("GetDevicesByProfile", TestDeviceProfileName).Return(deviceList, nil)
	mockMetaService.On("GetCompleteProfile", TestDeviceProfileName).Return(profileBeforeDeletion, nil)
	mockMetaService.On("UpdateCompleteProfile", TestDeviceProfileName, mock.AnythingOfType("dto.ProfileObject")).Return("updatedProfileID", nil)
	mockMetaService.On("DeleteDeviceExtension", TestDeviceName, []string{missingAttribute}).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "delete error"))

	err := restUpdateCompleteProfile(c, mockRouter)

	assert.Error(t, err)
	assert.Equal(t, http.StatusInternalServerError, err.Code)
	assert.Equal(t, "delete error", err.Message)
}

func Test_restUpdateDeviceExt(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	deviceExts := []dto.DeviceExt{{Field: deviceName, Value: TestDeviceName}}
	jsonByte, _ := json.Marshal(deviceExts)
	mockMetaService.On("UpdateDeviceExtension", deviceName, deviceExts, false).Return("lkj-87h-sd7", nil)

	tests := []struct {
		name       string
		deviceName string
		body       io.Reader
		wantStatus int
		wantError  bool
	}{
		{"UpdateDeviceExtensions - Passed", deviceName, bytes.NewReader(jsonByte), http.StatusOK, false},
		{"UpdateDeviceExtensions - Failed", deviceName, io.NopCloser(ErrorReader{}), http.StatusBadRequest, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/", tt.body)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetPath("/:device")
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)

			if err := restUpdateDeviceExt(c, mockRouter); err != nil {
				if !tt.wantError {
					t.Errorf("%s: unexpected error status: gotErr=%v, wantErr=%v", tt.name, err != nil, tt.wantError)
				}

				assert.Equal(t, tt.wantStatus, err.Code)
			}
		})
	}
}

func Test_restUpdateDvcProfileExt(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	profDeviceExtArray := []dto.DeviceExtension{{
		Field:       "device",
		Default:     "test",
		IsMandatory: false,
	}}
	jsonByte, _ := json.Marshal(profDeviceExtArray)
	mockMetaService.On("UpdateDeviceExtensionsInProfile", profileName, profDeviceExtArray, true).Return("lkj-87h-sd7", nil)
	mockMetaService.On("GetProfileExtensionInProfile", profileName).Return(profDeviceExtArray, nil)

	tests := []struct {
		name        string
		profileName string
		body        io.Reader
		wantStatus  int
		wantError   bool
	}{
		{"UpdateProfiles - Passed", profileName, bytes.NewReader(jsonByte), http.StatusOK, false},
		{"UpdateProfiles - Failed", profileName, io.NopCloser(ErrorReader{}), http.StatusBadRequest, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/", tt.body)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetPath("/:profile")
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			if err := restUpdateDvcProfileExt(c, mockRouter); err != nil {
				if !tt.wantError {
					t.Errorf("%s: unexpected error status: gotErr=%v, wantErr=%v", tt.name, err != nil, tt.wantError)
				}

				assert.Equal(t, tt.wantStatus, err.Code)
			}
		})
	}
}

func Test_upsertDeviceBusinessData(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	contextualData := map[string]interface{}{"Field1": deviceName, "Field2": TestDeviceName}
	mockMetaService.On("UpsertDeviceContextualAttributes", deviceName, contextualData).Return(nil)
	mockMetaService.On("GetCompleteDevice", deviceName, mock.Anything).Return(dto.DeviceObject{}, nil)
	mockMetaService.On("NotifyMetaSync", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	tests := []struct {
		name       string
		deviceName string
		bizData    map[string]interface{}
		wantStatus int
	}{
		{"upsertDeviceContextualData - Passed", deviceName, contextualData, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonByte, _ := json.Marshal(tt.bizData)
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(jsonByte)))
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("device")
			c.SetParamValues(tt.deviceName)

			err := upsertDeviceContextualData(c, mockRouter, jsonByte)
			if err != nil {
				if err.Code != tt.wantStatus {
					t.Errorf("upsertDeviceContextualData() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_upsertProfileBusinessAttributes(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	contextualAttr := []string{"attrb1", "attrb2", "attrb3"}
	mockMetaService.On("UpsertProfileContextualAttributes", profileName, contextualAttr).Return(nil)

	tests := []struct {
		name       string
		profile    string
		attributes []string
		wantStatus int
	}{
		{"UpsertProfileContextualAttributes - Passed", profileName, contextualAttr, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonByte, _ := json.Marshal(tt.attributes)
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(jsonByte)))
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profile)

			if err := upsertProfileContextualAttributes(c, mockRouter); err != nil {
				t.Errorf("upsertProfileContextualAttributes() error = %v", err)

				if err.Code != tt.wantStatus {
					t.Errorf("upsertProfileContextualAttributes() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
		})
	}
}

func Test_getMetadataServiceUrl(t *testing.T) {
	type args struct {
		service interfaces.ApplicationService
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"getMetadataServiceUrl - Passed", args{service: router.service}, router.metaDataServiceURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getMetadataServiceUrl(tt.args.service); got != tt.want {
				t.Errorf("getMetadataServiceUrl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getMetasyncServiceUrl(t *testing.T) {
	type args struct {
		service interfaces.ApplicationService
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"getMetasyncServiceUrl - Passed", args{service: router.service}, "MockMetaSyncUrl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getMetasyncServiceUrl(tt.args.service); got != tt.want {
				t.Errorf("getMetasyncServiceUrl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getHedgeAdminServiceUrl(t *testing.T) {
	type args struct {
		service interfaces.ApplicationService
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"getHedgeAdminServiceUrl - Passed", args{service: router.service}, "http://hedge-admin:4321"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHedgeAdminServiceUrl(tt.args.service); got != tt.want {
				t.Errorf("getHedgeAdminServiceUrl() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTelemetry_ContextualDataRequest(t *testing.T) {
	contextualDataRequestsCounter := metrics.NewCounter()
	contextualDataSizeCounter := metrics.NewCounter()
	mockedTelemetry := &Telemetry{
		contextualDataRequests: contextualDataRequestsCounter,
		contextualDataSize:     contextualDataSizeCounter,
		redisClient:            &dbc,
	}

	existingData := map[string]interface{}{
		"key1": "the same value",
		"key2": "another value",
	}
	existingData1 := map[string]interface{}{}
	existingData2 := map[string]interface{}{
		"key1": "the same value",
	}

	tests := []struct {
		name               string
		reqBody            map[string]interface{}
		existingData       map[string]interface{}
		expectedSizeChange int64
		expectedErr        error
	}{
		{
			name: "ContextualData - Passed with changes",
			reqBody: map[string]interface{}{
				"key1": "the same value",
				"key2": "new value for existing key",
				"key3": "new key",
			},
			existingData: existingData,
			expectedSizeChange: calculateSize(map[string]interface{}{"key2": "new value for existing key"}) +
				calculateSize(map[string]interface{}{"key3": "new key"}),
			expectedErr: nil,
		},
		{
			name: "ContextualData - New data only",
			reqBody: map[string]interface{}{
				"key1": "some value",
				"key2": "more data",
			},
			existingData:       existingData1,
			expectedSizeChange: calculateSize(map[string]interface{}{"key1": "some value", "key2": "more data"}),
			expectedErr:        nil,
		},
		{
			name: "ContextualData - No changes",
			reqBody: map[string]interface{}{
				"key1": "the same value", // same as existing
			},
			existingData:       existingData2,
			expectedSizeChange: 0,
			expectedErr:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset counters
			contextualDataRequestsCounter.Clear()
			contextualDataSizeCounter.Clear()

			reqBody, _ := json.Marshal(tt.reqBody)
			err := mockedTelemetry.ContextualDataRequest(tt.existingData, reqBody)

			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedSizeChange, contextualDataSizeCounter.Count())
			assert.Equal(t, int64(1), contextualDataRequestsCounter.Count())
		})
	}
}

func calculateSize(value interface{}) int64 {
	data, _ := json.Marshal(value)
	return int64(len(data))
}

func Test_getSQLMetaData(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("GetSQLMetaData", "some query here").Return(dto.SQLMetaData{}, nil)
	mockMetaService.On("GetSQLMetaData", "").Return(dto.SQLMetaData{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, "error"))

	tests := []struct {
		name     string
		sqlQuery string
		wantErr  bool
	}{
		{"getSqlMetaData - Passed", "some query here", false},
		{"getSqlMetaData - Failed", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"sql": "`+tt.sqlQuery+`"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := getSQLMetaData(c, mockRouter); (err != nil) != tt.wantErr {
				t.Errorf("getSQLMetaData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetRequestBody(t *testing.T) {
	tests := []struct {
		name        string
		requestBody string
		expectError bool
	}{
		{"GetRequestBody - Passed", "valid body", false},
		{"GetRequestBody - Failed", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", bytes.NewBufferString(tt.requestBody))
			if tt.expectError {
				req.Body = io.NopCloser(&ErrorReader{})
			}
			writer := httptest.NewRecorder()
			body, err := getRequestBody(writer, req, *router)

			if tt.expectError {
				assert.Nil(t, body)
				assert.Error(t, err)
			} else {
				assert.Equal(t, tt.requestBody, string(body))
				if err != nil {
					t.Errorf("Unexpected error: %s", err.Error())
				}
			}
		})
	}
}

type ErrorReader struct{}

func (ErrorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("custom error")
}

func Test_validateNodeRawDataConfigs(t *testing.T) {
	type args struct {
		nodeRawDataConfigs *[]dto.NodeRawDataConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{"valid configs",
			args{
				nodeRawDataConfigs: &[]dto.NodeRawDataConfig{
					{
						Node:        dto.Node{NodeID: "node1", Host: "host1"},
						SendRawData: true,
						StartTime:   time.Now().Unix(),
						EndTime:     time.Now().Unix() + 86400,
					},
				},
			}, nil,
		},
		{"missing NodeID",
			args{
				nodeRawDataConfigs: &[]dto.NodeRawDataConfig{
					{
						Node:        dto.Node{Host: "host1"},
						SendRawData: true,
					},
				},
			}, echo.NewHTTPError(http.StatusBadRequest, "Node value is not provided"),
		},
		{"EndTime before StartTime",
			args{
				nodeRawDataConfigs: &[]dto.NodeRawDataConfig{
					{
						Node:        dto.Node{NodeID: "node1", Host: "host1"},
						SendRawData: true,
						EndTime:     time.Now().Unix() - 1000,
					},
				},
			}, echo.NewHTTPError(http.StatusBadRequest, "Config 0 has EndTime before StartTime"),
		},
		{"EndTime is 0 && SendRawData",
			args{
				nodeRawDataConfigs: &[]dto.NodeRawDataConfig{
					{
						Node:        dto.Node{NodeID: "node1", Host: "host1"},
						SendRawData: true,
						StartTime:   time.Now().Unix(),
						EndTime:     0,
					},
				},
			}, nil,
		},
		{"SendRawData is false",
			args{
				nodeRawDataConfigs: &[]dto.NodeRawDataConfig{
					{
						Node:        dto.Node{NodeID: "node1", Host: "host1"},
						SendRawData: false,
					},
				},
			}, nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNodeRawDataConfigs(tt.args.nodeRawDataConfigs)

			// If an error is expected
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("validateNodeRawDataConfigs() expected error, got nil")
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("validateNodeRawDataConfigs() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				// No error is expected, but got one
				t.Errorf("validateNodeRawDataConfigs() unexpected error = %v", err)
			}
		})
	}
}

func Test_upsertDownsamplingConfig(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	mockMetaService.On("UpsertDownsamplingConfig", "validProfile", mock.Anything).Return(nil)
	mockMetaService.On("UpsertDownsamplingConfig", "validProfile1", mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "DefaultDownsamplingIntervalSecs or/and DefaultDataCollectionIntervalSecs value is not provided"))
	mockMetaService.On("UpsertDownsamplingConfig", "validProfile2", mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "mocked error from db"))

	tests := []struct {
		name         string
		profileName  string
		requestBody  string
		wantStatus   int
		wantErrorMsg string
	}{
		{"Successful upsert", "validProfile", `{"DefaultDownsamplingIntervalSecs": 10, "DefaultDataCollectionIntervalSecs": 20}`, http.StatusOK, ""},
		{"Invalid JSON", "validProfile1", `{"DefaultDownsamplingIntervalSecs": 0, "DefaultDataCollectionIntervalSecs": 0}`, http.StatusBadRequest, "DefaultDownsamplingIntervalSecs/DefaultDataCollectionIntervalSecs value not set"},
		{"Db error", "validProfile2", `{"DefaultDownsamplingIntervalSecs": 10, "DefaultDataCollectionIntervalSecs": 20}`, http.StatusBadRequest, "mocked error from db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.requestBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetPath("/:profile")
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			err := upsertDownsamplingConfig(c, mockRouter)
			if tt.wantErrorMsg != "" {
				if assert.Error(t, err) {
					assert.Contains(t, err.Error(), tt.wantErrorMsg, "Error message should contain the expected text")
				}
			} else {
				if err != nil {
					t.Errorf("upsertDownsamplingConfig() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
				}
			}
			if err != nil && err.Code != tt.wantStatus {
				t.Errorf("upsertDownsamplingConfig() status code = %v, wantStatus %v", err.Code, tt.wantStatus)
			}
		})
	}
}

func Test_getDownsamplingConfig(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	downsamplingConfig := dto.DownsamplingConfig{
		DefaultDataCollectionIntervalSecs: 10,
		DefaultDownsamplingIntervalSecs:   20,
	}

	tests := []struct {
		name             string
		profileName      string
		expectStatusCode int
		expectedResponse *dto.DownsamplingConfig
		returnError      error
	}{
		{
			name:             "GetDownsamplingConfig - Passed",
			profileName:      profileName,
			expectStatusCode: http.StatusOK,
			expectedResponse: &downsamplingConfig,
			returnError:      nil,
		},
		{
			name:             "GetDownsamplingConfig - Failed",
			profileName:      "NonExistentProfile",
			expectStatusCode: http.StatusNotFound,
			expectedResponse: nil,
			returnError:      hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			mockMetaService.On("GetDownsamplingConfig", tt.profileName).Return(tt.expectedResponse, tt.returnError)

			if err := getDownsamplingConfig(c, mockRouter); err != nil {
				assert.Equal(t, tt.expectStatusCode, err.Code)

				if tt.expectStatusCode == http.StatusOK {
					var got dto.DownsamplingConfig
					json.Unmarshal(rec.Body.Bytes(), &got)
					assert.Equal(t, *tt.expectedResponse, got)
				} else {
					assert.Contains(t, rec.Body.String(), tt.returnError.Error())
				}
			}
		})
	}
}

func Test_upsertAggregateDefinition(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService

	validConfig := dto.DownsamplingConfig{DefaultDataCollectionIntervalSecs: 0, DefaultDownsamplingIntervalSecs: 0, Aggregates: []dto.Aggregate(nil)}
	jsonByte, _ := json.Marshal(validConfig)
	mockMetaService.On("UpsertAggregateDefinition", "profileName-1", mock.Anything).Return(nil)
	mockMetaService.On("UpsertAggregateDefinition", "profileName-2", mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "meta service error"))

	tests := []struct {
		name             string
		profileName      string
		requestBody      io.Reader
		expectStatusCode int
		returnError      error
	}{
		{"UpsertAggregateDefinition - Success", "profileName-1", bytes.NewReader(jsonByte), http.StatusOK, nil},
		{"UpsertAggregateDefinition - MetaService Error", "profileName-2", bytes.NewReader(jsonByte), http.StatusBadRequest, errors.New("meta service error")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", tt.requestBody)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)
			c.SetParamNames("profile")
			c.SetParamValues(tt.profileName)

			err := upsertAggregateDefinition(c, mockRouter)
			if tt.returnError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.returnError.Error(), "Error message should contain the expected text")
				assert.Equal(t, tt.expectStatusCode, err.Code)
			} else {
				if err != nil {
					t.Errorf("upsertAggregateDefinition() status code = %v, wantStatus %v", err.Code, tt.returnError.Error())
				}
			}
		})
	}
}

func Test_UpsertNodeRawDataConfigs(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService

	validConfigs := []dto.NodeRawDataConfig{
		{
			SendRawData: true,
			StartTime:   17777777,
			EndTime:     188888888,
			Node: dto.Node{
				NodeID: "node-id",
				Host:   "node-host",
			},
		},
	}
	jsonByte, _ := json.Marshal(validConfigs)

	invalidConfigs := []dto.NodeRawDataConfig{
		{
			SendRawData: true,
			StartTime:   197777777,
			EndTime:     188888888,
			Node: dto.Node{
				NodeID: "node-id",
				Host:   "node-host",
			},
		},
	}
	jsonByte1, _ := json.Marshal(invalidConfigs)
	mockMetaService.On("UpsertNodeRawDataConfigs", mock.Anything).Return(nil)
	mockMetaService.On("UpsertNodeRawDataConfigs", mock.Anything).Return(hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "meta service error"))

	tests := []struct {
		name             string
		requestBody      io.Reader
		expectStatusCode int
		returnError      error
	}{
		{"UpsertNodeRawDataConfigs - Success", bytes.NewReader(jsonByte), http.StatusOK, nil},
		{"UpsertNodeRawDataConfigs - Validation Error", bytes.NewReader(jsonByte1), http.StatusBadRequest, echo.NewHTTPError(http.StatusBadRequest, "Config 0 has EndTime before StartTime")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", tt.requestBody)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			c := e.NewContext(req, rec)

			err := upsertNodeRawDataConfigs(c, mockRouter)
			if tt.returnError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.returnError.Error(), "Error message should contain the expected text")

				assert.Equal(t, tt.expectStatusCode, err.Code)
			}
		})
	}
}

func Test_GetNodeRawDataConfigs(t *testing.T) {
	e := echo.New()

	mockRouter := *router
	mockRouter.service = router.service
	mockRouter.metaService = mockMetaService
	nodeIDs := []string{"node1", "node2", "node3"}
	nodeIDs1 := []string{""}

	validNodeRawDataConfigs := map[string]*dto.NodeRawDataConfig{
		"node1": {
			SendRawData: true,
			StartTime:   17777777,
			EndTime:     188888888,
			Node: dto.Node{
				NodeID: "node1",
				Host:   "node-host1",
			},
		},
		"node2": {
			SendRawData: false,
			StartTime:   17777777,
			EndTime:     188888888,
			Node: dto.Node{
				NodeID: "node2",
				Host:   "node-host2",
			},
		},
		"node3": nil,
	}

	mockMetaService.On("GetNodeRawDataConfigs", nodeIDs1).Return(nil, errors.New("nodes parameter are required"))
	mockMetaService.On("GetNodeRawDataConfigs", nodeIDs).Return(validNodeRawDataConfigs, nil)

	tests := []struct {
		name             string
		expectStatusCode int
		expectedResponse map[string]*dto.NodeRawDataConfig
		returnError      error
	}{
		{"GetNodeRawDataConfigs - Success", http.StatusOK, validNodeRawDataConfigs, nil},
		{"GetNodeRawDataConfigs - QueryParam is empty Error", http.StatusBadRequest, nil, echo.NewHTTPError(http.StatusBadRequest, "nodes parameter are required")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err *echo.HTTPError
			rec := httptest.NewRecorder()

			if strings.Contains(tt.name, "Success") {
				req := httptest.NewRequest(http.MethodGet, "/v3/dummy?nodes=node1,node2,node3", nil)
				req.Header.Set("Content-Type", "application/json")
				c := e.NewContext(req, rec)
				err = getNodeRawDataConfigs(c, mockRouter)
			} else {
				req := httptest.NewRequest(http.MethodGet, "/v3/dummy?nodes=", nil)
				req.Header.Set("Content-Type", "application/json")
				c := e.NewContext(req, rec)
				err = getNodeRawDataConfigs(c, mockRouter)
			}

			if tt.returnError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectStatusCode, err.Code, "Expected and actual status codes should match")
			} else {
				if len(tt.expectedResponse) > 0 {
					var got map[string]*dto.NodeRawDataConfig
					errC := json.Unmarshal(rec.Body.Bytes(), &got)
					if errC != nil {
						t.Errorf("Should be able to unmarshal response")
					}

					assert.Equal(t, tt.expectedResponse, got, "Expected response should match the actual response")
				}
			}
		})
	}
}

func TestNewTelemetry(t *testing.T) {
	type args struct {
		service        interfaces.ApplicationService
		serviceName    string
		metricsManager inter.MetricsManager
		hostName       string
		redisClient    redis2.DeviceExtDBClientInterface
	}

	expectedTelemetry := &Telemetry{
		contextualDataRequests: metrics.NewCounter(),
		contextualDataSize:     metrics.NewCounter(),
		redisClient:            &dbc,
	}

	arg1 := args{
		service:        u.AppService,
		serviceName:    "Test Service",
		metricsManager: metricsManager.MetricsMgr,
		hostName:       "localhost",
		redisClient:    &dbc,
	}
	tests := []struct {
		name string
		args args
		want *Telemetry
	}{
		{"NewTelemetry - Passed", arg1, expectedTelemetry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTelemetry(tt.args.service, tt.args.serviceName, tt.args.metricsManager, tt.args.redisClient)

			if !reflect.DeepEqual(got.contextualDataRequests.Count(), tt.want.contextualDataRequests.Count()) {
				t.Errorf("NewTelemetry().contextualDataRequests.Count() = %v, want %v", got.contextualDataRequests.Count(), tt.want.contextualDataRequests.Count())
			}
			if !reflect.DeepEqual(got.contextualDataSize.Count(), tt.want.contextualDataSize.Count()) {
				t.Errorf("NewTelemetry().contextualDataSize.Count() = %v, want %v", got.contextualDataSize.Count(), tt.want.contextualDataSize.Count())
			}
		})
	}
}

func Test_restGetProfileTags(t *testing.T) {
	mockRouter := NewRouter(u.AppService, "TestService", metricsManager.MetricsMgr)
	mockRouter.metaDataServiceURL = "http://mock-meta-service"

	testResponseBody := `{
            "profiles": [
                {"name": "profile1", "description": "Description 1"}
            ]
        }`

	t.Run("restGetProfileTags - Passed", func(t *testing.T) {
		testResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(testResponseBody)),
		}

		mockedHttpClient := &mservice.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(testResponse, nil)
		util.HttpClient = mockedHttpClient

		mockMetaService := &mds.MockMetaDataService{}
		mockMetaService.On("GetCompleteProfile", "profile1").
			Return(dto.ProfileObject{
				ProfileExtension: dto.ProfileExtension{
					DeviceAttributes: []dto.DeviceExtension{
						{Field: "tag1"}, {Field: "tag2"},
					},
					ContextualAttributes: []string{"context1"},
				},
			}, nil)
		mockRouter.metaService = mockMetaService

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		t.Cleanup(func() {
			mockRouter.metaService = nil
			util.HttpClient = nil
		})

		err := restGetProfileTags(c, *mockRouter)
		require.Nil(t, err, "restGetProfileTags should not return an error")
		assert.Equal(t, http.StatusOK, rec.Code, "HTTP status code should be 200")

		expectedResponse := []interface{}{"tag1", "tag2", "context1"}
		var actualResponse []interface{}
		err1 := json.Unmarshal(rec.Body.Bytes(), &actualResponse)
		require.NoError(t, err1, "Response body should be valid JSON")
		assert.ElementsMatch(t, expectedResponse, actualResponse, "Response tags should match expected tags, ignoring order")
	})
	t.Run("restGetProfileTags - Failed (Error retrieving profile names)", func(t *testing.T) {
		mockedHttpClient := &mservice.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("dummy error"))
		util.HttpClient = mockedHttpClient

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		t.Cleanup(func() {
			mockRouter.metaService = nil
			util.HttpClient = nil
		})

		err := restGetProfileTags(c, *mockRouter)
		require.Error(t, err)
	})
	t.Run("restGetProfileTags - Failed (Error retrieving complete profile)", func(t *testing.T) {
		testResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(testResponseBody)),
		}

		mockedHttpClient := &mservice.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(testResponse, nil)
		util.HttpClient = mockedHttpClient

		mockMetaService := &mds.MockMetaDataService{}
		mockMetaService.On("GetCompleteProfile", "profile1").
			Return(dto.ProfileObject{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "mock error"))
		mockRouter.metaService = mockMetaService

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		t.Cleanup(func() {
			mockRouter.metaService = nil
			util.HttpClient = nil
		})

		err := restGetProfileTags(c, *mockRouter)
		require.Error(t, err)
	})
	t.Run("restGetProfileTags - Passed (No Tags)", func(t *testing.T) {
		testResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(testResponseBody)),
		}

		mockedHttpClient := &mservice.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(testResponse, nil)
		util.HttpClient = mockedHttpClient

		mockMetaService := &mds.MockMetaDataService{}
		mockMetaService.On("GetProfileNames", mockRouter.metaDataServiceURL, u.AppService.LoggingClient()).
			Return([]string{"profile1"}, nil)
		mockMetaService.On("GetCompleteProfile", "profile1").
			Return(dto.ProfileObject{
				ProfileExtension: dto.ProfileExtension{
					DeviceAttributes:     []dto.DeviceExtension{},
					ContextualAttributes: []string{},
				},
			}, nil)
		mockRouter.metaService = mockMetaService

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		t.Cleanup(func() {
			mockRouter.metaService = nil
			util.HttpClient = nil
		})

		err := restGetProfileTags(c, *mockRouter)
		require.Nil(t, err, "restGetProfileTags should not return an error")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `null`, rec.Body.String(), "Expected an empty JSON array")
	})
}

func Test_restGetProfilesByTags(t *testing.T) {
	mockRouter := NewRouter(u.AppService, "TestService", metricsManager.MetricsMgr)
	mockRouter.metaDataServiceURL = "http://mock-meta-service"

	t.Run("restGetProfilesByTags - Passed", func(t *testing.T) {
		mockRequestBody := `["tag1", "context1"]`
		testResponseBody := `{
            "profiles": [
                {"name": "profile1", "description": "Description 1"}
            ]
        }`

		testResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(testResponseBody)),
		}

		mockedHttpClient := &mservice.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(testResponse, nil)
		util.HttpClient = mockedHttpClient

		mockMetaService := &mds.MockMetaDataService{}
		mockMetaService.On("GetCompleteProfile", "profile1").
			Return(dto.ProfileObject{
				ApiVersion: "v1",
				ProfileExtension: dto.ProfileExtension{
					DeviceAttributes: []dto.DeviceExtension{
						{Field: "tag1"},
					},
					ContextualAttributes: []string{"context1"},
				},
			}, nil)
		mockMetaService.On("GetCompleteProfile", "profile2").
			Return(dto.ProfileObject{
				ApiVersion: "v1",
				ProfileExtension: dto.ProfileExtension{
					DeviceAttributes: []dto.DeviceExtension{
						{Field: "tag2"},
					},
					ContextualAttributes: []string{"context2"},
				},
			}, nil)
		mockRouter.metaService = mockMetaService

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(mockRequestBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		t.Cleanup(func() {
			mockRouter.metaService = nil
			util.HttpClient = nil
		})

		err := restGetProfilesByTags(c, *mockRouter)
		require.Nil(t, err, "restGetProfilesByTags should not return an error")
		assert.Equal(t, http.StatusOK, rec.Code, "HTTP status code should be 200")

		expectedResponse := []dto.ProfileObject{
			{
				ApiVersion: "v1",
				ProfileExtension: dto.ProfileExtension{
					DeviceAttributes: []dto.DeviceExtension{
						{Field: "tag1"},
					},
					ContextualAttributes: []string{"context1"},
				},
			},
		}
		var actualResponse []dto.ProfileObject
		err1 := json.Unmarshal(rec.Body.Bytes(), &actualResponse)
		require.NoError(t, err1, "Response body should be valid JSON")
		assert.Equal(t, expectedResponse, actualResponse, "Response profiles should match expected profiles")
	})
	t.Run("restGetProfilesByTags - Failed (Error retrieving profile names)", func(t *testing.T) {
		mockRequestBody := `["tag1"]`

		mockedHttpClient := &mservice.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(nil, errors.New("dummy error"))
		util.HttpClient = mockedHttpClient

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(mockRequestBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		t.Cleanup(func() {
			mockRouter.metaService = nil
			util.HttpClient = nil
		})

		err := restGetProfilesByTags(c, *mockRouter)
		require.Error(t, err, "restGetProfilesByTags should return an error")
	})
	t.Run("restGetProfilesByTags - Passed (No matching profiles)", func(t *testing.T) {
		mockRequestBody := `["unknownTag"]`
		testResponseBody := `{
            "profiles": []
        }`

		testResponse := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(testResponseBody)),
		}

		mockedHttpClient := &mservice.MockHTTPClient{}
		mockedHttpClient.On("Do", mock.Anything).Return(testResponse, nil)
		util.HttpClient = mockedHttpClient

		mockMetaService := &mds.MockMetaDataService{}
		mockMetaService.On("GetCompleteProfile", "profile1").
			Return(dto.ProfileObject{
				ApiVersion: "v1",
				ProfileExtension: dto.ProfileExtension{
					DeviceAttributes:     []dto.DeviceExtension{},
					ContextualAttributes: []string{},
				},
			}, nil)
		mockRouter.metaService = mockMetaService

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(mockRequestBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e := echo.New()
		c := e.NewContext(req, rec)

		t.Cleanup(func() {
			mockRouter.metaService = nil
			util.HttpClient = nil
		})

		err := restGetProfilesByTags(c, *mockRouter)
		require.Nil(t, err, "restGetProfilesByTags should not return an error")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `null`, rec.Body.String(), "Expected an empty JSON array")
	})
}
