/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"encoding/gob"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	cl "hedge/common/client"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	comModels "hedge/common/models"
	"hedge/common/telemetry"
	"hedge/mocks/hedge/app-services/data-enrichment/cache"
	"hedge/mocks/hedge/common/client"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"hedge/mocks/hedge/common/service"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	inter "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	commonDTO "github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/mock"
)

type args struct {
	ctx  interfaces.AppFunctionContext
	data interface{} // issue here since we need different arguments for Enrich step and the other steps
}
type fields struct {
	apiServer             string
	nodeName              string
	eventsPublisherURL    string
	mqttSender            *transforms.MQTTSecretSender
	redisClient           cl.DBClientInterface
	deviceToDeviceInfoMap map[string]comModels.DeviceInfo
	telemetry             *Telemetry
	persistOnError        bool
}

const DOWNSAMPLINGINTERVALSECS = 50

var (
	mockService         *mocks.ApplicationService
	mockLogger          *logger.MockLogger
	u                   *utils.HedgeMockUtils
	ctx                 interfaces.AppFunctionContext
	httpClient          *utils.MockClient
	deviceServ          *service.MockDeviceServiceInter
	metricEvent         dtos.Event
	arg                 args
	flds                fields
	tel                 Telemetry
	dev                 map[string]interface{}
	devices             []interface{}
	devserv             map[string]interface{}
	mockedRedisDbClient *client.MockDBClientInterface
	mqttSender          *transforms.MQTTSecretSender
	metricsManager      *telemetry.MetricsManager
	mockCache           *cache.MockManager
)

func Init() {
	mockLogger = new(logger.MockLogger)
	mockService = new(mocks.ApplicationService)
	mockService.On("LoggingClient").Return(mockLogger)

	u = utils.NewApplicationServiceMock(
		map[string]string{"Topic": "SomeTopic", "MetricReportInterval": "10"},
	)
	u.InitMQTTSettings()
	ctx = pkg.NewAppFuncContextForTest("Test", u.AppService.LoggingClient())

	dev = map[string]interface{}{
		"id":             "55b68fcf-0fd2-445a-9fae-670b37fb9274",
		"name":           "Random-Boolean-Device",
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
	}
	devices = append(devices, dev)
	devserv = map[string]interface{}{}
	devserv = map[string]interface{}{
		"apiVersion": "v2",
		"requestId":  "479439fa-0148-11eb-adc1-0242ac120002",
		"statusCode": 200,
		"totalCount": 3,
		"devices":    "",
	}
	devserv["devices"] = devices

	tel = Telemetry{
		inMessages:   metrics.NewCounter(),
		inRawMetrics: metrics.NewCounter(),
	}
	metricsManager, _ = telemetry.NewMetricsManager(u.AppService, "HedgeDataEnrichmentServiceName")

	mockedRedisDbClient = &client.MockDBClientInterface{}
	cl.DBClientImpl = mockedRedisDbClient
	mockedRedisDbClient.On("GetDbClient", mock.Anything).Return(cl.DBClientImpl)
	mockedRedisDbClient.On("PublishToRedisBus", mock.Anything, mock.Anything).Return(nil)
	mockedRedisDbClient.On("GetMetricCounter", mock.Anything).Return(int64(10), nil)
	mockedRedisDbClient.On("SetMetricCounter", mock.Anything, mock.Anything).Return(nil)

	mqttConfig := transforms.MQTTSecretConfig{
		BrokerAddress: "",
		ClientId:      "test_dataEnrich",
		Topic:         "BMCMetrics",
		QoS:           0,
	}
	mqttSender = transforms.NewMQTTSecretSender(mqttConfig, false)

	deviceServ = &service.MockDeviceServiceInter{}
	deviceServ.On("GetDeviceInfoService", mock.Anything).Return(deviceServ)
	deviceServ.On("GetDeviceToDeviceInfoMap").Return(mock.Anything)

	httpClient = utils.NewMockClient()
	cl.Client = httpClient

	metricEvent = dtos.Event{
		Versionable: commonDTO.Versionable{ApiVersion: "V3"},
		Id:          "f1c5f0e8-6b64-48ad-91b7-c5981b5ca3b9",
		DeviceName:  "HVAC-A",
		ProfileName: "HVAC",
		SourceName:  "Humidity",
		Origin:      1695327555034742000,
		Readings: []dtos.BaseReading{
			{Id: "db8d3d6e-a6ca-4206-97f0-aed525767c0f",
				Origin:        1695327555034742000,
				DeviceName:    "HVAC-A",
				ResourceName:  "Humidity",
				ProfileName:   "HVAC",
				ValueType:     "Float64",
				Units:         "",
				BinaryReading: dtos.BinaryReading{BinaryValue: nil, MediaType: ""},
				SimpleReading: dtos.SimpleReading{Value: "2.097398e+00"},
				ObjectReading: dtos.ObjectReading{ObjectValue: nil}},
		},
		Tags: map[string]any{
			"devicelabels": "Telco-Tower-A,HVAC,Temperature,Pressure,Humidity",
			"host":         "",
		},
	}

	arg = args{
		ctx:  ctx,
		data: metricEvent,
	}

	flds = fields{
		apiServer:             "",
		nodeName:              "",
		eventsPublisherURL:    "",
		mqttSender:            mqttSender,
		redisClient:           mockedRedisDbClient,
		deviceToDeviceInfoMap: nil,
		telemetry:             &tel,
		persistOnError:        false,
	}

	mockCache = new(cache.MockManager)
}

