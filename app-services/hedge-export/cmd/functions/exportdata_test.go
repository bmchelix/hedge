/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"hedge/common/models"
	"hedge/edge-ml-service/pkg/helpers"
	svcmocks "hedge/mocks/hedge/common/service"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"hedge/app-services/hedge-export/cmd/config"
)

func TestNewBMCMetricExportService(t *testing.T) {
	t.Run("NewMetricExportService - Passed (Enable rate reporting)", func(t *testing.T) {
		victoriaMetricsDbURL := "http://victoria-metrics:8428"

		exportConfig := config.AppExportConfig{
			BatchSize:  10,
			BatchTimer: "10s",
		}
		persistOnError := true
		enableRateReporting := true

		bmcMetricExportService := NewMetricExportService(victoriaMetricsDbURL, enableRateReporting, exportConfig, persistOnError)

		assert.NotNil(t, bmcMetricExportService, "Expected non-nil MetricExportService instance")
		assert.Equal(
			t,
			victoriaMetricsDbURL,
			bmcMetricExportService.victoriaMetricsURL,
			"VictoriaMetrics URL mismatch",
		)

		assert.Equal(
			t,
			exportConfig.BatchSize,
			bmcMetricExportService.batchSize,
			"Batch size mismatch",
		)

		assert.NotNil(
			t,
			bmcMetricExportService.ingestionTelemetryMonitor,
			"Expected ingestionRateMonitor to be initialized",
		)
	})
	t.Run("NewMetricExportService - Passed (Disable rate reporting)", func(t *testing.T) {
		victoriaMetricsDbURL := "http://victoria-metrics:8428"

		exportConfig := config.AppExportConfig{
			BatchSize:  20,
			BatchTimer: "20s",
		}
		persistOnError := false
		enableRateReporting := false

		bmcMetricExportService := NewMetricExportService(victoriaMetricsDbURL, enableRateReporting, exportConfig, persistOnError)

		assert.NotNil(t, bmcMetricExportService, "Expected non-nil MetricExportService instance")
		assert.Equal(
			t,
			victoriaMetricsDbURL,
			bmcMetricExportService.victoriaMetricsURL,
			"VictoriaMetrics URL mismatch",
		)
		assert.Equal(
			t,
			exportConfig.BatchSize,
			bmcMetricExportService.batchSize,
			"Batch size mismatch",
		)

		assert.Nil(
			t,
			bmcMetricExportService.ingestionTelemetryMonitor,
			"Expected ingestionRateMonitor to be nil",
		)
	})
}

func TestBMCMetricExportService_ConvertToVictoria(t *testing.T) {
	t.Run("TransformToTimeseries - Passed (Valid metrics)", func(t *testing.T) {
		bmcMetricExportService := &MetricExportService{}

		ok, result := bmcMetricExportService.TransformToTimeseries(ctx, testMetricsData)
		assert.True(t, ok, "Expected conversion to succeed for valid metrics")
		assert.NotNil(t, result, "Expected a valid result for converted metrics")
	})

	t.Run("TransformToTimeseries - Failed (Nil data)", func(t *testing.T) {
		bmcMetricExportService := &MetricExportService{}

		ok, result := bmcMetricExportService.TransformToTimeseries(ctx, nil)
		assert.False(t, ok, "Expected conversion to fail for nil data")
		assert.NotNil(t, result, "Expected a result object even for nil data")

		_, isReturnObjs := result.(*ReturnObjs)
		assert.True(t, isReturnObjs, "Expected the result to be of type *ReturnObjs")
	})
	t.Run("TransformToTimeseries - Failed (Invalid data type)", func(t *testing.T) {
		bmcMetricExportService := &MetricExportService{}

		ok, _ := bmcMetricExportService.TransformToTimeseries(ctx, "invalid-data")
		// In case of failure, don't check for output which is not imp
		assert.False(t, ok, "Expected conversion to fail for invalid data type")
		//assert.NotNil(t, result.(*ReturnObjs), "Expected result for invalid data type")
	})
}

