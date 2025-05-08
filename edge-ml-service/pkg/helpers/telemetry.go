/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package helpers

import (
	commonConfig "hedge/common/config"
	"hedge/common/db"
	hedgeErrors "hedge/common/errors"
	common "hedge/common/telemetry"
	"hedge/edge-ml-service/pkg/db/redis"
	"hedge/edge-ml-service/pkg/dto/job"
	sdkinterfaces "github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	gometrics "github.com/rcrowley/go-metrics"
)

type Telemetry struct {
	TrainingTimeCompleted  gometrics.Counter
	TrainingTimeFailed     gometrics.Counter
	TrainingTimeCanceled   gometrics.Counter
	TrainingTimeTerminated gometrics.Counter
	CompletedJobs          gometrics.Counter
	FailedJobs             gometrics.Counter
	CanceledJobs           gometrics.Counter
	TerminatedJobs         gometrics.Counter
	redisClient            redis.MLDbInterface
}

func NewTelemetry(service sdkinterfaces.ApplicationService, serviceName string, metricsManager interfaces.MetricsManager, redisClient redis.MLDbInterface) *Telemetry {
	telemetry := Telemetry{}
	telemetry.TrainingTimeCompleted = gometrics.NewCounter()
	telemetry.TrainingTimeFailed = gometrics.NewCounter()
	telemetry.TrainingTimeCanceled = gometrics.NewCounter()
	telemetry.TrainingTimeTerminated = gometrics.NewCounter()
	telemetry.CompletedJobs = gometrics.NewCounter()
	telemetry.FailedJobs = gometrics.NewCounter()
	telemetry.CanceledJobs = gometrics.NewCounter()
	telemetry.TerminatedJobs = gometrics.NewCounter()

	/*	nodeName, err := service.GetAppSetting("EdgeNodeName")
		if err != nil {
			service.LoggingClient().Errorf("failed to retrieve EdgeNodeName from configuration: %s", err.Error())
		}*/
	_, hostName := commonConfig.GetCurrentNodeIdAndHost(service)
	if hostName == "" {
		service.LoggingClient().Errorf("Error getting HostName from hedge-admin, will continue..")
	}

	tags := make(map[string]string)
	tags["data_provider_service"] = serviceName
	tags["host"] = hostName

	metricsManager.Register(common.CompletedJobsTime, telemetry.TrainingTimeCompleted, tags)
	metricsManager.Register(common.FailedJobsTime, telemetry.TrainingTimeFailed, tags)
	metricsManager.Register(common.CanceledJobsTime, telemetry.TrainingTimeCanceled, tags)
	metricsManager.Register(common.TerminatedJobsTime, telemetry.TrainingTimeTerminated, tags)
	metricsManager.Register(common.CompletedJobsCount, telemetry.CompletedJobs, tags)
	metricsManager.Register(common.FailedJobsCount, telemetry.FailedJobs, tags)
	metricsManager.Register(common.CanceledJobsCount, telemetry.CanceledJobs, tags)
	metricsManager.Register(common.TerminatedJobsCount, telemetry.TerminatedJobs, tags)

	telemetry.redisClient = redisClient
	return &telemetry
}

