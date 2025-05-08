package functions

import (
	"hedge/common/models"
	clientmocks "hedge/mocks/hedge/common/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

/*func TestNewElasticExporter(t *testing.T) {
	elasticEventExporter := NewElasticExporter(u.AppService)
	assert.Nil(t, elasticEventExporter)
}*/

func TestElasticExporter_SaveEventToElastic(t *testing.T) {
	t.Run("SaveEventToElastic - Passed (Index successful)", func(t *testing.T) {
		mockedElasticClient := clientmocks.MockElasticClientInterface{}
		mockedElasticClient.On("IndexEvent", mock.Anything).Return(nil)

		elasticEvent := ElasticExporter{
			elasticClient: &mockedElasticClient,
			lc:            u.AppService.LoggingClient(),
		}

		ok, gotData := elasticEvent.SaveEventToElastic(u.AppFunctionContext, testEventData)
		assert.True(t, ok, "Expected SaveEventToElastic to return true for valid data")
		assert.Equal(t, testEventData, gotData, "Expected returned data to match input")
	})
	t.Run("SaveEventToElastic - Passed (Search successful)", func(t *testing.T) {
		mockedElasticClient := clientmocks.MockElasticClientInterface{}
		mockedElasticClient.On("SearchEvents", mock.Anything).Return([]*models.HedgeEvent{&testEventData1}, nil)
		mockedElasticClient.On("IndexEvent", mock.Anything).Return(nil)

		elasticEvent := ElasticExporter{
			elasticClient: &mockedElasticClient,
			lc:            u.AppService.LoggingClient(),
		}

		ok, gotData := elasticEvent.SaveEventToElastic(u.AppFunctionContext, testEventData1)
		assert.True(t, ok, "Expected SaveEventToElastic to return true for search success")
		assert.Equal(t, testEventData1, gotData, "Expected returned data to match input")
	})
	t.Run("SaveEventToElastic - Failed (Search failed)", func(t *testing.T) {
		mockedElasticClient := clientmocks.MockElasticClientInterface{}
		mockedElasticClient.On("SearchEvents", mock.Anything).Return(nil, testError)

		elasticEvent := ElasticExporter{
			elasticClient: &mockedElasticClient,
			lc:            u.AppService.LoggingClient(),
		}

		ok, gotData := elasticEvent.SaveEventToElastic(u.AppFunctionContext, testEventData1)
		assert.False(t, ok, "Expected SaveEventToElastic to return false for nil data")
		assert.Error(t, gotData.(error), "Expected an error for nil data")
		assert.Contains(t, gotData.(error).Error(), "error while searching Open event with correlationId: : "+testError.Error(), "Unexpected error message for nil data")
	})
	t.Run("SaveEventToElastic - Failed (Nil data)", func(t *testing.T) {
		mockedElasticClient := clientmocks.MockElasticClientInterface{}

		elasticEvent := ElasticExporter{
			elasticClient: &mockedElasticClient,
			lc:            u.AppService.LoggingClient(),
		}

		ok, gotData := elasticEvent.SaveEventToElastic(u.AppFunctionContext, nil)
		assert.False(t, ok, "Expected SaveEventToElastic to return false for nil data")
		assert.Error(t, gotData.(error), "Expected an error for nil data")
		assert.Contains(t, gotData.(error).Error(), "no Data Received", "Unexpected error message for nil data")
	})
	t.Run("SaveEventToElastic - Failed (Invalid data)", func(t *testing.T) {
		mockedElasticClient := clientmocks.MockElasticClientInterface{}

		elasticEvent := ElasticExporter{
			elasticClient: &mockedElasticClient,
			lc:            u.AppService.LoggingClient(),
		}

		invalidData := []byte("wrong data")

		ok, gotData := elasticEvent.SaveEventToElastic(u.AppFunctionContext, invalidData)
		assert.False(t, ok, "Expected SaveEventToElastic to return false for invalid data type")
		assert.Error(t, gotData.(error), "Expected an error for invalid data type")
		assert.Contains(t, gotData.(error).Error(), "error while unmarshalling data", "Unexpected error message for invalid data type")
	})
	t.Run("SaveEventToElastic - Failed (Index failed)", func(t *testing.T) {
		mockedElasticClient := clientmocks.MockElasticClientInterface{}
		mockedElasticClient.On("IndexEvent", mock.Anything).Return(testError)

		elasticEvent := ElasticExporter{
			elasticClient: &mockedElasticClient,
			lc:            u.AppService.LoggingClient(),
		}

		ok, gotData := elasticEvent.SaveEventToElastic(u.AppFunctionContext, testEventData)
		assert.False(t, ok, "Expected SaveEventToElastic to return false for index failure")
		assert.Error(t, gotData.(error), "Expected an error for index failure")
		assert.Contains(t, gotData.(error).Error(), "dummy error", "Unexpected error message for index failure")
	})
	t.Run("SaveEventToElastic - Failed (Unmarshalling error)", func(t *testing.T) {
		mockedElasticClient := clientmocks.MockElasticClientInterface{}
		mockedElasticClient.On("IndexEvent", mock.Anything).Return(testError)

		elasticEvent := ElasticExporter{
			elasticClient: &mockedElasticClient,
			lc:            u.AppService.LoggingClient(),
		}

		testData := "valid data"

		ok, gotData := elasticEvent.SaveEventToElastic(u.AppFunctionContext, testData)
		assert.False(t, ok, "Expected SaveEventToElastic to return false for index failure")
		assert.Error(t, gotData.(error), "Expected an error for index failure")
		assert.Contains(t, gotData.(error).Error(), "error while unmarshalling data", "Unexpected error message for index failure")
	})
	t.Run("SaveMLPredictionToElastic - Failed (Marshalling error)", func(t *testing.T) {
		mockedElasticClient := &clientmocks.MockElasticClientInterface{}

		elasticEvent := ElasticExporter{
			elasticClient: mockedElasticClient,
			lc:            u.AppService.LoggingClient(),
		}

		testData := make(chan int) // Invalid data type for marshalling

		ok, gotData := elasticEvent.SaveEventToElastic(u.AppFunctionContext, testData)
		assert.False(t, ok, "Expected SaveEventToElastic to return false for index failure")
		assert.Error(t, gotData.(error), "Expected an error for index failure")
		assert.Contains(t, gotData.(error).Error(), "error while marshalling data", "Unexpected error message for index failure")
	})
}
