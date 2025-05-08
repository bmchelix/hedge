package helpers

import (
	"hedge/common/client"
	"hedge/common/telemetry"
	"hedge/edge-ml-service/pkg/dto/job"
	svcmocks "hedge/mocks/hedge/common/service"
	redisMock "hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces/mocks"
	"github.com/go-redsync/redsync/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"testing"
)

func TestNewTelemetry(t *testing.T) {
	mockHttpClient := svcmocks.MockHTTPClient{}
	mockHttpClient.On("Do", mock.Anything).Return(&http.Response{StatusCode: 200}, nil)
	client.Client = &mockHttpClient

	mockedMetricMngr := &mocks.MetricsManager{}
	mockedMetricMngr.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	metricCounters := map[string]int64{
		telemetry.CompletedJobsTime:   int64(10),
		telemetry.FailedJobsTime:      int64(10),
		telemetry.CanceledJobsTime:    int64(10),
		telemetry.TerminatedJobsTime:  int64(10),
		telemetry.CompletedJobsCount:  int64(2),
		telemetry.FailedJobsCount:     int64(2),
		telemetry.CanceledJobsCount:   int64(2),
		telemetry.TerminatedJobsCount: int64(2),
	}
	mockedDbClient := redisMock.MockMLDbInterface{}
	mockedDbClient.On("GetMetricCountersValues", mock.Anything).Return(metricCounters, nil)
	tel := NewTelemetry(u.AppService, "serviceName", mockedMetricMngr, &mockedDbClient)
	if tel == nil {
		t.Error("Expected a non-nil telemetry object, but got nil")
	}
}

func TestTelemetry_ProcessJob(t *testing.T) {
	mockHttpClient := svcmocks.MockHTTPClient{}
	mockHttpClient.On("Do", mock.Anything).Return(&http.Response{StatusCode: 200}, nil)
	client.Client = &mockHttpClient

	mockedMetricMngr := &mocks.MetricsManager{}
	mockedMetricMngr.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	mockedDbClient := redisMock.MockMLDbInterface{}
	mockedDbClient.On("GetMetricCounter", mock.Anything).Return(int64(0), nil)
	mockedDbClient.On("IncrMetricCounterBy", mock.Anything, mock.Anything).Return(int64(1), nil)
	mockedDbClient.On("AcquireRedisLock", mock.Anything).Return(&redsync.Mutex{}, nil)

	t.Run("ProcessJob - TrainingCompleted", func(t *testing.T) {
		tlmtry := NewTelemetry(u.AppService, "serviceName", mockedMetricMngr, &mockedDbClient)

		jobDetails := &job.TrainingJobDetails{
			StatusCode: job.TrainingCompleted,
			StartTime:  100,
			EndTime:    200,
		}

		_ = tlmtry.ProcessJob(jobDetails)
		expectedTrainingTime := int64((200 - 100) / 60)
		assert.Equal(t, expectedTrainingTime, tlmtry.TrainingTimeCompleted.Count(), "Expected training time completed mismatch")
		assert.Equal(t, int64(1), tlmtry.CompletedJobs.Count(), "Expected completed jobs count mismatch")
	})
	t.Run("ProcessJob - Failed", func(t *testing.T) {
		tlmtry := NewTelemetry(u.AppService, "serviceName", mockedMetricMngr, &mockedDbClient)

		jobDetails := &job.TrainingJobDetails{
			StatusCode: job.Failed,
			StartTime:  150,
			EndTime:    300,
		}

		_ = tlmtry.ProcessJob(jobDetails)
		expectedTrainingTime := int64((300 - 150) / 60)
		assert.Equal(t, expectedTrainingTime, tlmtry.TrainingTimeFailed.Count(), "Expected training time failed mismatch")
		assert.Equal(t, int64(1), tlmtry.FailedJobs.Count(), "Expected failed jobs count mismatch")
	})
	t.Run("ProcessJob - Canceled", func(t *testing.T) {
		tlmtry := NewTelemetry(u.AppService, "serviceName", mockedMetricMngr, &mockedDbClient)

		jobDetails := &job.TrainingJobDetails{
			StatusCode: job.Cancelled,
			StartTime:  200,
			EndTime:    400,
		}

		_ = tlmtry.ProcessJob(jobDetails)
		expectedTrainingTime := int64((400 - 200) / 60)
		assert.Equal(t, expectedTrainingTime, tlmtry.TrainingTimeCanceled.Count(), "Expected training time canceled mismatch")
		assert.Equal(t, int64(1), tlmtry.CanceledJobs.Count(), "Expected canceled jobs count mismatch")
	})
	t.Run("ProcessJob - Terminated", func(t *testing.T) {
		tlmtry := NewTelemetry(u.AppService, "serviceName", mockedMetricMngr, &mockedDbClient)

		jobDetails := &job.TrainingJobDetails{
			StatusCode: job.Terminated,
			StartTime:  250,
			EndTime:    500,
		}

		_ = tlmtry.ProcessJob(jobDetails)
		expectedTrainingTime := int64((500 - 250) / 60)
		assert.Equal(t, expectedTrainingTime, tlmtry.TrainingTimeTerminated.Count(), "Expected training time terminated mismatch")
		assert.Equal(t, int64(1), tlmtry.TerminatedJobs.Count(), "Expected terminated jobs count mismatch")
	})
}