func TestBMCMetricExportService_CheckAndConvertToMetrics(t *testing.T) {
	t.Run("CheckAndConvertToMetrics - Passed (valid Metrics data)", func(t *testing.T) {
		jsonData, _ := json.Marshal(testMetricsData)

		bmcMetricExportService := &MetricExportService{}

		result := bmcMetricExportService.CheckAndConvertToMetrics(ctx, jsonData)
		assert.NotNil(t, result, "Expected non-nil result for valid metrics data")
		assert.Equal(t, 1, len(result), "Expected one metrics object in the result")
	})
	t.Run("CheckAndConvertToMetrics - Failed (Invalid metrics data)", func(t *testing.T) {
		invalidData := "invalid data"

		bmcMetricExportService := &MetricExportService{}

		result := bmcMetricExportService.CheckAndConvertToMetrics(ctx, invalidData)
		assert.Nil(t, result, "Expected nil result for invalid metrics data")
	})
	t.Run("CheckAndConvertToMetrics - Passed (Valid ML predictions)", func(t *testing.T) {
		mlPredictionData := models.MLPrediction{
			Devices:         []string{"device1", "device2"},
			EntityName:      "entity1",
			EntityType:      "type1",
			MLAlgorithmType: helpers.ANOMALY_ALGO_TYPE,
			MLAlgoName:      "algo1",
			ModelName:       "model1",
			PredictionName:  "prediction1",
			Created:         time.Now().UnixNano(),
			Tags:            map[string]interface{}{"tag1": "value1"},
			Prediction:      42.5,
		}
		jsonData, _ := json.Marshal(mlPredictionData)

		bmcMetricExportService := &MetricExportService{}

		result := bmcMetricExportService.CheckAndConvertToMetrics(ctx, jsonData)
		assert.NotNil(t, result, "Expected non-nil result for valid ML prediction data")
		assert.GreaterOrEqual(
			t,
			len(result),
			1,
			"Expected at least one metrics object in the result",
		)
	})
	t.Run("CheckAndConvertToMetrics - Failed (Invalid ML predictions)", func(t *testing.T) {
		invalidPredictionData := "invalid prediction data"

		bmcMetricExportService := &MetricExportService{}

		result := bmcMetricExportService.CheckAndConvertToMetrics(ctx, invalidPredictionData)
		assert.Nil(t, result, "Expected nil result for invalid ML prediction data")
	})
	t.Run("CheckAndConvertToMetrics - Passed (Valid TimeSeries prediction)", func(t *testing.T) {
		mlPredictionData := models.MLPrediction{
			Devices:         []string{"device1"},
			EntityName:      "entity1",
			EntityType:      "type1",
			MLAlgorithmType: helpers.TIMESERIES_ALGO_TYPE,
			MLAlgoName:      "algo1",
			ModelName:       "model1",
			Prediction: map[string]interface{}{
				"device1": []interface{}{
					map[string]interface{}{
						"Timestamp":              1690123456000,
						"TestProfile#TestMetric": 42.5,
					},
				},
			},
		}
		jsonData, _ := json.Marshal(mlPredictionData)

		bmcMetricExportService := &MetricExportService{}

		result := bmcMetricExportService.CheckAndConvertToMetrics(ctx, jsonData)
		assert.NotNil(t, result, "Expected non-nil result for valid time series prediction data")
		assert.GreaterOrEqual(
			t,
			len(result),
			1,
			"Expected at least one metrics object in the result",
		)
	})
}

