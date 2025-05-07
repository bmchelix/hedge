/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"hedge/app-services/data-enrichment/cache"
	"hedge/common/client"
	cl "hedge/common/client"
	commonconfig "hedge/common/config"
	"hedge/common/db"
	"hedge/common/dto"
	comModels "hedge/common/models"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	metricinterfaces "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
)

// max concurrent redis publish command (to not crash)
const maxConcurrency = 5

type DataEnricher struct {
	service               interfaces.ApplicationService
	nodeName              string
	EventsPublisherURL    string
	redisClient           cl.DBClientInterface
	aggregateBuilder      *AggregateBuilder
	deviceToDeviceInfoMap map[string]comModels.DeviceInfo
	telemetry             *Telemetry
	cache                 cache.Manager
	sem                   chan bool
}

type TargetPayload struct {
	Metrics *dto.Metrics
	Events  []comModels.HedgeEvent
}

type PublishData struct {
	ReceivedTopic string `json:"receivedTopic" codec:"receivedTopic"`
	CorrelationID string `json:"correlationID" codec:"correlationID"`
	Payload       string `json:"payload"       codec:"payload"`
	ContentType   string `json:"contentType"   codec:"contentType"`
}

const (
	enrichedTopicPrefix = "enriched.events.device."
	AlertEventType      = "AlertEvent"
	EventMetricsKey     = "EventMetrics"
	UndefinedType       = "Undefined"
)

func NewDataEnricher(
	service interfaces.ApplicationService,
	metricsManager metricinterfaces.MetricsManager,
	cache cache.Manager,
	serviceName, nodeId string,
) (*DataEnricher, error) {
	dataEnricher := DataEnricher{}
	dataEnricher.service = service

	eventsPublisherUrls, err := service.GetAppSetting("EventsPublisherURL")
	if err != nil {
		service.LoggingClient().
			Errorf("failed to retrieve EventsPublisherURL from configuration: %s", err.Error())
		return nil, err
	}

	hostName, err := dataEnricher.getHostName()
	if err != nil {
		service.LoggingClient().Errorf("failed to get hostName: %s", err.Error())
		return nil, err
	}

	var metricsBatchSize int64
	metricsBatchSizeStr, err := service.GetAppSetting("MetricsBatchSize")
	if err == nil {
		if metricsBatchSize, err = strconv.ParseInt(metricsBatchSizeStr, 10, 64); err == nil {
			service.LoggingClient().Infof("metricsBatchSize=%v", metricsBatchSize)
		}
	} else {
		service.LoggingClient().Errorf("failed to retrieve MetricsBatchSize from configuration: %s", err.Error())
		return nil, err
	}

	dataEnricher.nodeName = hostName
	dataEnricher.EventsPublisherURL = eventsPublisherUrls + "/api/v3/trigger"
	// limit concurrent execution
	dataEnricher.sem = make(chan bool, maxConcurrency)

	// Redis connection
	redisConfig := db.NewDatabaseConfig()
	redisConfig.LoadAppConfigurations(service)
	dataEnricher.redisClient = cl.NewDBClient(redisConfig)
	dataEnricher.telemetry, err = NewTelemetry(
		service,
		serviceName,
		metricsManager,
		hostName,
		dataEnricher.redisClient,
		metricsBatchSize,
	)
	if dataEnricher.telemetry == nil {
		service.LoggingClient().Errorf("failed to create Telemetry instance: %s", err.Error())
		return nil, err
	}

	dataEnricher.cache = cache
	dataEnricher.aggregateBuilder = NewAggregateBuilder(service.LoggingClient(), cache, nodeId)

	return &dataEnricher, nil
}

