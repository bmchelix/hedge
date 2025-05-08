/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	redis "hedge/app-services/hedge-event-publisher/pkg/db"
	"hedge/common/db"
	"github.com/hashicorp/go-uuid"
	"regexp"
	"strings"

	"hedge/common/models"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/pkg/errors"
	"sync"
	"time"
)

type EventCoordinator struct {
	dbClient redis.DBClient
	host     string
	mu       sync.Mutex
}

func NewEdgeEventCoordinator(dbConfig *db.DatabaseConfig, hostName string) *EventCoordinator {
	e := new(EventCoordinator)
	e.dbClient = redis.NewDBClient(dbConfig)
	e.host = hostName
	return e
}

// CheckWhetherToPublishEvent ensures that same event is not published again unless status or severity has changed
func (e *EventCoordinator) CheckWhetherToPublishEvent(ctx interfaces.AppFunctionContext, data interface{}) (continuePipeline bool, stringType interface{}) {
	if data == nil {
		return false, errors.New("No Event Received")
	}
	lc := ctx.LoggingClient()
	hedgeEvent, ok := data.(models.HedgeEvent)
	if !ok {
		return false, errors.New("type received is not a HedgeEvent")
	} else {
		lc.Debugf("New/Updated Event Received: %v", hedgeEvent)
	}

	// Set up the request object.
	//bmcEvent, _ := util.CoerceType(bmcEvent)

	isPublishEvent := false

	// Check if the event exist based on correlationId
	e.mu.Lock()
	defer e.mu.Unlock()

	if strings.Contains(hedgeEvent.CorrelationId, ":") && hedgeEvent.IsRemediationTxn {
		// try finding the event after removing the training eventId
		re := regexp.MustCompile(":[^:]*")
		hedgeEvent.CorrelationId = re.ReplaceAllString(hedgeEvent.CorrelationId, "")
		lc.Infof("truncated correlationId: %s", hedgeEvent.CorrelationId)
		//existingEvent, err = e.dbClient.GetEventByCorrelationId(bmcEvent.CorrelationId, lc)
	}
	existingEvent, err := e.dbClient.GetEventByCorrelationId(hedgeEvent.CorrelationId, lc)

	if err != nil || hedgeEvent.Status == "" {
		lc.Errorf("Error while getting Event by Correlation Id %s. Error: %v", hedgeEvent.CorrelationId, err)
		return false, err
	}

	// Open event handled here
	if existingEvent == nil && hedgeEvent.Status != "Closed" {
		// New event with status="In Progress" or "Open" will get added
		lc.Debugf("New Event being added: %s\n", hedgeEvent.CorrelationId)
		hedgeEvent.Id = ""
		hedgeEvent.IsNewEvent = true
		err = e.saveEventAndBuildCorrelationId(&hedgeEvent, lc)
		if err == nil {
			isPublishEvent = true
		}
		return isPublishEvent, hedgeEvent
	}

	if existingEvent == nil {
		// no existing event + New event in Closed state
		// We don't publish in this case, just ignore it
		lc.Debugf("No existing event, new event in closed status, so ignoring it: %s\n", hedgeEvent.CorrelationId)
		return false, nil
	}
	hedgeEvent.Id = existingEvent.Id
	hedgeEvent.CorrelationId = existingEvent.CorrelationId

	if hedgeEvent.Status == "Closed" {
		lc.Debugf("Deleting existing event: %s\n", hedgeEvent.CorrelationId)
		err = e.dbClient.DeleteEvent(hedgeEvent.CorrelationId)
		if err != nil {
			// Ignore the delete error for now, another close event will catch it when no data is found and will not publish multiple delete events

			lc.Errorf("Error while deleting event using correlationId from redis:%v", err)
		}
		// Don't overwrite existingEvent when closing, this code was in hedge-event, now moved in here
		hedgeEvent = *existingEvent
		hedgeEvent.Status = "Closed"
		hedgeEvent.IsNewEvent = false
		hedgeEvent.CorrelationId = buildCorrelationId(&hedgeEvent)

		// Instance specific correlationId
		isPublishEvent = true
		lc.Infof("Closing the event, event being overwritten by open event details: %v", hedgeEvent)

	} else if hedgeEvent.Status != "" && hedgeEvent.Status != existingEvent.Status || (hedgeEvent.Severity != "" && hedgeEvent.Severity != existingEvent.Severity) || hedgeEvent.IsRemediationTxn {
		// For remediation, need an explicit signal that it is from remediation
		//Update event in redis
		lc.Debugf("Updating existing event: %s\n", hedgeEvent.CorrelationId)

		hedgeEvent.IsNewEvent = false

		// Reset IsRemediationTxn to false so it doesn't impact the next call
		err = e.saveEventAndBuildCorrelationId(&hedgeEvent, ctx.LoggingClient())
		if err == nil {
			isPublishEvent = true
		}
	}

	// Perform the request with the client.
	if isPublishEvent == false {
		ctx.LoggingClient().Debugf("De-dup:Event will not be published: %s\n", hedgeEvent.CorrelationId)
	}

	return isPublishEvent, hedgeEvent
}

func (e *EventCoordinator) saveEventAndBuildCorrelationId(hedgeEvent *models.HedgeEvent, lc logger.LoggingClient) error {
	// If ID is empty, generate one
	if len(hedgeEvent.Id) == 0 {
		hedgeEvent.Id, _ = uuid.GenerateUUID()
	}
	currentTime := time.Now().UnixNano() / 1000000
	if hedgeEvent.Created == 0 {
		hedgeEvent.Created = currentTime // Donot overwrite createDate for existing docs
	}
	hedgeEvent.Modified = currentTime
	hedgeEvent.SourceNode = e.host

	err := e.dbClient.SaveEvent(*hedgeEvent)
	if err != nil {
		lc.Errorf("Error while saving event to redis database, error: %v", err)
	}
	// Update correlationId so it is unique by appending eventId
	hedgeEvent.CorrelationId = buildCorrelationId(hedgeEvent)
	return err
}

func buildCorrelationId(hedgeEvent *models.HedgeEvent) string {
	if !strings.HasSuffix(hedgeEvent.CorrelationId, hedgeEvent.Id) {
		return hedgeEvent.CorrelationId + ":" + hedgeEvent.Id
	} else {
		return hedgeEvent.CorrelationId
	}
}
