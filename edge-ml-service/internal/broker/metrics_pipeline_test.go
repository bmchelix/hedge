package broker

import (
	"hedge/common/dto"
	"hedge/edge-ml-service/pkg/dto/data"
	"hedge/edge-ml-service/pkg/helpers"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	appCtx *mocks.AppFunctionContext
	inf    *MLBrokerInferencing
)

func Init1() {
	inf, _, _ = BuildMLBrokerPipelineMocks(helpers.ANOMALY_ALGO_TYPE)
	u := utils.NewApplicationServiceMock(map[string]string{})
	appCtx = &mocks.AppFunctionContext{}
	appCtx.On("LoggingClient").Return(u.AppService.LoggingClient())
	appCtx.On("PipelineId").Return("12345")
}

func TestMLBrokerInferencing_FilterMetricsByFeatureNames(t *testing.T) {
	Init1()
	type args struct {
		edgexcontext interfaces.AppFunctionContext
		eventData    interface{}
	}
	args1 := args{
		edgexcontext: appCtx,
		eventData:    "invalid_data", // Not of type dto.MetricGroup
	}
	args2 := args{
		edgexcontext: appCtx,
		eventData: dto.MetricGroup{
			Tags:    map[string]interface{}{}, // No profileName
			Samples: []dto.Data{},
		},
	}
	args3 := args{
		edgexcontext: appCtx,
		eventData: dto.MetricGroup{
			Tags: map[string]interface{}{
				"profileName": "NonExistentProfile",
			},
			Samples: []dto.Data{},
		},
	}
	args4 := args{
		edgexcontext: appCtx,
		eventData: dto.MetricGroup{
			Tags: map[string]interface{}{
				"profileName": "TestProfile",
			},
			Samples: []dto.Data{
				{Name: "NonMatchingMetric"},
			},
		},
	}
	args5 := args{
		edgexcontext: appCtx,
		eventData: dto.MetricGroup{
			Tags: map[string]interface{}{
				"profileName": "WindTurbine",
			},
			Samples: []dto.Data{
				{Name: "WindSpeed"},
				{Name: "TurbinePower"},
				{Name: "NonMatchingMetric"},
			},
		},
	}
	want1 := dto.MetricGroup{
		Samples: []dto.Data{
			{Name: "WindSpeed"},
			{Name: "TurbinePower"},
		},
		Tags: map[string]interface{}{
			"profileName": "WindTurbine",
		},
	}

	tests := []struct {
		name  string
		args  args
		want  bool
		want1 interface{}
	}{
		{"FilterMetricsByFeatureNames - Invalid Event Data Type", args1, true, "invalid_data"},
		{"FilterMetricsByFeatureNames - Missing Profile Name in Tags", args2, false, dto.MetricGroup{}},
		{"FilterMetricsByFeatureNames - No Matching Profile", args3, false, dto.MetricGroup{}},
		{"FilterMetricsByFeatureNames - No Matching Features", args4, false, dto.MetricGroup{}},
		{"FilterMetricsByFeatureNames - Matching Features Found", args5, true, want1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := inf.FilterMetricsByFeatureNames(tt.args.edgexcontext, tt.args.eventData)
			assert.Equalf(t, tt.want, got, "FilterMetricsByFeatureNames(%v, %v)", tt.args.edgexcontext, tt.args.eventData)
			assert.Equalf(t, tt.want1, got1, "FilterMetricsByFeatureNames(%v, %v)", tt.args.edgexcontext, tt.args.eventData)
		})
	}
}

func TestMLBrokerInferencing_TakeMetricSample(t *testing.T) {
	Init1()
	type args struct {
		edgexcontext interfaces.AppFunctionContext
		eventData    interface{}
	}
	args1 := args{
		edgexcontext: appCtx,
		eventData:    "invalid_data", // Not of type dto.MetricGroup
	}
	args2 := args{
		edgexcontext: appCtx,
		eventData: dto.MetricGroup{
			Tags:    map[string]interface{}{}, // No profileName in tags
			Samples: []dto.Data{},
		},
	}
	args3 := args{
		edgexcontext: appCtx,
		eventData: dto.MetricGroup{
			Tags: map[string]interface{}{
				"profileName": "WindTurbine",
			},
			Samples: []dto.Data{}, // No samples present
		},
	}
	args4 := args{
		edgexcontext: appCtx,
		eventData: dto.MetricGroup{
			Tags: map[string]interface{}{
				"profileName":  "WindTurbine",
				"location":     "Lab1",
				"equipmentID":  "1234",
				"extraInfo":    "value",
				"anotherField": "unused",
			},
			Samples: []dto.Data{
				{Name: "WindSpeed", Value: "25.6", ValueType: common.ValueTypeFloat64, TimeStamp: 1728126515693},
				{Name: "TurbinePower", Value: "45.2", ValueType: common.ValueTypeFloat64, TimeStamp: 1728126515693},
			},
		},
	}
	want1 := []*data.TSDataElement{
		{
			Profile:         "WindTurbine",
			DeviceName:      "GENERIC",
			Timestamp:       1728126,
			MetricName:      "WindSpeed",
			Value:           25.6,
			Tags:            map[string]interface{}{"profileName": "WindTurbine", "location": "Lab1", "equipmentID": "1234", "extraInfo": "value", "anotherField": "unused"},
			GroupByMetaData: map[string]string{"deviceName": "GENERIC"},
			MetaData:        map[string]string{"profileName": "WindTurbine", "location": "Lab1", "equipmentID": "1234", "extraInfo": "value", "anotherField": "unused"},
		},
		{
			Profile:         "WindTurbine",
			DeviceName:      "GENERIC",
			Timestamp:       1728126,
			MetricName:      "TurbinePower",
			Value:           45.2,
			Tags:            map[string]interface{}{"profileName": "WindTurbine", "location": "Lab1", "equipmentID": "1234", "extraInfo": "value", "anotherField": "unused"},
			GroupByMetaData: map[string]string{"deviceName": "GENERIC"},
			MetaData:        map[string]string{"profileName": "WindTurbine", "location": "Lab1", "equipmentID": "1234", "extraInfo": "value", "anotherField": "unused"},
		},
	}

	tests := []struct {
		name  string
		args  args
		want  bool
		want1 interface{}
	}{
		{"TakeMetricSample - Invalid Event Data Type", args1, true, "invalid_data"},
		{"TakeMetricSample - Missing Profile Name in Tags", args2, false, nil},
		{"TakeMetricSample - No Metrics to Sample", args3, true, []*data.TSDataElement{}},
		{"TakeMetricSample - Successfully Take Metric Samples", args4, true, want1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := inf.TakeMetricSample(tt.args.edgexcontext, tt.args.eventData)
			assert.Equalf(t, tt.want, got, "TakeMetricSample(%v, %v)", tt.args.edgexcontext, tt.args.eventData)
			assert.Equalf(t, tt.want1, got1, "TakeMetricSample(%v, %v)", tt.args.edgexcontext, tt.args.eventData)
		})
	}
}