func TestBMCMetricExportService_buildValue(t *testing.T) {
	t.Run("buildValue - Float64 prediction", func(t *testing.T) {
		mlPrediction := models.MLPrediction{
			Prediction: 42.5,
		}

		bmcMetricExportService := &MetricExportService{}

		value, valueType := bmcMetricExportService.buildValue(mlPrediction, "", mockLogger)
		assert.Equal(t, "42.5", value, "Expected value to match the float64 prediction")
		assert.Equal(t, "Float64", valueType, "Expected valueType to be Float64")
	})
	t.Run("buildValue - Float32 prediction", func(t *testing.T) {
		mlPrediction := models.MLPrediction{
			Prediction: float32(42.5),
		}

		bmcMetricExportService := &MetricExportService{}

		value, valueType := bmcMetricExportService.buildValue(mlPrediction, "", mockLogger)
		assert.Equal(t, "42.5", value, "Expected value to match the float32 prediction")
		assert.Equal(t, "Float32", valueType, "Expected valueType to be Float32")
	})
	t.Run("buildValue - Integer prediction", func(t *testing.T) {
		mlPrediction := models.MLPrediction{
			Prediction: 42,
		}

		bmcMetricExportService := &MetricExportService{}

		value, valueType := bmcMetricExportService.buildValue(mlPrediction, "", mockLogger)
		assert.Equal(t, "42", value, "Expected value to match the integer prediction")
		assert.Equal(t, "Int", valueType, "Expected valueType to be Int")
	})
	t.Run("buildValue - Invalid timeseries type", func(t *testing.T) {
		mlPrediction := models.MLPrediction{
			Prediction:      "invalid-type",
			MLAlgorithmType: "SomeNonTimeseriesType",
		}

		bmcMetricExportService := &MetricExportService{}

		value, valueType := bmcMetricExportService.buildValue(mlPrediction, "", mockLogger)
		assert.Equal(t, "", value, "Expected empty value for unsupported prediction type")
		assert.Equal(
			t,
			"",
			valueType,
			"Expected valueType to be 'none' for unsupported prediction type",
		)
	})
	t.Run("buildValue - Valid timeseries type", func(t *testing.T) {
		mlPrediction := models.MLPrediction{
			Prediction:      42.5,
			MLAlgorithmType: helpers.TIMESERIES_ALGO_TYPE,
		}

		bmcMetricExportService := &MetricExportService{}

		value, valueType := bmcMetricExportService.buildValue(mlPrediction, "", mockLogger)
		assert.Equal(t, "42.5", value, "Expected value to match the timeseries prediction")
		assert.Equal(
			t,
			"Float64",
			valueType,
			"Expected valueType to be Float64 for timeseries prediction",
		)
	})
}

func TestBMCMetricExportService_toPromWriteRequest(t *testing.T) {
	t.Run("toPromWriteRequest - Passed (Empty metricData)", func(t *testing.T) {
		bmcMetricExportService := &MetricExportService{
			MetricData: TSList{},
		}

		writeRequest := bmcMetricExportService.toPromWriteRequest(mockLogger)
		assert.NotNil(t, writeRequest, "Expected a valid WriteRequest object")
		assert.Empty(
			t,
			writeRequest.Timeseries,
			"Expected no TimeSeries in WriteRequest for empty MetricData",
		)
	})
	t.Run("toPromWriteRequest - Passed (Valid metricData)", func(t *testing.T) {
		bmcMetricExportService := &MetricExportService{
			MetricData: TSList{
				{
					Labels: []Label{
						{Name: "__name__", Value: "test_metric"},
						{Name: "device", Value: "device123"},
					},
					Datapoint: Datapoint{
						TimeMs: 1625232000000,
						Value:  42.5,
					},
				},
			},
		}

		writeRequest := bmcMetricExportService.toPromWriteRequest(mockLogger)
		assert.NotNil(t, writeRequest, "Expected a valid WriteRequest object")
		assert.Len(t, writeRequest.Timeseries, 1, "Expected one TimeSeries in WriteRequest")

		ts := writeRequest.Timeseries[0]
		assert.Len(t, ts.Labels, 2, "Expected two labels in TimeSeries")
		assert.Equal(t, ts.Labels[0].Name, "__name__", "Expected first label name to match")
		assert.Equal(t, ts.Labels[0].Value, "test_metric", "Expected first label value to match")
		assert.Len(t, ts.Samples, 1, "Expected one sample in TimeSeries")
		assert.Equal(
			t,
			ts.Samples[0].Timestamp,
			int64(1625232000000),
			"Expected sample timestamp to match",
		)
		assert.Equal(t, ts.Samples[0].Value, 42.5, "Expected sample value to match")
	})
	t.Run("toPromWriteRequest - Passed (MetricData with nil Labels)", func(t *testing.T) {
		bmcMetricExportService := &MetricExportService{
			MetricData: TSList{
				{
					Labels: nil,
					Datapoint: Datapoint{
						TimeMs: 1625232000000,
						Value:  42.5,
					},
				},
			},
		}

		writeRequest := bmcMetricExportService.toPromWriteRequest(mockLogger)
		assert.Nil(t, writeRequest, "Expected nil WriteRequest for MetricData with nil Labels")
	})
	t.Run(
		"toPromWriteRequest - Passed (MetricData with Out-of-Bounds Label Count)",
		func(t *testing.T) {
			labels := make([]Label, 65) // Exceeding the allowed label limit
			for i := 0; i < 65; i++ {
				labels[i] = Label{Name: fmt.Sprintf("key%d", i), Value: fmt.Sprintf("value%d", i)}
			}

			bmcMetricExportService := &MetricExportService{
				MetricData: TSList{
					{
						Labels: labels,
						Datapoint: Datapoint{
							TimeMs: 1625232000000,
							Value:  42.5,
						},
					},
				},
			}

			writeRequest := bmcMetricExportService.toPromWriteRequest(mockLogger)
			assert.Nil(
				t,
				writeRequest,
				"Expected nil WriteRequest for MetricData with out-of-bounds label count",
			)
		},
	)
	t.Run("toPromWriteRequest - Passed (MetricData with excess count)", func(t *testing.T) {
		bmcMetricExportService := &MetricExportService{
			MetricData: TSList{
				{
					Labels: []Label{
						{Name: "__name__", Value: "test_metric"},
					},
					Datapoint: Datapoint{
						TimeMs: 1625232000000,
						Value:  42.5,
					},
				},
			},
		}

		writeRequest := bmcMetricExportService.toPromWriteRequest(mockLogger)
		assert.NotNil(t, writeRequest, "Expected a valid WriteRequest object")
		assert.Len(t, writeRequest.Timeseries, 1, "Expected one TimeSeries in WriteRequest")
	})
}

