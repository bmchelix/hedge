/*******************************************************************************
 * Copyright 2022 Intel Corp.
 * (c) Copyright 2020-2025 BMC Software, Inc.
 *
 * Contributors: BMC Software, Inc. - BMC Helix Edge
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package telemetry

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"hedge/common/dto"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"

	"hedge/common/config"
	sdkinterfaces "github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	gometrics "github.com/rcrowley/go-metrics"
)

type MQTTMetricReporter struct {
	service           sdkinterfaces.ApplicationService
	serviceName       string
	topic             string
	tags              map[string]string
	mqttSender        *transforms.MQTTSecretSender
	mu                sync.Mutex
	lastReportedValue map[string]int64
}

// NewMessageBusReporter creates a new MessageBus reporter which reports metrics to the EdgeX MessageBus
func NewMQTTMetricReporter(
	service sdkinterfaces.ApplicationService,
	baseTopic string,
	serviceName string,
	tags map[string]string,
) interfaces.MetricsReporter {
	lc := service.LoggingClient()
	topic := baseTopic + "/" + serviceName
	// need to have different clientIds for multiple instances in a pod in case of subscribers, this may not be the case for subscription, still safe to use diff client Ids
	clientId := fmt.Sprintf("%s_%d", serviceName, rand.Int())
	mqttConfig, err := config.BuildMQTTSecretConfig(service, topic, clientId)
	if err != nil {
		lc.Errorf("failed to create MQTT configuration: %s", err.Error())
		return nil
	}

	// Girish: Disabled store and forward for telemetry, not sure how it can work
	// MQTTSender automatically has the code to store retry data in context(appContext). thereafter pipeline function needs to return false
	// so the context data is persisted. Not sure if the pipeline is in place for telemetry
	/*	var persistOnError bool
		if strings.Contains(serviceName, "data-enrichment") {
			mqttConfig.QoS = 0
			persistOnError = false
		} else {
			persistOnError = config.GetPersistOnError(service)
		}*/
	reporter := &MQTTMetricReporter{
		tags:        tags,
		service:     service,
		serviceName: serviceName,
		topic:       topic,
		//mqttSender:  transforms.NewMQTTSecretSender(mqttConfig, persistOnError),
		mqttSender:        transforms.NewMQTTSecretSender(mqttConfig, false),
		lastReportedValue: make(map[string]int64),
	}
	return reporter
}

func (r *MQTTMetricReporter) Report(
	registry gometrics.Registry,
	metricTags map[string]map[string]string,
) error {
	var errs error
	publishedCount := 0

	lc := r.service.LoggingClient()

	if r.mqttSender == nil {
		return errors.New("mqtt client not available. Unable to report metrics")
	}

	metricGroup := dto.MetricGroup{
		Tags:    make(map[string]interface{}),
		Samples: make([]dto.Data, 0),
	}

	/*	event := dtos.NewEvent(r.serviceName, r.serviceName, r.serviceName)
		event.AddSimpleReading()*/

	if r.tags != nil {
		for key, value := range r.tags {
			metricGroup.Tags[key] = value
		}
	}
	registry.Each(func(name string, item interface{}) {
		if !strings.HasPrefix(name, "hb_") {
			return
		}
		var value int64
		switch metric := item.(type) {
		case gometrics.Counter:
			value = metric.Count()
			if value == 0 {
				return
			}
			if value >= (math.MaxInt64 - 1000) { // Near overflow threshold
				r.service.LoggingClient().Warnf("Resetting counter '%s' with value: %d to avoid overflow as it has almost reached the maximum int64 value", name, value)
				metric.Clear()
			}
		case gometrics.Gauge:
			value = metric.Value()
			metric.Update(0)
		default:
			errs = multierror.Append(errs, fmt.Errorf("metric type %T not supported", metric))
			return
		}

		// Check if the metric value has changed
		r.mu.Lock()
		if lastValue, exists := r.lastReportedValue[name]; !exists || lastValue != value {
			metricGroup.Samples = append(metricGroup.Samples, dto.Data{
				Name:      name,
				TimeStamp: time.Now().UnixNano(),
				Value:     strconv.FormatInt(value, 10),
				ValueType: common.ValueTypeInt64,
			})
			r.lastReportedValue[name] = value // update last reported value
			publishedCount++
		}
		r.mu.Unlock()

		// The tags are same for all telemetry metrics of a service
		if len(metricGroup.Tags) == 0 {
			for key, value := range metricTags[name] {
				metricGroup.Tags[key] = value
			}
		}

	})

	metrics := dto.Metrics{
		IsCompressed: false,
		MetricGroup:  metricGroup,
	}

	if publishedCount > 0 {
		ok, err1 := r.mqttSender.MQTTSend(r.service.BuildContext(uuid.NewString(), "Test"), metrics)
		if !ok && err1 != nil {
			lc.Errorf("Error publishing telemetry data to MQTT: %v", err1)
			errs = multierror.Append(
				errs,
				fmt.Errorf(
					"failed to publish telemetry event '%v' to topic '%s', \n\terror: %v",
					metrics,
					r.topic,
					err1,
				),
			)
		} else {
			lc.Debugf("Publish %d telemetry metrics to the '%s' base topic. Data %v", publishedCount, r.topic, metrics)
		}
	} else {
		lc.Debugf("No telemetry metrics to publish.")
	}

	return errs
}
