/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	redis "hedge/app-services/hedge-event-publisher/pkg/db"
	"hedge/common/db"
	"hedge/common/models"
	redis2 "hedge/mocks/hedge/app-services/hedge-event-publisher/pkg/db"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces/mocks"
	mocks2 "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
	"time"
)

var dbConfig *db.DatabaseConfig
var dBMockClient redis2.MockDBClient
var event models.HedgeEvent
var mockLogger *mocks2.LoggingClient
var ctx *mocks.AppFunctionContext

func init() {
	dbConfig = &db.DatabaseConfig{
		RedisHost:          "edgex-redis",
		RedisPort:          "6349",
		RedisName:          "",
		RedisUsername:      "default",
		RedisPassword:      "bn784738bnxpassword",
		VictoriaMetricsURL: "",
	}
	dBMockClient = redis2.MockDBClient{}
	redis.DBClientImpl = &dBMockClient
	dBMockClient.On("GetDbClient", dbConfig).Return(redis.DBClientImpl)
	event = models.HedgeEvent{
		Id:               "",
		Class:            models.BASE_EVENT_CLASS,
		EventType:        "EVENT",
		DeviceName:       "AltaNS12",
		Name:             "HighTemperatureEvent",
		Msg:              "Temperatue above 90",
		Severity:         models.SEVERITY_MAJOR,
		Priority:         "High",
		Profile:          "WindTurbine",
		SourceNode:       "clm-pun-uojx84",
		Status:           "Open",
		RelatedMetrics:   []string{"Temperature"},
		Thresholds:       map[string]interface{}{"Threshold": 90},
		ActualValues:     map[string]interface{}{"ActualValue": 100},
		ActualValueStr:   "100",
		Unit:             "degree C",
		Location:         "Pune",
		Version:          0,
		AdditionalData:   nil,
		EventSource:      "",
		CorrelationId:    "hightempRule_AltaNS12",
		Labels:           nil,
		Remediations:     nil,
		Created:          0,
		Modified:         0,
		IsNewEvent:       true,
		IsRemediationTxn: false,
	}

	mockLogger = &mocks2.LoggingClient{}
	mockLogger.On("Debugf", mock.Anything, mock.Anything).Return(mock.Anything)
	mockLogger.On("Infof", mock.Anything, mock.Anything).Return(mock.Anything)
	mockLogger.On("Errorf", mock.Anything, mock.Anything).Return(mock.Anything)
	mockLogger.On("Errorf", mock.Anything, mock.Anything, mock.Anything).Return()

	ctx = &mocks.AppFunctionContext{}
	ctx.On("LoggingClient").Return(mockLogger)
}