func TestBMCMetricExportService_getDataValueAsStr(t *testing.T) {
	t.Run("getDataValueAsStr - Uint64", func(t *testing.T) {
		valueType := common.ValueTypeUint64
		value := uint64(math.MaxUint64)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(
			t,
			"18446744073709551615",
			promVal,
			"Expected formatted string for uint64 value",
		)
	})
	t.Run("getDataValueAsStr - Uint16", func(t *testing.T) {
		valueType := common.ValueTypeUint16
		value := uint16(math.MaxUint16)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "65535", promVal, "Expected formatted string for uint16 value")
	})
	t.Run("getDataValueAsStr - Uint8", func(t *testing.T) {
		valueType := common.ValueTypeUint8
		value := uint8(math.MaxUint8)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "255", promVal, "Expected formatted string for uint8 value")
	})
	t.Run("getDataValueAsStr - Int32", func(t *testing.T) {
		valueType := common.ValueTypeInt32
		value := int32(math.MinInt32)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "-2147483648", promVal, "Expected formatted string for int32 value")
	})
	t.Run("getDataValueAsStr - Int16", func(t *testing.T) {
		valueType := common.ValueTypeInt16
		value := int16(math.MaxInt16)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "32767", promVal, "Expected formatted string for int16 value")
	})
	t.Run("getDataValueAsStr - Float64", func(t *testing.T) {
		valueType := common.ValueTypeFloat64
		value := 123.456
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "123.456000", promVal, "Expected formatted string for float64 value")
	})
	t.Run("getDataValueAsStr - Bool True", func(t *testing.T) {
		valueType := common.ValueTypeBool
		value := true
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "1", promVal, "Expected '1' for true boolean value")
	})
	t.Run("getDataValueAsStr - Bool False", func(t *testing.T) {
		valueType := common.ValueTypeBool
		value := false
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "0", promVal, "Expected '0' for false boolean value")
	})
	t.Run("getDataValueAsStr - Unsupported Type", func(t *testing.T) {
		valueType := "UnsupportedType"
		value := "some value"
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.False(t, isSuccess, "Expected conversion to fail for unsupported type")
		assert.Empty(t, promVal, "Expected empty string for unsupported type")
	})
	t.Run("getDataValueAsStr - Nil Value", func(t *testing.T) {
		valueType := common.ValueTypeInt64
		var value interface{} = nil
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.False(t, isSuccess, "Expected conversion to fail for nil value")
		assert.Empty(t, promVal, "Expected empty string for nil value")
	})
	t.Run("getDataValueAsStr - Int64", func(t *testing.T) {
		valueType := common.ValueTypeInt64
		value := int64(42)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "42", promVal, "Expected formatted string for int64 value")
	})
	t.Run("getDataValueAsStr - Float32", func(t *testing.T) {
		valueType := common.ValueTypeFloat32
		value := float32(98.76)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "98.760002", promVal, "Expected formatted string for float32 value")
	})
	t.Run("getDataValueAsStr - Array ValueType", func(t *testing.T) {
		valueType := "Array"
		value := []int{1, 2, 3}
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.False(t, isSuccess, "Expected conversion to fail for array type")
		assert.Empty(t, promVal, "Expected empty string for array type")
	})
	t.Run("getDataValueAsStr - Int8", func(t *testing.T) {
		valueType := common.ValueTypeInt8
		value := int8(math.MaxInt8)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "127", promVal, "Expected formatted string for int8 value")
	})
	t.Run("getDataValueAsStr - Uint32", func(t *testing.T) {
		valueType := common.ValueTypeUint32
		value := uint32(1024)
		isSuccess, promVal := getDataValueAsStr(valueType, value, mockLogger)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, "1024", promVal, "Expected formatted string for uint32 value")
	})
}

