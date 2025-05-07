/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"hedge/common/client"
	"hedge/common/db"
	hedgeErrors "hedge/common/errors"
	common "hedge/common/telemetry"
	sdkinterfaces "github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	comm "github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"math"
	"sync"

	gometrics "github.com/rcrowley/go-metrics"
)

type Telemetry struct {
	inMessages        gometrics.Counter // Should be counter
	inRawMetrics      gometrics.Counter
	redisClient       client.DBClientInterface
	metricsBatchSize  int64
	currentBatchCount int64
	mutex             sync.Mutex
}

func NewTelemetry(service sdkinterfaces.ApplicationService, serviceName string, metricsManager interfaces.MetricsManager, hostName string, redisClient client.DBClientInterface, batchSize int64) (*Telemetry, hedgeErrors.HedgeError) {
	telemetry := Telemetry{}
	telemetry.inMessages = gometrics.NewCounter()
	telemetry.inRawMetrics = gometrics.NewCounter()

	telemetry.redisClient = redisClient
	err := telemetry.fetchCurrentCountersValuesFromDb()
	if err != nil {
		// If call to db failed - we should stop the service - otherwise the metrics counting will start from 0 and affect the metrics aggregation
		service.LoggingClient().Errorf(err.Error())
		return nil, err
	}
	service.LoggingClient().Infof("metrics counters retrieved from db at the service start: %s-%v, %s-%v",
		common.MetricMessageCount, telemetry.inMessages.Count(), common.MetricsCount, telemetry.inRawMetrics.Count())

	tags := make(map[string]string)
	tags["data_provider_service"] = serviceName
	tags["host"] = hostName

	metricsManager.Register(common.MetricMessageCount, telemetry.inMessages, tags)
	metricsManager.Register(common.MetricsCount, telemetry.inRawMetrics, tags)

	telemetry.metricsBatchSize = batchSize
	telemetry.currentBatchCount = 0
	return &telemetry, nil
}

func (t *Telemetry) IncomingMessage(reading dtos.BaseReading) hedgeErrors.HedgeError {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// The 'inMessages' metric counter takes into account the device name and metric name,
	// each of which is limited to a maximum of 25 bytes (device name) and 35 (metric name).
	// As a result, if the ValueType is not string/arrays - each message does not exceed the maximum size of 100 bytes.
	// TODO: add size calculation for arrays once supported
	size := common.MaxDeviceNameAndMetricNameSize
	if reading.ValueType == comm.ValueTypeString {
		size += len(reading.Value)
		var result int64
		count := float64(size) / float64(common.MaxMessageSize)
		if count < 1 { // e.g, if count == 0.3, result == 1
			result = 1
		} else {
			result = int64(math.Ceil(count)) // e.g, if count == 2.3, result == 3
		}
		t.inMessages.Inc(result)
	} else {
		t.inMessages.Inc(1)
	}
	t.inRawMetrics.Inc(1)
	t.currentBatchCount++

	// Check if the batch size threshold is met
	if t.currentBatchCount >= t.metricsBatchSize {
		err := t.updateCountersInDb()
		if err != nil {
			return err
		}
		t.currentBatchCount = 0 // reset batch counter
	}
	return nil
}

// incrementCounters increments the metrics counters in Redis
func (t *Telemetry) updateCountersInDb() hedgeErrors.HedgeError {
	if err := t.redisClient.SetMetricCounter(db.MetricCounter+":"+common.MetricMessageCount, t.inMessages.Count()); err != nil {
		return err
	}
	if err := t.redisClient.SetMetricCounter(db.MetricCounter+":"+common.MetricsCount, t.inRawMetrics.Count()); err != nil {
		return err
	}
	return nil
}

func (t *Telemetry) fetchCurrentCountersValuesFromDb() hedgeErrors.HedgeError {
	metricMessageCount, err := t.redisClient.GetMetricCounter(db.MetricCounter + ":" + common.MetricMessageCount)
	if err != nil {
		return err
	}

	metricsCount, err := t.redisClient.GetMetricCounter(db.MetricCounter + ":" + common.MetricsCount)
	if err != nil {
		return err
	}

	t.inMessages.Clear()
	t.inMessages.Inc(metricMessageCount)
	t.inRawMetrics.Clear()
	t.inRawMetrics.Inc(metricsCount)

	return nil
}