func TestNewEdgeEventCoordinator(t *testing.T) {
	type args struct {
		dbConfig *db.DatabaseConfig
		hostName string
	}

	config := args{dbConfig: dbConfig, hostName: "clm-pun-upjx84"}
	tests := []struct {
		name string
		args args
		want *EventCoordinator
	}{
		{"NewEdgeEventCoordinator_test", config, NewEdgeEventCoordinator(dbConfig, "clm-pun-upjx84")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewEdgeEventCoordinator(tt.args.dbConfig, tt.args.hostName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewEdgeEventCoordinator() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_eventCoordinator_CheckWhetherToPublishEvent(t *testing.T) {

	// Initialize the vars that we need to provide to test cases
	newEvent := event

	savedEvent := event
	savedEvent.Id = "EV001"
	savedEvent.CorrelationId += ":eventId001"
	savedEvent.Created = time.Now().UnixNano() / 1000000

	closedEvent := savedEvent
	closedEvent.Status = "Closed"

	nonExistingEvent := savedEvent
	nonExistingEvent.CorrelationId = ":eventId002"

	nonExistingEvent1 := savedEvent
	nonExistingEvent1.CorrelationId = ":eventId003"
	nonExistingEvent1.Status = "Closed"

	closedEvent2 := savedEvent
	closedEvent2.CorrelationId = ":eventId004"
	closedEvent2.Status = "Closed"

	// Remediation without pre-existing event
	remediationNoExistingEvent := event
	remediationNoExistingEvent.Id = "" // for new remediationAction
	remediationNoExistingEvent.EventType = "UserAction"
	remediationNoExistingEvent.IsRemediationTxn = true
	remediationNoExistingEvent.Remediations = make([]models.Remediation, 0)
	remediation := models.Remediation{
		Id:      "Ticket_REQ001",
		Type:    "ServiceRequest",
		Summary: "Service request to change faulty thermostat",
	}
	remediationNoExistingEvent.Remediations = append(remediationNoExistingEvent.Remediations, remediation)
	remediationNoExistingEvent.CorrelationId = "myUserActionNoEvent:Ticket_REQ001"

	// Remediation on pre-existing event
	remediationExistingEvent := remediationNoExistingEvent
	remediationExistingEvent.IsNewEvent = false
	remediationExistingEvent.CorrelationId = "myUserActionNoEventExisting:Ticket_REQ001"
	remediationExistingEvent.Id = "EV001"

	type args struct {
		ctx  interfaces.AppFunctionContext
		data interface{}
	}

	dBMockClient.On("GetEventByCorrelationId", newEvent.CorrelationId, mock.Anything).Return(nil, nil)
	dBMockClient.On("GetEventByCorrelationId", savedEvent.CorrelationId, mock.Anything).Return(&savedEvent, nil)

	dBMockClient.On("GetEventByCorrelationId", remediationNoExistingEvent.CorrelationId, mock.Anything).Return(&remediationNoExistingEvent, nil)
	dBMockClient.On("GetEventByCorrelationId", "myUserActionNoEvent", mock.Anything).Return(nil, nil)
	dBMockClient.On("GetEventByCorrelationId", "myUserActionNoEventExisting", mock.Anything).Return(&remediationNoExistingEvent, nil)
	dBMockClient.On("GetEventByCorrelationId", nonExistingEvent.CorrelationId, mock.Anything).Return(&nonExistingEvent, errors.New("mocker error"))
	dBMockClient.On("GetEventByCorrelationId", nonExistingEvent1.CorrelationId, mock.Anything).Return(nil, nil)
	dBMockClient.On("GetEventByCorrelationId", closedEvent2.CorrelationId, mock.Anything).Return(&closedEvent2, nil)

	dBMockClient.On("SaveEvent", mock.Anything).Return(nil)
	dBMockClient.On("DeleteEvent", savedEvent.CorrelationId).Return(nil)
	dBMockClient.On("DeleteEvent", closedEvent2.CorrelationId).Return(errors.New("mocked error"))

	tests := []struct {
		name                 string
		args                 args
		wantContinuePipeline bool
		newEvent             bool
		isRemediationTxn     bool
	}{
		{"CheckWhetherToPublishEvent_NewEvent", args{ctx, newEvent}, true, true, false},
		{"CheckWhetherToPublishEvent_ExistingEvent", args{ctx, savedEvent}, false, false, false},
		{"CheckWhetherToPublishEvent_CloseEvent", args{ctx, closedEvent}, true, false, false},
		{"CheckWhetherToPublishEvent_RemediationNoExistingEvent", args{ctx, remediationNoExistingEvent}, true, true, true},
		{"CheckWhetherToPublishEvent_RemediationExistingEvent", args{ctx, remediationExistingEvent}, true, false, true},
		{"CheckWhetherToPublishEvent_NoData", args{ctx, nil}, false, false, true},
		{"CheckWhetherToPublishEvent_WrongData", args{ctx, []byte(nil)}, false, false, true},
		{"CheckWhetherToPublishEvent_GetEventError", args{ctx, nonExistingEvent}, false, false, true},
		{"CheckWhetherToPublishEvent_EventIsNil", args{ctx, nonExistingEvent1}, false, false, true},
		{"CheckWhetherToPublishEvent_CloseEvent1", args{ctx, closedEvent2}, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EventCoordinator{
				dbClient: &dBMockClient,
				host:     "clm-pun-uojx84",
			}
			gotContinuePipeline, eventToPublish := e.CheckWhetherToPublishEvent(tt.args.ctx, tt.args.data)
			if gotContinuePipeline != tt.wantContinuePipeline {
				t.Errorf("CheckWhetherToPublishEvent() gotContinuePipeline = %v, want %v", gotContinuePipeline, tt.wantContinuePipeline)
			}
			if gotContinuePipeline {
				assert.Equal(t, tt.newEvent, eventToPublish.(models.HedgeEvent).IsNewEvent)
				assert.Equal(t, tt.isRemediationTxn, eventToPublish.(models.HedgeEvent).IsRemediationTxn)
			}

		})
	}
}