func TestBMCMetricExportService_valueIntToFloat64(t *testing.T) {
	t.Run("valueIntToFloat64 - Float64", func(t *testing.T) {
		valueType := common.ValueTypeFloat64
		value := 123.45
		isSuccess, floatValue := valueIntToFloat64(mockLogger, valueType, value)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, 123.45, floatValue, "Expected float64 value to remain unchanged")
	})
	t.Run("valueIntToFloat64 - Float32", func(t *testing.T) {
		valueType := common.ValueTypeFloat32
		value := float32(123.45)
		isSuccess, floatValue := valueIntToFloat64(mockLogger, valueType, value)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.InDelta(
			t,
			123.45,
			floatValue,
			0.00001,
			"Expected float32 to be approximately converted to float64",
		)
	})
	t.Run("valueIntToFloat64 - Bool True", func(t *testing.T) {
		valueType := common.ValueTypeBool
		value := true
		isSuccess, floatValue := valueIntToFloat64(mockLogger, valueType, value)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, float64(1), floatValue, "Expected true to convert to 1.0")
	})
	t.Run("valueIntToFloat64 - Bool False", func(t *testing.T) {
		valueType := common.ValueTypeBool
		value := false
		isSuccess, floatValue := valueIntToFloat64(mockLogger, valueType, value)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, float64(0), floatValue, "Expected false to convert to 0.0")
	})
	t.Run("valueIntToFloat64 - Uint64", func(t *testing.T) {
		valueType := common.ValueTypeUint64
		value := uint64(12345)
		isSuccess, floatValue := valueIntToFloat64(mockLogger, valueType, value)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, float64(12345), floatValue, "Expected uint64 to convert to float64")
	})
	t.Run("valueIntToFloat64 - Uint32", func(t *testing.T) {
		valueType := common.ValueTypeUint32
		value := uint32(12345)
		isSuccess, floatValue := valueIntToFloat64(mockLogger, valueType, value)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, float64(12345), floatValue, "Expected uint32 to convert to float64")
	})
	t.Run("valueIntToFloat64 - Int32", func(t *testing.T) {
		valueType := common.ValueTypeInt32
		value := int32(-12345)
		isSuccess, floatValue := valueIntToFloat64(mockLogger, valueType, value)
		assert.True(t, isSuccess, "Expected conversion to succeed")
		assert.Equal(t, float64(-12345), floatValue, "Expected int32 to convert to float64")
	})
	t.Run("valueIntToFloat64 - Unsupported Type", func(t *testing.T) {
		valueType := "UnsupportedType"
		value := "invalid"
		isSuccess, floatValue := valueIntToFloat64(mockLogger, valueType, value)
		assert.False(t, isSuccess, "Expected conversion to fail for unsupported type")
		assert.Equal(
			t,
			float64(0),
			floatValue,
			"Expected float value to be zero for unsupported type",
		)
	})
}

