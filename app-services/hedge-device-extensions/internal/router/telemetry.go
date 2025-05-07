/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package router

import (
	"hedge/app-services/hedge-device-extensions/pkg/db/redis"
	commonConfig "hedge/common/config"
	"hedge/common/db"
	hedgeErrors "hedge/common/errors"
	common "hedge/common/telemetry"
	"encoding/json"
	"fmt"
	sdkinterfaces "github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	gometrics "github.com/rcrowley/go-metrics"
)

type Telemetry struct {
	contextualDataRequests gometrics.Counter
	contextualDataSize     gometrics.Counter
	redisClient            redis.DeviceExtDBClientInterface
}

func NewTelemetry(service sdkinterfaces.ApplicationService, serviceName string, metricsManager interfaces.MetricsManager, redisClient redis.DeviceExtDBClientInterface) *Telemetry {
	telemetry := Telemetry{}
	telemetry.contextualDataRequests = gometrics.NewCounter()
	telemetry.contextualDataSize = gometrics.NewCounter()

	_, hostName := commonConfig.GetCurrentNodeIdAndHost(service)
	if hostName == "" {
		service.LoggingClient().Error("Failed to get hostName from hedge-admin, check if hedge-admin is running")
	}
	tags := make(map[string]string)
	tags["data_provider_service"] = serviceName
	tags["host"] = hostName

	metricsManager.Register(common.ContextDataCallsCount, telemetry.contextualDataRequests, tags)
	metricsManager.Register(common.ContextDataSize, telemetry.contextualDataSize, tags)

	telemetry.redisClient = redisClient
	return &telemetry
}

func (t *Telemetry) ContextualDataRequest(existingContextualData map[string]interface{}, reqBody []byte) hedgeErrors.HedgeError {
	// Create a distributed lock for the contextual data
	mutex, err := t.redisClient.AcquireRedisLock("contextual_data_lock")
	if err != nil {
		return err
	}
	defer mutex.Unlock()

	// fetch current values from Redis (synced with other service instances)
	err = t.fetchCurrentCountersValuesFromDb()
	if err != nil {
		return err
	}

	if len(existingContextualData) == 0 {
		// No existing data, increase the metric by the size of the new data
		if err = t.incrementCounters(int64(len(reqBody))); err != nil {
			return err
		}
		return nil
	}

	var newContextualData map[string]interface{}
	err1 := json.Unmarshal(reqBody, &newContextualData)
	if err1 != nil {
		errMsg := fmt.Sprintf("error parsing request body: %v", err1.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}

	var newData []byte
	for key, newValue := range newContextualData {
		entry := map[string]interface{}{key: newValue}
		newKeyValueData, _ := json.Marshal(entry)

		if existingValue, exists := existingContextualData[key]; exists {
			// Check if the existing value is different from the new value.
			if existingValue != newValue {
				newData = append(newData, newKeyValueData...)
			}
		} else {
			// Key-value does not exist in existing data, so add its size.
			newData = append(newData, newKeyValueData...)
		}
	}
	if err = t.incrementCounters(int64(len(newData))); err != nil {
		return err
	}

	return nil
}

// incrementCounters increments the counters in Redis and gometrics
func (t *Telemetry) incrementCounters(sizeIncrement int64) hedgeErrors.HedgeError {
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.ContextDataSize, sizeIncrement); err != nil {
		return err
	}
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.ContextDataCallsCount, 1); err != nil {
		return err
	}

	// Increment the gometrics counters
	t.contextualDataSize.Inc(sizeIncrement)
	t.contextualDataRequests.Inc(1)

	return nil
}

func (t *Telemetry) fetchCurrentCountersValuesFromDb() hedgeErrors.HedgeError {
	currentCount, err := t.redisClient.GetMetricCounter(db.MetricCounter + ":" + common.ContextDataCallsCount)
	if err != nil {
		return err
	}

	currentSize, err := t.redisClient.GetMetricCounter(db.MetricCounter + ":" + common.ContextDataSize)
	if err != nil {
		return err
	}

	t.contextualDataRequests.Clear()
	t.contextualDataRequests.Inc(currentCount)
	t.contextualDataSize.Clear()
	t.contextualDataSize.Inc(currentSize)

	return nil
}