func (t *Telemetry) ProcessJob(trainingJob *job.TrainingJobDetails) hedgeErrors.HedgeError {
	// Create a distributed lock for the training job data
	mutex, err := t.redisClient.AcquireRedisLock("training_data_lock")
	if err != nil {
		return err
	}
	defer mutex.Unlock()

	if trainingJob.StatusCode == job.TrainingCompleted {
		deltaInSecs := trainingJob.EndTime - trainingJob.StartTime

		// fetch current values from Redis (synced with other service instances)
		trainingTimeCompleted, err := t.fetchCurrentCounterValueFromDb(common.CompletedJobsTime)
		if err != nil {
			return err
		}
		completedJobs, err := t.fetchCurrentCounterValueFromDb(common.CompletedJobsCount)
		if err != nil {
			return err
		}
		t.TrainingTimeCompleted.Clear()
		t.TrainingTimeCompleted.Inc(trainingTimeCompleted)
		t.CompletedJobs.Clear()
		t.CompletedJobs.Inc(completedJobs)
		return t.incrementCompletedJobsCounters(deltaInSecs / 60)
	} else if trainingJob.StatusCode == job.Failed {
		deltaInSecs := trainingJob.EndTime - trainingJob.StartTime

		// fetch current values from Redis (synced with other service instances)
		trainingTimeFailed, err := t.fetchCurrentCounterValueFromDb(common.FailedJobsTime)
		if err != nil {
			return err
		}
		failedJobs, err := t.fetchCurrentCounterValueFromDb(common.FailedJobsCount)
		if err != nil {
			return err
		}
		t.TrainingTimeFailed.Clear()
		t.TrainingTimeFailed.Inc(trainingTimeFailed)
		t.FailedJobs.Clear()
		t.FailedJobs.Inc(failedJobs)
		return t.incrementFailedJobsCounters(deltaInSecs / 60)
	} else if trainingJob.StatusCode == job.Cancelled {
		deltaInSecs := trainingJob.EndTime - trainingJob.StartTime

		// fetch current values from Redis (synced with other service instances)
		trainingTimeCanceled, err := t.fetchCurrentCounterValueFromDb(common.CanceledJobsTime)
		if err != nil {
			return err
		}
		canceledJobs, err := t.fetchCurrentCounterValueFromDb(common.CanceledJobsCount)
		if err != nil {
			return err
		}
		t.TrainingTimeCanceled.Clear()
		t.TrainingTimeCanceled.Inc(trainingTimeCanceled)
		t.CanceledJobs.Clear()
		t.CanceledJobs.Inc(canceledJobs)
		return t.incrementCanceledJobsCounters(deltaInSecs / 60)
	} else if trainingJob.StatusCode == job.Terminated {
		deltaInSecs := trainingJob.EndTime - trainingJob.StartTime

		// fetch current values from Redis (synced with other service instances)
		trainingTimeTerminated, err := t.fetchCurrentCounterValueFromDb(common.TerminatedJobsTime)
		if err != nil {
			return err
		}
		terminatedJobs, err := t.fetchCurrentCounterValueFromDb(common.TerminatedJobsCount)
		if err != nil {
			return err
		}
		t.TrainingTimeTerminated.Clear()
		t.TrainingTimeTerminated.Inc(trainingTimeTerminated)
		t.TerminatedJobs.Clear()
		t.TerminatedJobs.Inc(terminatedJobs)
		return t.incrementTerminatedJobsCounters(deltaInSecs / 60)
	}

	return nil
}

// incrementCompletedJobsCounters increments the counters in Redis and gometrics
func (t *Telemetry) incrementCompletedJobsCounters(trainingTimeCompleted int64) hedgeErrors.HedgeError {
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.CompletedJobsTime, trainingTimeCompleted); err != nil {
		return err
	}
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.CompletedJobsCount, 1); err != nil {
		return err
	}
	t.TrainingTimeCompleted.Inc(trainingTimeCompleted)
	t.CompletedJobs.Inc(1)
	return nil
}

// incrementFailedJobsCounters increments the counters in Redis and gometrics
func (t *Telemetry) incrementFailedJobsCounters(trainingTimeFailed int64) hedgeErrors.HedgeError {
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.FailedJobsTime, trainingTimeFailed); err != nil {
		return err
	}
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.FailedJobsCount, 1); err != nil {
		return err
	}
	t.TrainingTimeFailed.Inc(trainingTimeFailed)
	t.FailedJobs.Inc(1)
	return nil
}

// incrementCanceledJobsCounters increments the counters in Redis and gometrics
func (t *Telemetry) incrementCanceledJobsCounters(trainingTimeCanceled int64) hedgeErrors.HedgeError {
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.CanceledJobsTime, trainingTimeCanceled); err != nil {
		return err
	}
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.CanceledJobsCount, 1); err != nil {
		return err
	}
	t.TrainingTimeCanceled.Inc(trainingTimeCanceled)
	t.CanceledJobs.Inc(1)
	return nil
}

// incrementTerminatedJobsCounters increments the counters in Redis and gometrics
func (t *Telemetry) incrementTerminatedJobsCounters(trainingTimeTerminated int64) hedgeErrors.HedgeError {
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.TerminatedJobsTime, trainingTimeTerminated); err != nil {
		return err
	}
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.TerminatedJobsCount, 1); err != nil {
		return err
	}
	t.TrainingTimeTerminated.Inc(trainingTimeTerminated)
	t.TerminatedJobs.Inc(1)
	return nil
}

// fetchCurrentCounterValueFromDb fetches current values from Redis (synced with other service instances)
func (t *Telemetry) fetchCurrentCounterValueFromDb(metricName string) (int64, hedgeErrors.HedgeError) {
	counterValue, err := t.redisClient.GetMetricCounter(db.MetricCounter + ":" + metricName)
	if err != nil {
		return 0, err
	}
	return counterValue, nil
}