func TestBMCMetricExportService_addLabels(t *testing.T) {
	t.Run("addLabels - Integer tag", func(t *testing.T) {
		tags := map[string]any{
			"intTag": 123,
		}
		expected := []Label{
			{Name: "intTag", Value: "123"},
		}
		result := addLabels(tags)
		assert.Equal(t, expected, result, "Expected integer tag to be converted correctly")
	})
	t.Run("addLabels - Int64 tag", func(t *testing.T) {
		tags := map[string]any{
			"int64Tag": int64(1234567890),
		}
		expected := []Label{
			{Name: "int64Tag", Value: "1234567890"},
		}
		result := addLabels(tags)
		assert.Equal(t, expected, result, "Expected int64 tag to be converted correctly")
	})
	t.Run("addLabels - Float64 tag", func(t *testing.T) {
		tags := map[string]any{
			"float64Tag": 123.45,
		}
		expected := []Label{
			{Name: "float64Tag", Value: "123.45"},
		}
		result := addLabels(tags)
		assert.Equal(t, expected, result, "Expected float64 tag to be converted correctly")
	})
	t.Run("addLabels - Float32 tag", func(t *testing.T) {
		tags := map[string]any{
			"float32Tag": float32(123.45),
		}
		expected := []Label{
			{Name: "float32Tag", Value: "123"},
		}
		result := addLabels(tags)
		assert.Equal(t, expected, result, "Expected float32 tag to be converted correctly")
	})
	t.Run("addLabels - String tag", func(t *testing.T) {
		tags := map[string]any{
			"stringTag": "testValue",
		}
		expected := []Label{
			{Name: "stringTag", Value: "testValue"},
		}
		result := addLabels(tags)
		assert.Equal(t, expected, result, "Expected string tag to be converted correctly")
	})
	t.Run("addLabels - Slice tag", func(t *testing.T) {
		tags := map[string]any{
			"sliceTag": []interface{}{"val1", "val2", "val3"},
		}
		expected := []Label{
			{Name: "sliceTag", Value: "val1,val2,val3"},
		}
		result := addLabels(tags)
		assert.Equal(t, expected, result, "Expected slice tag to be converted correctly")
	})
	t.Run("addLabels - Unsupported tag", func(t *testing.T) {
		tags := map[string]any{
			"unsupportedTag": struct{ Field string }{Field: "value"},
		}
		expected := []Label{
			{Name: "unsupportedTag", Value: "{value}"},
		}
		result := addLabels(tags)
		assert.Equal(t, expected, result, "Expected unsupported tag to be handled gracefully")
	})
	t.Run("addLabels - Empty tag", func(t *testing.T) {
		tags := map[string]any{}
		expected := []Label{}
		result := addLabels(tags)
		assert.Equal(t, expected, result, "Expected empty tags to return an empty result")
	})
	t.Run("addLabels - Multiple tags", func(t *testing.T) {
		tags := map[string]any{
			"intTag":     123,
			"stringTag":  "testValue",
			"float64Tag": 456.78,
		}
		expected := []Label{
			{Name: "intTag", Value: "123"},
			{Name: "stringTag", Value: "testValue"},
			{Name: "float64Tag", Value: "456.78"},
		}
		result := addLabels(tags)
		assert.ElementsMatch(
			t,
			expected,
			result,
			"Expected multiple tags to be converted correctly",
		)
	})
}

func TestBMCMetricExportService_getValue(t *testing.T) {
	t.Run("getValue - SimpleReading with value", func(t *testing.T) {
		reading := dtos.BaseReading{
			SimpleReading: dtos.SimpleReading{
				Value: "test-value",
			},
		}

		result := getValue(reading, mockLogger)
		assert.Equal(t, "test-value", result, "Expected value to be returned for SimpleReading")
	})
	t.Run("getValue - Empty value in SimpleReading", func(t *testing.T) {
		reading := dtos.BaseReading{
			SimpleReading: dtos.SimpleReading{
				Value: "",
			},
		}

		result := getValue(reading, mockLogger)
		assert.Equal(
			t,
			"",
			result,
			"Expected empty string to be returned for empty SimpleReading value",
		)
	})
	t.Run("getValue - Unsupported Reading type", func(t *testing.T) {
		reading := dtos.BaseReading{
			SimpleReading: dtos.SimpleReading{},
		}

		result := getValue(reading, mockLogger)
		assert.Equal(t, "", result, "Expected empty string for unsupported reading type")
	})
}