func TestPubSub_EnrichData(t *testing.T) {
	Init()

	deviceTags := map[string]interface{}{
		"devicelabels": "Telco-Tower-A, HVAC, Temperature, Pressure, Humidity",
	}
	profileTags := map[string]interface{}{"model": "", "manufacturer": ""}

	mockCache.On("FetchDeviceTags", mock.AnythingOfType("string"), mock.Anything).
		Return(deviceTags, true)
	mockCache.On("FetchProfileTags", mock.AnythingOfType("string"), mock.Anything).
		Return(profileTags, true)

	arg1 := args{
		ctx:  ctx,
		data: nil,
	}
	arg2 := args{
		ctx:  ctx,
		data: []byte("wrong data"),
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
		want1  interface{}
	}{
		{"EnrichData - Passed", flds, arg, true, metricEvent},
		{
			"EnrichData - Failed1",
			flds,
			arg1,
			false,
			errors.New("function EnrichData in pipeline '': No Data Received"),
		},
		{
			"EnrichData - Failed2",
			flds,
			arg2,
			false,
			errors.New("function EnrichData in pipeline '': type received is not an Event"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DataEnricher{
				nodeName:              tt.fields.nodeName,
				EventsPublisherURL:    tt.fields.eventsPublisherURL,
				redisClient:           tt.fields.redisClient,
				deviceToDeviceInfoMap: tt.fields.deviceToDeviceInfoMap,
				telemetry:             tt.fields.telemetry,
				cache:                 mockCache,
				//PersistOnError:        tt.fields.persistOnError,
			}
			got, got1 := s.EnrichData(tt.args.ctx, tt.args.data)
			if got != tt.want {
				t.Errorf("EnrichData() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("EnrichData() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestPubSub_PublishEnrichedDataInternally(t *testing.T) {
	Init()

	arg1 := args{ctx, nil}
	redisDb1 := client.MockDBClientInterface{}
	redisDb1.On("PublishToRedisBus", mock.Anything, mock.Anything).
		Return(errors.New("mocked error"))

	flds1 := fields{
		redisClient:           &redisDb1,
		deviceToDeviceInfoMap: nil,
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
		want1  interface{}
	}{
		{"PublishEnrichedDataInternally - Passed", flds, arg, true, metricEvent},
		{
			"PublishEnrichedDataInternally - Failed1",
			flds,
			arg1,
			false,
			errors.New("function PublishEnrichedDataInternally in pipeline '': No Data Received"),
		},
		{"PublishEnrichedDataInternally - Failed2", flds1, arg, false, errors.New("mocked error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DataEnricher{
				nodeName:              tt.fields.nodeName,
				EventsPublisherURL:    tt.fields.eventsPublisherURL,
				redisClient:           tt.fields.redisClient,
				deviceToDeviceInfoMap: tt.fields.deviceToDeviceInfoMap,
				telemetry:             tt.fields.telemetry,
				cache:                 mockCache,
				sem:                   make(chan bool, 1),
				//PersistOnError:        tt.fields.persistOnError,
			}
			got, got1 := s.PublishEnrichedDataInternally(tt.args.ctx, tt.args.data)
			if got != tt.want {
				t.Errorf("PublishEnrichedDataInternally() got = %v, want = %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("PublishEnrichedDataInternally() got1 = %+v, want = %+v", got1, tt.want1)
			}
		})
	}
}

func TestPubSub_TransformToAggregatesAndEvents(t *testing.T) {
	Init()

	downSamplingCfg := dto.DownsamplingConfig{
		DefaultDataCollectionIntervalSecs: 2,
		DefaultDownsamplingIntervalSecs:   DOWNSAMPLINGINTERVALSECS,
		Aggregates: []dto.Aggregate{
			{
				FunctionName:         "avg",
				GroupBy:              "deviceName",
				SamplingIntervalSecs: DOWNSAMPLINGINTERVALSECS,
			},
			{
				FunctionName:         "max",
				GroupBy:              "deviceName",
				SamplingIntervalSecs: DOWNSAMPLINGINTERVALSECS,
			},
			{
				FunctionName:         "max",
				GroupBy:              "deviceName",
				SamplingIntervalSecs: DOWNSAMPLINGINTERVALSECS,
			},
		},
	}

	nodeRawDataConfig := dto.NodeRawDataConfig{
		SendRawData: true,
		StartTime:   0,
		EndTime:     time.Now().Unix() + 60*60,
		Node: dto.Node{
			NodeID: "node10",
			Host:   "node1.bmc.com",
		},
	}

	nodeDisableRawDataConfig := dto.NodeRawDataConfig{
		SendRawData: false,
		Node: dto.Node{
			NodeID: "node10",
			Host:   "node1.bmc.com",
		},
	}

	reading1, _ := dtos.NewSimpleReading(
		"HVAC",
		"HVAC-A",
		"Humidity",
		common.ValueTypeInt64,
		int64(24),
	)
	reading2, _ := dtos.NewSimpleReading("HVAC", "HVAC-A", "IsOn", common.ValueTypeBool, true)
	reading3, _ := dtos.NewSimpleReading(
		"WindTurbine",
		"AltaNS12",
		"isWorking",
		common.ValueTypeBool,
		false,
	)
	reading4, _ := dtos.NewSimpleReading(
		"HVAC",
		"HVAC-A",
		"Humidity",
		common.ValueTypeInt64,
		int64(30),
	)
	reading5, _ := dtos.NewSimpleReading(
		"HVAC",
		"HVAC-A",
		"rpm",
		common.ValueTypeFloat64,
		float64(230.45),
	)

	metricDataNoSampling := dtos.Event{
		Versionable: commonDTO.Versionable{ApiVersion: "V3"},
		Id:          "f1c5f0e8-6b64-48ad-91b7-c5981b5ca3b9",
		DeviceName:  "AltaNS12",
		ProfileName: "WindTurbine",
		Origin:      100,
		Readings: []dtos.BaseReading{
			reading3,
		},
		Tags: map[string]interface{}{
			"devicelabels": "Telco-Tower-A,HVAC,Temperature,Pressure,Humidity",
			"host":         "clm-pun-ddd",
		},
	}

	data1 := dtos.Event{
		Versionable: commonDTO.Versionable{ApiVersion: "V3"},
		Id:          "f1c5f0e8-6b64-48ad-91b7-c5981b5ca3b9",
		DeviceName:  "HVAC-A",
		ProfileName: "HVAC",
		Origin:      100,
		Readings: []dtos.BaseReading{
			reading1,
		},
		Tags: map[string]interface{}{
			"devicelabels": "Telco-Tower-A,HVAC,Temperature,Pressure,Humidity",
			"host":         "clm-host",
		},
		//"EventMetrics": "Humidity"
	}

	data2 := data1
	data2.Readings = []dtos.BaseReading{
		reading2,
	}

	data3 := data1
	data3.Readings = append(data3.Readings, reading4)

	data4 := metricDataNoSampling
	data4.Tags["EventMetrics"] = "isWorking"

	dataPause := data1
	dataPause.Readings = []dtos.BaseReading{
		reading5,
	}

	mockedRedisDbClient.On("SetMetricCounter", mock.Anything, mock.Anything).Return(nil)
	mockedTelemetry := &Telemetry{
		inMessages:   metrics.NewCounter(),
		inRawMetrics: metrics.NewCounter(),
	}
	mockedTelemetry.inMessages.Inc(int64(123))
	mockedTelemetry.redisClient = mockedRedisDbClient

	tests := []struct {
		name             string
		fields           fields
		args             args
		want             bool
		shouldOutputData bool
	}{
		// This test case fails when running full, but passes when run standalone, need to debug and fix: Girish
		{
			"TransformToAggregatesAndEvents - Pass: PauseDownSampling",
			flds,
			args{ctx, dataPause},
			true,
			true,
		},
		{
			"TransformToAggregatesAndEvents - Pass: no downsampling at all",
			flds,
			args{ctx, metricDataNoSampling},
			true,
			true,
		},
		{
			"TransformToAggregatesAndEvents - Pass: collect for downsampling",
			flds,
			args{ctx, data1},
			false,
			false,
		},
		{
			"TransformToAggregatesAndEvents - Pass: aggregate now",
			flds,
			args{ctx, data3},
			true,
			true,
		},
		{
			"TransformToAggregatesAndEvents - Pass: bypass bool for downsampling",
			flds,
			args{ctx, data2},
			true,
			true,
		},
		{
			"TransformToAggregatesAndEvents - Pass: extractEvent from metricdata",
			flds,
			args{ctx, data4},
			true,
			true,
		},
		{
			"TransformToAggregatesAndEvents - Failed1",
			flds,
			args{ctx, nil},
			false,
			true,
		}, //outputs error
		{"TransformToAggregatesAndEvents - Failed2", flds, args{ctx, []byte(nil)}, false, true},
		{
			"TransformToAggregatesAndEvents - Failed3",
			flds,
			args{ctx, []byte("mocked data")},
			false,
			true,
		},
		{"TransformToAggregatesAndEvents - Failed4", flds, args{ctx, "mocked data"}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// disable raw data send on test case 2, "Pass: collect for downsampling"
			if strings.Contains(tt.name, "collect for downsampling") {
				mockCache = new(cache.MockManager)
				mockCache.On("FetchRawDataConfiguration", mock.AnythingOfType("string")).
					Return(nodeDisableRawDataConfig, true)
				mockCache.On("FetchDownSamplingConfiguration", mock.AnythingOfType("string")).
					Return(downSamplingCfg, true)
			} else {
				mockCache.On("FetchDeviceTags", mock.AnythingOfType("string")).Return([]string{}, true)
				mockCache.On("FetchDownSamplingConfiguration", mock.AnythingOfType("string")).Return(downSamplingCfg, true)
				mockCache.On("FetchRawDataConfiguration", mock.AnythingOfType("string")).Return(nodeRawDataConfig, true)
			}

			s := &DataEnricher{
				service:               mockService,
				nodeName:              tt.fields.nodeName,
				EventsPublisherURL:    tt.fields.eventsPublisherURL,
				redisClient:           tt.fields.redisClient,
				deviceToDeviceInfoMap: tt.fields.deviceToDeviceInfoMap,
				telemetry:             mockedTelemetry,
				cache:                 mockCache,
				aggregateBuilder:      NewAggregateBuilder(mockLogger, mockCache, "test-node"),
			}
			if strings.Contains(tt.name, "aggregate") {
				// Earlier use case should have run
				s.TransformToAggregatesAndEvents(tt.args.ctx, data1)

				metricAgg := s.aggregateBuilder.device2MetricAggregatorMap["HVAC-A"]
				aggrByMetric := metricAgg.aggregateGroupByMetric["Humidity"]
				aggrByMetric.StartTimeStamp -= DOWNSAMPLINGINTERVALSECS * 1000000000 // reduce by DOWNSAMPLINGINTERVALSECS seconds
			}

			// Now execute the test case with test data in args.data
			got, pipelineData := s.TransformToAggregatesAndEvents(tt.args.ctx, tt.args.data)
			if got != tt.want {
				t.Errorf("TransformToAggregatesAndEvents() got = %v, want %v", got, tt.want)
			}
			if pipelineData == nil && tt.shouldOutputData ||
				pipelineData != nil && !tt.shouldOutputData {
				t.Errorf(
					"TransformToAggregatesAndEvents() got = %v, shouldOutputData %v",
					pipelineData != nil,
					tt.shouldOutputData,
				)
			}
		})
	}
}

func TestPubSub_SendToEventPublisher(t *testing.T) {
	Init()
	bmcEvent := comModels.HedgeEvent{
		Id:               "ba2d37a0-ddbe-4070-a476-0dcf9d59d056",
		Class:            "OT_EVENT",
		EventType:        "EVENT",
		DeviceName:       "Telco-Tower-OSS",
		Name:             "FaultStatus",
		Msg:              "Telco-Tower-OSS raised an event with metric FaultStatus:true",
		Severity:         comModels.SEVERITY_MAJOR,
		Priority:         "",
		Profile:          "Telco-Tower-OSS",
		SourceNode:       "",
		Status:           "Open",
		RelatedMetrics:   []string{"FaultStatus"},
		Thresholds:       map[string]interface{}{"Threshold": 0},
		ActualValues:     map[string]interface{}{"ActualValue": 0},
		ActualValueStr:   "true",
		Unit:             "",
		Location:         "",
		Version:          0,
		AdditionalData:   nil,
		EventSource:      "DeviceService",
		CorrelationId:    "Telco-Tower-OSS:FaultStatus",
		Labels:           nil,
		Remediations:     nil,
		Created:          1698866730636,
		Modified:         0,
		IsNewEvent:       false,
		IsRemediationTxn: false,
	}
	// data
	bmcEvents := make([]comModels.HedgeEvent, 0)
	bmcEvents = append(bmcEvents, bmcEvent)

	metricPayload := buildAggregateMetricPayload()
	targetPayloadFull := TargetPayload{
		Events:  bmcEvents,
		Metrics: metricPayload,
	}

	targetPayloadFullArgs := args{
		ctx:  ctx,
		data: targetPayloadFull,
	}

	targetPayload := TargetPayload{
		Events:  []comModels.HedgeEvent{bmcEvent},
		Metrics: nil,
	}
	targetOnlyEventArgs := args{
		ctx:  ctx,
		data: targetPayload,
	}

	metricOnlyEventArgs := args{
		ctx: ctx,
		data: TargetPayload{
			Events:  nil,
			Metrics: metricPayload,
		},
	}

	gob.Register(comModels.HedgeEvent{})

	// 2 events together
	targetPayload1 := TargetPayload{
		Events:  []comModels.HedgeEvent{bmcEvent, bmcEvent},
		Metrics: nil,
	}

	twoEventsArgs := args{
		ctx:  ctx,
		data: targetPayload1,
	}

	httpClient.RegisterExternalMockRestCall("/api/v3/trigger", "POST", mock.Anything, 200, nil)

	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
		want1  interface{}
	}{
		{
			"SendToEventPublisher - Pass BothMetricAndEvent",
			flds,
			targetPayloadFullArgs,
			true,
			metricPayload,
		},
		{"SendToEventPublisher - Pass OnlyEvent", flds, targetOnlyEventArgs, false, nil},
		{"SendToEventPublisher - Pass 2 Event", flds, twoEventsArgs, false, nil},
		{
			"SendToEventPublisher - Pass Only MetricData, no event",
			flds,
			metricOnlyEventArgs,
			true,
			metricPayload,
		},
		{
			"SendToEventPublisher - Fail nil data",
			flds,
			args{ctx, nil},
			false,
			errors.New("invalid TargetPayload received"),
		},
		{
			"SendToEventPublisher - Fail - Invalid payload",
			flds,
			args{ctx, []byte{}},
			false,
			errors.New("invalid TargetPayload received"),
		},
		{
			"SendToEventPublisher - eventPublishError",
			flds,
			targetPayloadFullArgs,
			true,
			metricPayload,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DataEnricher{
				nodeName:              tt.fields.nodeName,
				EventsPublisherURL:    tt.fields.eventsPublisherURL,
				redisClient:           tt.fields.redisClient,
				deviceToDeviceInfoMap: tt.fields.deviceToDeviceInfoMap,
				telemetry:             tt.fields.telemetry,
				cache:                 mockCache,
				//PersistOnError:        tt.fields.persistOnError,
			}
			if strings.Contains(tt.name, "eventPublishError") {
				httpClient.RegisterExternalMockRestCall(
					"/api/v3/trigger",
					"POST",
					mock.Anything,
					502,
					nil,
				)
			}
			ok, pipelinePayload := s.SendToEventPublisher(tt.args.ctx, tt.args.data)

			if ok != tt.want {
				t.Errorf("SendToEventPublisher() got = %v, want %v", pipelinePayload, tt.want)
			}
			if !reflect.DeepEqual(pipelinePayload, tt.want1) {
				t.Errorf("ExportToEventPublisher() got1 = %v, want %v", pipelinePayload, tt.want1)
			}
		})
	}
}

func TestPubSub_MergeMaps(t *testing.T) {
	Init()
	var ps DataEnricher

	type args struct {
		ctx    interfaces.AppFunctionContext
		exMap  map[string]interface{}
		newMap map[string]interface{}
		nodeId string
	}
	nmap := map[string]interface{}{
		"devicelabels": "Telco-Tower-A,HVAC,Temperature,Pressure,Humidity",
		"host":         "node01",
	}
	delete(metricEvent.Tags, "host")
	delete(metricEvent.Tags, "manufacturer")
	delete(metricEvent.Tags, "model")

	exMap := map[string]interface{}{
		"existingKey1": "existingValue1",
		"existingKey2": "existingValue2",
		"devicelabels": "label1",
	}
	newMap := map[string]interface{}{
		"newKey1":      "newValue1",
		"newKey2":      "newValue2",
		"existingKey2": "newValueForExistingKey2",
		"devicelabels": "label1",
	}
	resultMap := map[string]interface{}{
		"devicelabels": "label1",
		"existingKey1": "existingValue1",
		"existingKey2": "existingValue2",
		"host":         "node01",
		"newKey1":      "newValue1",
		"newKey2":      "newValue2"}

	arg := args{
		exMap:  metricEvent.Tags,
		newMap: nil,
		nodeId: "node01",
	}
	arg1 := args{exMap: nil, ctx: ctx}
	arg3 := args{
		exMap:  exMap,
		newMap: newMap,
		nodeId: "node01",
	}

	emptyMap := map[string]interface{}{}
	emptyMap["host"] = ""

	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{"addToMap - Passed", arg, nmap},
		{"addToMap - Failed1", arg1, emptyMap},
		{"addToMap - Failed2", arg3, resultMap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ps.MergeMaps(tt.args.exMap, tt.args.newMap, tt.args.nodeId); !reflect.DeepEqual(
				got,
				tt.want,
			) {
				t.Errorf("addToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTelemetry_IncomingMessage(t *testing.T) {
	Init()
	inMessagesCounter := metrics.NewCounter()
	inRawMetricsCounter := metrics.NewCounter()

	tel := &Telemetry{
		inMessages:   inMessagesCounter,
		inRawMetrics: inRawMetricsCounter,
		redisClient:  mockedRedisDbClient,
	}

	reading1 := dtos.BaseReading{
		ValueType: "Int32",
	}

	reading2 := dtos.BaseReading{
		ValueType: "String",
		SimpleReading: dtos.SimpleReading{
			Value: "123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"},
	}

	tel.IncomingMessage(reading1)
	expectedInMessagesCount := int64(1)
	expectedInRawMetricsCount := int64(1)

	// Check if the metric counters have been incremented correctly
	if tel.inMessages.Count() != expectedInMessagesCount {
		t.Errorf(
			"inMessages counter = %v, want %v",
			tel.inMessages.Count(),
			expectedInMessagesCount,
		)
	}
	if tel.inRawMetrics.Count() != expectedInRawMetricsCount {
		t.Errorf(
			"inRawMetrics counter = %v, want %v",
			tel.inRawMetrics.Count(),
			expectedInRawMetricsCount,
		)
	}

	// Call IncomingMessage again to test multiple increments
	tel.IncomingMessage(reading2)
	expectedInMessagesCount = int64(4)
	expectedInRawMetricsCount = int64(2)

	// Check if the metric counters have been incremented correctly
	if tel.inMessages.Count() != expectedInMessagesCount {
		t.Errorf(
			"inMessages counter = %v, want %v",
			tel.inMessages.Count(),
			expectedInMessagesCount,
		)
	}
	if tel.inRawMetrics.Count() != expectedInRawMetricsCount {
		t.Errorf(
			"inRawMetrics counter = %v, want %v",
			tel.inRawMetrics.Count(),
			expectedInRawMetricsCount,
		)
	}
}

func TestNewTelemetry(t *testing.T) {
	Init()
	type args struct {
		service        interfaces.ApplicationService
		serviceName    string
		metricsManager inter.MetricsManager
		hostName       string
		redisClient    cl.DBClientInterface
	}

	mockedRedisDbClient.On("GetMetricCounter", mock.Anything).Return(int64(10), nil)
	mockedRedisDbClient1 := &client.MockDBClientInterface{}
	mockedRedisDbClient1.On("GetMetricCounter", mock.Anything).
		Return(int64(0), hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, "mocked error from db"))

	expectedTelemetry := &Telemetry{
		inMessages:   metrics.NewCounter(),
		inRawMetrics: metrics.NewCounter(),
	}
	expectedTelemetry.inMessages.Inc(10)
	expectedTelemetry.inRawMetrics.Inc(10)

	arg1 := args{
		service:        u.AppService,
		serviceName:    "Test Service",
		metricsManager: metricsManager.MetricsMgr,
		hostName:       "localhost",
		redisClient:    mockedRedisDbClient,
	}
	arg2 := args{
		service:        u.AppService,
		serviceName:    "Test Service",
		metricsManager: metricsManager.MetricsMgr,
		hostName:       "localhost",
		redisClient:    mockedRedisDbClient1,
	}
	tests := []struct {
		name string
		args args
		want *Telemetry
	}{
		{"NewTelemetry - Passed", arg1, expectedTelemetry},
		{"NewTelemetry - Failed", arg2, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.name, "Failed") {
				got, _ := NewTelemetry(
					tt.args.service,
					tt.args.serviceName,
					tt.args.metricsManager,
					tt.args.hostName,
					tt.args.redisClient,
					1,
				)
				if got != nil {
					t.Errorf("NewTelemetry() == %v, want %v", got, tt.want)
				}
			} else {
				got, _ := NewTelemetry(tt.args.service, tt.args.serviceName, tt.args.metricsManager, tt.args.hostName, tt.args.redisClient, 1)

				if !reflect.DeepEqual(got.inMessages.Count(), tt.want.inMessages.Count()) {
					t.Errorf("NewTelemetry().inMessages.Count() = %v, want %v", got.inMessages.Count(), tt.want.inMessages.Count())
				}
				if !reflect.DeepEqual(got.inRawMetrics.Count(), tt.want.inRawMetrics.Count()) {
					t.Errorf("NewTelemetry().inRawMetrics.Count() = %v, want %v", got.inRawMetrics.Count(), tt.want.inRawMetrics.Count())
				}
			}
		})
	}
}

func buildAggregateMetricPayload() *dto.Metrics {
	sample1 := dto.Data{
		Name:      "Temperature",
		TimeStamp: 1000,
		Value:     "30",
		ValueType: common.ValueTypeFloat64,
	}

	sample2 := dto.Data{
		Name:      "Temperature_avg",
		TimeStamp: 1010,
		Value:     "25",
		ValueType: common.ValueTypeFloat64,
	}

	metricGroup := dto.MetricGroup{
		Tags:    map[string]any{"model": "SZKKKLG"},
		Samples: []dto.Data{sample1, sample2},
	}

	metric := dto.Metrics{
		IsCompressed: false,
		MetricGroup:  metricGroup,
	}

	return &metric
}
