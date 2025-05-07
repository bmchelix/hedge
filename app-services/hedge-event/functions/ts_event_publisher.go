/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"encoding/json"
	"errors"

	"hedge/common/client"
	comModels "hedge/common/models"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
)

type TSEventPublisher struct {
}

func NewTSEventPublisher() *TSEventPublisher {
	tsEventHandler := new(TSEventPublisher)
	return tsEventHandler
}

func (bm *TSEventPublisher) BuildOTMetricEventAsResponseData(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	if data == nil {
		return false, errors.New("no Metric data received")
	}

	ctx.LoggingClient().Debug("Transforming HedgeEvent to Prometheus TS format")

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return false, errors.New("could not marshal data to bytes")
	}
	var bmcEvent comModels.HedgeEvent
	err = json.Unmarshal(jsonBytes, &bmcEvent)
	if err != nil {
		return false, errors.New("invalid data type received, expected HedgeEvent")
	}

	profile := bmcEvent.Profile
	if profile == "" {
		// Some dummy profile
		profile = "OT_EVENT_PROFILE"
	}
	edgexEvent := dtos.NewEvent(profile, bmcEvent.DeviceName, client.MetricEvent)
	var eventStatus int64
	if bmcEvent.Status != "Closed" {
		eventStatus = 1
	} else {
		eventStatus = 0
	}

	edgexEvent.AddSimpleReading(client.MetricEvent, common.ValueTypeInt64, eventStatus)
	// Add labels/ tags
	tags := make(map[string]interface{})
	tags[client.LabelEventSummary] = bmcEvent.Msg
	tags[client.LabelCorrelationId] = bmcEvent.CorrelationId
	tags[client.LabelEventType] = bmcEvent.EventType
	tags[client.LabelNodeName] = bmcEvent.SourceNode
	edgexEvent.Tags = tags
	// Set the edgexEvent in Response so this is published to BMCMetric towards end of the pipeline
	metricBytes, _ := util.CoerceType(edgexEvent)
	ctx.SetResponseData(metricBytes)
	ctx.SetResponseContentType("application/json")

	return true, data

}