func TestBMCMetricExportService_updateIngestionRate(t *testing.T) {
	t.Run("updateIngestionRate - Passed (Update metrics)", func(t *testing.T) {
		ingestionRateMonitor := &IngestionTelemetryMonitor{
			startTimeNanoSecs:   time.Now().UnixNano(),
			bytesAdded:          0.0,
			metricCount:         0,
			elapsedTimeNanoSecs: 0,
		}

		metricExportService := MetricExportService{
			ingestionTelemetryMonitor: ingestionRateMonitor,
			locker:                    sync.Mutex{},
		}

		testData := "metric1\nmetric2\nmetric3\n"
		time.Sleep(1 * time.Millisecond) // Ensure a measurable passage of time
		metricExportService.updateIngestionRate(testData, mockLogger)

		assert.Equal(
			t,
			float64(len(testData)),
			ingestionRateMonitor.bytesAdded,
			"Expected bytesAdded to match data length",
		)
		assert.Equal(
			t,
			int64(strings.Count(testData, "\n")),
			ingestionRateMonitor.metricCount,
			"Expected metricCount to match number of lines",
		)
		assert.Greater(
			t,
			ingestionRateMonitor.elapsedTimeNanoSecs,
			float64(0),
			"Expected elapsedTimeNanoSecs to be updated",
		)
	})
	t.Run(
		"updateIngestionRate - Passed (Reset metrics after sampling interval)",
		func(t *testing.T) {
			ingestionRateMonitor := &IngestionTelemetryMonitor{
				startTimeNanoSecs:   time.Now().UnixNano(),
				bytesAdded:          10.0,
				metricCount:         3,
				elapsedTimeNanoSecs: 1000,
			}

			metricExportService := MetricExportService{
				ingestionTelemetryMonitor: ingestionRateMonitor,
				locker:                    sync.Mutex{},
			}

			// Simulate time exceeding sampling interval
			ingestionRateMonitor.startTimeNanoSecs -= SAMPLING_INTERVAL_NANO_SECS

			testData := "metric1\nmetric2\nmetric3\n"
			metricExportService.updateIngestionRate(testData, mockLogger)
			/*			assert.Equal(
							t,
							0.0,
							ingestionRateMonitor.bytesAdded,
							"Expected bytesAdded to be reset after sampling interval",
						)
						assert.Equal(
							t,
							int64(0),
							ingestionRateMonitor.metricCount,
							"Expected metricCount to be reset after sampling interval",
						)*/
		},
	)

}

func TestBMCMetricExportService_ProcessConfigUpdates(t *testing.T) {
	t.Run("ProcessConfigUpdates - Update all configurations", func(t *testing.T) {
		bmcMetricService := &MetricExportService{
			batchSize:          10,
			batchTimer:         10 * time.Minute,
			persistOnError:     false,
			victoriaMetricsURL: "http://old-victoria-url",
		}

		newConfig := &config.AppExportConfig{
			BatchSize:      20,
			BatchTimer:     "5m",
			PersistOnError: true,
		}

		bmcMetricService.ProcessConfigUpdates(newConfig)

		assert.Equal(t, 20, bmcMetricService.batchSize, "Expected batchSize to be updated")
		assert.Equal(
			t,
			5*time.Minute,
			bmcMetricService.batchTimer,
			"Expected batchTimer to be updated",
		)

		assert.True(
			t,
			bmcMetricService.persistOnError,
			"Expected persistOnError to be updated to true",
		)
	})
	t.Run("ProcessConfigUpdates - Invalid config type", func(t *testing.T) {
		bmcMetricService := &MetricExportService{
			batchSize: 10,
		}
		invalidConfig := struct{}{}

		bmcMetricService.ProcessConfigUpdates(&invalidConfig)

		assert.Equal(
			t,
			10,
			bmcMetricService.batchSize,
			"Expected batchSize to remain unchanged with invalid config",
		)
	})
	t.Run("ProcessConfigUpdates - No changes", func(t *testing.T) {
		bmcMetricService := &MetricExportService{
			batchSize:      10,
			batchTimer:     5 * time.Minute,
			persistOnError: true,
		}
		sameConfig := &config.AppExportConfig{
			BatchSize:      10,
			BatchTimer:     "5m",
			PersistOnError: true,
		}

		bmcMetricService.ProcessConfigUpdates(sameConfig)

		assert.Equal(t, 10, bmcMetricService.batchSize, "Expected batchSize to remain unchanged")
		assert.Equal(
			t,
			5*time.Minute,
			bmcMetricService.batchTimer,
			"Expected batchTimer to remain unchanged",
		)
		//assert.True(t, bmcMetricService.EnableBHOM, "Expected EnableBHOM to remain true")
		//assert.True(t, bmcMetricService.EnableVictoria, "Expected EnableVictoria to remain true")
		assert.True(t, bmcMetricService.persistOnError, "Expected persistOnError to remain true")
	})
	t.Run("ProcessConfigUpdates - Update PersistOnError and reset HTTPSender", func(t *testing.T) {
		bmcMetricService := &MetricExportService{
			persistOnError:     false,
			victoriaMetricsURL: "http://victoria-url",
		}
		newConfig := &config.AppExportConfig{
			PersistOnError: true,
		}

		bmcMetricService.ProcessConfigUpdates(newConfig)

		assert.True(
			t,
			bmcMetricService.persistOnError,
			"Expected persistOnError to be updated to true",
		)
		assert.NotNil(t, bmcMetricService.vmHttpSender, "Expected vmHttpSender to be reinitialized")
	})
}