// EnrichData enriches with profile, biz data and more from Redis and passes the transformed data to next function in the pipeline
func (s *DataEnricher) EnrichData(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {
	lc := ctx.LoggingClient()
	if data == nil {
		return false, fmt.Errorf(
			"function EnrichData in pipeline '%s': No Data Received",
			ctx.PipelineId(),
		)
	}

	event, ok := data.(dtos.Event)
	if !ok {
		return false, fmt.Errorf(
			"function EnrichData in pipeline '%s': type received is not an Event",
			ctx.PipelineId(),
		)
	}

	lc.Debugf("received event enriching '%s' with tags ...", event.DeviceName)

	deviceTags, _ := s.cache.FetchDeviceTags(event.DeviceName)
	profileTags, found := s.cache.FetchProfileTags(event.ProfileName)
	if !found {
		return false, nil
	}

	if deviceTags != nil {
		event.Tags = s.MergeMaps(event.Tags, deviceTags.(map[string]interface{}), s.nodeName)
	}
	if profileTags != nil {
		event.Tags = s.MergeMaps(event.Tags, profileTags.(map[string]interface{}), s.nodeName)
	}

	// tags cannot be empty, it will at least have model or manufacturer from profile
	if len(event.Tags) == 0 {
		return false, nil
	}

	return true, event
}

// PublishEnrichedDataInternally sends back the data to the MessageBus
func (s *DataEnricher) PublishEnrichedDataInternally(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {
	// var publishData PublishData
	if data == nil {
		return false, fmt.Errorf(
			"function PublishEnrichedDataInternally in pipeline '%s': No Data Received",
			ctx.PipelineId(),
		)
	}

	event := data.(dtos.Event)
	var channel = enrichedTopicPrefix + event.ProfileName + "." + event.DeviceName + "." + event.SourceName
	channel = commonconfig.BuildTopicNameFromBaseTopicPrefix(channel, ".")
	ctx.LoggingClient().Debugf("Publishing to redis: %v", data)
	status, result := s.publish(ctx, data, channel, "redis")
	if !status {
		// Don't call next pipeline
		return false, result
	}
	return true, event
}

// TransformToAggregatesAndEvents Break down the metric data between events and aggregate metrics, returns events and aggregates.
// Metrics are returned only when the sampling interval is elapsed
func (s *DataEnricher) TransformToAggregatesAndEvents(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {
	if data == nil {
		return false, errors.New("empty event")
	}
	lc := ctx.LoggingClient()
	event, ok := data.(dtos.Event)
	if !ok {
		lc.Errorf("invalid event format, failed when casting to dtos.Event")
		return false, errors.New("invalid metric event format")
	}
	lc.Debugf("input: %v", event)
	targetPayload := TargetPayload{
		Metrics: nil,
		Events:  nil,
	}

	metricEvent := event
	metricEvent.Readings = []dtos.BaseReading{}

	eventMetricsAsCommaDelimitedTags, eventFoundInMetricData := event.Tags[EventMetricsKey].(string)
	var eventMetrics []string
	if eventFoundInMetricData {
		eventMetrics = strings.Split(eventMetricsAsCommaDelimitedTags, ",")
		delete(metricEvent.Tags, EventMetricsKey)
	}

	for _, reading := range event.Readings {
		lc.Debugf("reading data:%v\n", reading)
		err := s.telemetry.IncomingMessage(reading)
		if err != nil {
			lc.Errorf("Error while calculating message metrics counters, error: %v", err)
			return false, errors.New(
				"error while calculating message metrics counters, error: " + err.Error(),
			)
		}

		var contained bool
		for _, m := range eventMetrics {
			if m == reading.ResourceName {
				contained = true
			}
		}

		if eventFoundInMetricData && contained {
			// if EventMetric list contains reading resource name, addToAggregate reading to Elastic Output
			bmcEvent := s.buildEventFromMetric(reading, event)
			if targetPayload.Events == nil {
				targetPayload.Events = make([]comModels.HedgeEvent, 0)
			}
			targetPayload.Events = append(targetPayload.Events, *bmcEvent)
		} else {
			metricEvent.Readings = append(metricEvent.Readings, reading)
		}
	} // Iteration over Readings over

	if len(metricEvent.Readings) > 0 {
		// aggregation builder, this will under mutex so blocking call per device instance
		ok, aggregateMetrics := s.aggregateBuilder.BuildAggregates(metricEvent)
		if ok {
			targetPayload.Metrics = aggregateMetrics
		}
	}

	if targetPayload.Events == nil && targetPayload.Metrics == nil {
		return false, nil
	}

	return true, targetPayload
}

// SendToEventPublisher Sends Events to Event-Publisher
func (s *DataEnricher) SendToEventPublisher(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {
	lc := ctx.LoggingClient()

	targetPayload, ok := data.(TargetPayload)
	if !ok {
		// retObj.Payload = errors.New("Invalid Event received. Failed when casting to dtos.Event")
		lc.Errorf("invalid TargetPayload received. Failed when casting to TargetPayload")
		return false, errors.New("invalid TargetPayload received")
	}

	if targetPayload.Events != nil {
		lc.Debug("Sending data to Event Publisher")
		for _, bmcEvent := range targetPayload.Events {
			byteArray, err := json.Marshal(bmcEvent)
			if err != nil {
				lc.Errorf("failed marshalling the event data. Error: %v, Event: %v", err, bmcEvent)
				continue // Continue with next object
			}
			ctx.LoggingClient().Debugf("posting to event publish data: %v", bmcEvent)
			// Use http interface in here
			payload := strings.NewReader(string(byteArray))
			req, err := http.NewRequest(http.MethodPost, s.EventsPublisherURL, payload)
			if err != nil {
				return false, err
			}
			req.Header.Set("Content-Type", "application/json")
			response, err := client.Client.Do(req)
			if err != nil {
				lc.Errorf(
					"http call to publish event to event publisher failed, event data. Error: %v, Event: %v",
					err,
					bmcEvent,
				)
			} else if response != nil && (response.StatusCode >= 203) {
				lc.Errorf("response from publishing to event_publisher: httpStatus: %d, response: %v", response.StatusCode, response)
				// Continue with next object even though error
			}
		}
	}

	if targetPayload.Metrics != nil {
		return true, targetPayload.Metrics
	}
	return false, nil
}

func (s *DataEnricher) MergeMaps(
	exMap map[string]interface{},
	newMap map[string]interface{},
	nodeId string,
) map[string]interface{} {
	if exMap == nil {
		exMap = make(map[string]interface{})
	}
	for key, newVal := range newMap {
		exVal2 := newVal
		if exVal, exists := exMap[key]; exists {
			if key == "devicelabels" {
				// concat device labels
				exVal2 = exVal.(string) + "," + newMap[key].(string)
			}
		} else {
			exMap[key] = exVal2
		}

	}
	// Add host name if not already added
	if _, ok := exMap["host"]; !ok {
		exMap["host"] = nodeId
	}
	return exMap
}

func (s *DataEnricher) buildEventFromMetric(
	reading dtos.BaseReading,
	event dtos.Event,
) *comModels.HedgeEvent {
	// build HedgeEvent
	s.service.LoggingClient().Debugf("metricType is event for metric : %s\n", reading.ResourceName)
	bmcEvent := new(comModels.HedgeEvent)
	bmcEvent.Id = reading.Id
	bmcEvent.Name = reading.ResourceName
	bmcEvent.EventSource = "DeviceService"

	bmcEvent.DeviceName = reading.DeviceName
	bmcEvent.Profile = reading.ProfileName
	bmcEvent.Class = comModels.BASE_EVENT_CLASS
	bmcEvent.EventType = "EVENT"

	// Do this based on type
	if reading.ValueType == common.ValueTypeFloat64 {
		actualValue, _ := strconv.ParseFloat(reading.Value, 64)
		actual := make(map[string]interface{})
		actual[reading.ResourceName] = actualValue
		bmcEvent.ActualValues = actual
		bmcEvent.ActualValueStr = fmt.Sprintf("%v", bmcEvent.ActualValues)
	} else {
		// TODO for other types
		// Need to be able to handle the faultCode which is a string type
		bmcEvent.ActualValueStr = fmt.Sprintf("%v", reading.Value)
		actualValue, _ := strconv.ParseFloat(bmcEvent.ActualValueStr, 64)
		actual := make(map[string]interface{})
		actual[reading.ResourceName] = actualValue
		bmcEvent.ActualValues = actual
	}

	// Have one correlation Id per device+metric so we don't create duplicates
	bmcEvent.CorrelationId = reading.DeviceName + ":" + reading.ResourceName
	// Need to work out how since we can't decide whether it is open or closed event
	// For now, assume the events are closed manually there is no closed loop closure
	bmcEvent.Status = "Open"
	bmcEvent.Labels = make([]string, 1)
	bmcEvent.Labels[0] = AlertEventType
	bmcEvent.Created = reading.Origin / 1000000 // Convert to milli-seconds for Elastic DB
	bmcEvent.RelatedMetrics = []string{event.SourceName}
	// Add some defaults that can be overridden by tags being passed from device service
	// Now if some values are not populated, addToAggregate some defaults, Expect Closed status will be passed by device service to close this event
	bmcEvent.Msg = reading.DeviceName + " raised an event with metric " + event.SourceName + ":" + bmcEvent.ActualValueStr
	bmcEvent.Status = "Open"
	bmcEvent.Severity = comModels.SEVERITY_MAJOR
	if event.Tags != nil {
		bmcEvent.AdditionalData = make(map[string]string, 0)
		for key, value := range event.Tags {
			strVal := fmt.Sprintf("%v", value)
			switch key {
			case "msg", "message", "description":
				bmcEvent.Msg = strVal
			case "severity":
				bmcEvent.Severity = strVal
			case "status":
				bmcEvent.Status = strVal // we rely on decision sent out to open or close an event
			default:
				bmcEvent.AdditionalData[key] = strVal
			}
		}
	}
	return bmcEvent
}

func (s *DataEnricher) publish(
	ctx interfaces.AppFunctionContext,
	data interface{},
	channel string,
	destination string,
) (bool, interface{}) {
	if destination == "redis" {
		ctx.LoggingClient().Debugf("Publishing to Redis data: %v\n", data)
		var publishData PublishData
		bizD, err := json.Marshal(data)
		if err != nil {
			return false, err
		}
		publishData.ReceivedTopic, _ = ctx.GetValue("receivedtopic")
		publishData.CorrelationID = ctx.CorrelationID()
		publishData.ContentType = "application/json"
		publishData.Payload = base64.StdEncoding.EncodeToString(bizD)
		pd, err := json.Marshal(publishData)
		if err != nil {
			return false, err
		}

		// limit concurrent publish operations
		s.sem <- true
		defer func() { <-s.sem }()

		err = s.redisClient.PublishToRedisBus(channel, pd)
		if err != nil {
			fmt.Printf("error publishing to redis topic, error:'%v', data: %v", err, pd)
			return false, err
		}
	}

	return true, nil
}

func (s *DataEnricher) getHostName() (string, error) {
	lc := s.service.LoggingClient()
	_, hostName := commonconfig.GetCurrentNodeIdAndHost(s.service)
	if hostName == "" {
		lc.Errorf(
			"error getting node/hostName from hedge-admin, will check EdgeNodeName from config",
		)
		var err error
		hostName, err = s.service.GetAppSetting("EdgeNodeName")
		if err != nil {
			lc.Errorf(
				"failed to retrieve EdgeNodeName from configuration: %s, exiting",
				err.Error(),
			)
			return "", err
		}
	}
	lc.Infof("hostName = %s", hostName)
	return hostName, nil
}
