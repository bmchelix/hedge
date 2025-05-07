/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	redis "hedge/app-services/hedge-remediate/db"
	"hedge/common/db"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	common "hedge/common/telemetry"
	"fmt"
	sdkinterfaces "github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	gometrics "github.com/rcrowley/go-metrics"
	"math"
)

type Telemetry struct {
	commandsExecuted gometrics.Counter
	commandMessages  gometrics.Counter
	redisClient      redis.RemediateDBClientInterface
}

func NewTelemetry(service sdkinterfaces.ApplicationService, serviceName string, metricsManager interfaces.MetricsManager, hostName string, redisClient redis.RemediateDBClientInterface) *Telemetry {
	telemetry := Telemetry{}
	telemetry.commandsExecuted = gometrics.NewCounter()
	telemetry.commandMessages = gometrics.NewCounter()

	tags := make(map[string]string)
	tags["data_provider_service"] = serviceName
	tags["host"] = hostName

	metricsManager.Register(common.CommandMessageCount, telemetry.commandMessages, tags)
	metricsManager.Register(common.CommandsCount, telemetry.commandsExecuted, tags)

	telemetry.redisClient = redisClient
	return &telemetry
}

func (t *Telemetry) increment(command models.Command) hedgeErrors.HedgeError {
	// Create a distributed lock for the command data
	mutex, err := t.redisClient.AcquireRedisLock("command_lock")
	if err != nil {
		return err
	}
	defer mutex.Unlock()

	// fetch current values from Redis (synced with other service instances)
	err = t.fetchCurrentCountersValuesFromDb()
	if err != nil {
		return err
	}

	commandBody, exists := command.CommandParameters["body"]
	if !exists {
		errMsg := fmt.Sprint("commands parameters don't contain 'body'")
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errMsg)
	}
	bodyArray, ok := commandBody.([]interface{})
	if ok {
		size := len(bodyArray)
		var result int64
		count := float64(size) / float64(common.MaxMessageSize)
		if count < 1 { // e.g, if count == 0.3, result == 1
			result = 1
		} else {
			result = int64(math.Ceil(count)) // e.g, if count == 2.3, result == 3
		}

		return t.incrementCounters(result)
	}

	return nil
}

// incrementCounters increments the metrics counters in Redis and gometrics
func (t *Telemetry) incrementCounters(commandMessages int64) hedgeErrors.HedgeError {
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.CommandMessageCount, commandMessages); err != nil {
		return err
	}
	if _, err := t.redisClient.IncrMetricCounterBy(db.MetricCounter+":"+common.CommandsCount, 1); err != nil {
		return err
	}
	t.commandMessages.Inc(commandMessages)
	t.commandsExecuted.Inc(1)
	return nil
}

func (t *Telemetry) fetchCurrentCountersValuesFromDb() hedgeErrors.HedgeError {
	commMsgCount, err := t.redisClient.GetMetricCounter(db.MetricCounter + ":" + common.CommandMessageCount)
	if err != nil {
		return err
	}

	commandsCount, err := t.redisClient.GetMetricCounter(db.MetricCounter + ":" + common.CommandsCount)
	if err != nil {
		return err
	}

	t.commandMessages.Clear()
	t.commandMessages.Inc(commMsgCount)
	t.commandsExecuted.Clear()
	t.commandsExecuted.Inc(commandsCount)

	return nil
}