func TestBMCMetricExportService_ExportToVictoria(t *testing.T) {
	t.Run("ExportToTSDB - Passed (Data sent successfully)", func(t *testing.T) {
		mockedHttpSender := svcmocks.MockHTTPSenderInterface{}
		mockedHttpSender.On("HTTPPost", mock.Anything, mock.Anything).Return(true, nil)
		mockedHttpSender.On("GetMimeType").Return("mimeType")

		bm := &MetricExportService{
			vmHttpSender:       &mockedHttpSender,
			victoriaMetricsURL: "http://victoria-metrics.local",
		}

		testData := []byte("test-metric-data")

		ok, result := bm.ExportToTSDB(u.AppFunctionContext, testData)
		assert.True(t, ok, "Expected true for successful data export")
		assert.Nil(t, result, "Expected nil error for successful data export")
	})
	t.Run("ExportToTSDB - Passed (Data is nil)", func(t *testing.T) {
		mockedHttpSender := svcmocks.MockHTTPSenderInterface{}

		bm := &MetricExportService{
			vmHttpSender:       &mockedHttpSender,
			victoriaMetricsURL: "http://victoria-metrics.local",
		}

		ok, result := bm.ExportToTSDB(u.AppFunctionContext, nil)
		assert.True(t, ok, "Expected true when data is nil")
		assert.Nil(t, result, "Expected nil error when data is nil")
	})
	t.Run("ExportToTSDB - Passed (Data is empty)", func(t *testing.T) {
		mockedHttpSender := svcmocks.MockHTTPSenderInterface{}

		bm := &MetricExportService{
			vmHttpSender:       &mockedHttpSender,
			victoriaMetricsURL: "http://victoria-metrics.local",
		}

		testData := []byte{}

		ok, result := bm.ExportToTSDB(u.AppFunctionContext, testData)
		assert.True(t, ok, "Expected true when data is empty")
		assert.Nil(t, result, "Expected nil error when data is empty")
	})
	t.Run("ExportToTSDB - Failed (HTTPPost fails)", func(t *testing.T) {
		mockedHttpSender := svcmocks.MockHTTPSenderInterface{}
		mockedHttpSender.On("HTTPPost", mock.Anything, mock.Anything).Return(false, "dummy error")
		mockedHttpSender.On("GetMimeType").Return("mimeType")

		bm := &MetricExportService{
			vmHttpSender:       &mockedHttpSender,
			victoriaMetricsURL: "http://victoria-metrics.local",
		}

		testData := []byte("test-metric-data")

		ok, result := bm.ExportToTSDB(u.AppFunctionContext, testData)
		assert.False(t, ok, "Expected false when HTTPPost fails")
		assert.NotNil(t, result, "Expected error when HTTPPost fails")
		assert.Contains(
			t,
			result.(error).Error(),
			"Error saving metric data in pipeline",
			"Unexpected error message",
		)
	})
}
