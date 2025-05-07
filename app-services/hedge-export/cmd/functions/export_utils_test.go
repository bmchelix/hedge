package functions

import (
	"hedge/common/dto"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBMCMetricExportService_ConvertToMetrics(t *testing.T) {
	t.Run("ConvertToMetrics - Passed (Valid Metrics Data)", func(t *testing.T) {
		testData, _ := json.Marshal(testMetricsData)

		bmcService := &MetricExportService{}

		result := bmcService.convertToMetrics(u.AppFunctionContext, testData)
		assert.NotNil(t, result, "Expected result to be non-nil for valid metrics data")
		assert.Equal(t, testMetricsData, result.(dto.Metrics), "Expected the converted metrics to match the input")
	})
	t.Run("ConvertToMetrics - Failed (Invalid data format)", func(t *testing.T) {
		invalidData := "invalid-json"

		bmcService := &MetricExportService{}

		result := bmcService.convertToMetrics(u.AppFunctionContext, invalidData)
		assert.Nil(t, result, "Expected result to be nil for invalid data format")
	})
	t.Run("ConvertToMetrics - Failed (Missing samples)", func(t *testing.T) {
		testMetrics := dto.Metrics{
			MetricGroup: dto.MetricGroup{},
		}
		testData, _ := json.Marshal(testMetrics)

		bmcService := &MetricExportService{}

		result := bmcService.convertToMetrics(u.AppFunctionContext, testData)
		assert.Nil(t, result, "Expected result to be nil for metrics data with missing samples")
	})
	t.Run("ConvertToMetrics - Failed (CoerceType error)", func(t *testing.T) {
		invalidData := make(chan int) // Invalid data type for coercion

		bmcService := &MetricExportService{}

		result := bmcService.convertToMetrics(u.AppFunctionContext, invalidData)
		assert.Nil(t, result, "Expected result to be nil for CoerceType error")
	})
}
