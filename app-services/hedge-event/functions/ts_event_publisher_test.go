package functions

import (
	"hedge/common/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestNewTSEventPublisher(t *testing.T) {
	t.Run("NewTSEventPublisher - Passed", func(t *testing.T) {
		eventPub := &TSEventPublisher{}
		got := NewTSEventPublisher()
		assert.Equal(t, eventPub, got, "Expected NewTSEventPublisher to return a new TSEventPublisher instance")
	})
}

func TestTSEventPublisher_BuildOTMetricEvent(t *testing.T) {
	ctx := u.AppFunctionContext
	ctx.On("SetResponseData", mock.Anything)
	ctx.On("SetResponseContentType", mock.Anything)

	t.Run("BuildOTMetricEventAsResponseData - Passed (Valid data)", func(t *testing.T) {
		bm := &TSEventPublisher{}
		testData := models.HedgeEvent{
			Id:             "f1c5f0e8-6b64-48ad-91b7-c5981b5ca3b9",
			Thresholds:     map[string]interface{}{"Threshold": float64(0)},
			ActualValues:   map[string]interface{}{"ActualValue": float64(0)},
			AdditionalData: map[string]string{"key": "value"},
			CorrelationId:  "123456",
			Remediations: []models.Remediation{
				{
					Id:      "123",
					Status:  "OK",
					Type:    "type",
					Summary: "summary",
				},
			},
		}

		got, gotData := bm.BuildOTMetricEventAsResponseData(ctx, testData)

		assert.True(t, got, "Expected BuildOTMetricEventAsResponseData to return true for valid data")
		assert.Equal(t, testData, gotData, "Expected returned data to match input data")
	})
	t.Run("BuildOTMetricEventAsResponseData - Passed (Additional valid data)", func(t *testing.T) {
		bm := &TSEventPublisher{}
		testData := models.HedgeEvent{
			Id:            "f1c5f0e8-6b64-48ad-91b7-c5981b5ca3b1",
			Status:        "Closed",
			IsNewEvent:    true,
			CorrelationId: "",
		}

		got, gotData := bm.BuildOTMetricEventAsResponseData(ctx, testData)

		assert.True(t, got, "Expected BuildOTMetricEventAsResponseData to return true for additional valid data")
		assert.Equal(t, testData, gotData, "Expected returned data to match input data")
	})
	t.Run("BuildOTMetricEventAsResponseData - Failed (nil data)", func(t *testing.T) {
		bm := &TSEventPublisher{}

		got, gotData := bm.BuildOTMetricEventAsResponseData(ctx, nil)

		assert.False(t, got, "Expected BuildOTMetricEventAsResponseData to return false for nil data")
		assert.Error(t, gotData.(error), "Expected an error for nil data")
		assert.Contains(t, gotData.(error).Error(), "no Metric data received", "Unexpected error message for nil data")
	})
	t.Run("BuildOTMetricEventAsResponseData - Failed (Invalid data type)", func(t *testing.T) {
		bm := &TSEventPublisher{}
		invalidData := []byte("wrong data")

		got, gotData := bm.BuildOTMetricEventAsResponseData(ctx, invalidData)

		assert.False(t, got, "Expected BuildOTMetricEventAsResponseData to return false for invalid data type")
		assert.Error(t, gotData.(error), "Expected an error for invalid data type")
		assert.Contains(t, gotData.(error).Error(), "invalid data type received, expected HedgeEvent", "Unexpected error message for invalid data type")
	})
}
