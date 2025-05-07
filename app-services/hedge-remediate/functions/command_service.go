/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"encoding/json"
	"errors"
	"fmt"
	redis "hedge/app-services/hedge-remediate/db"
	"hedge/common/client"
	"hedge/common/db"
	redis2 "hedge/common/db/redis"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	bootstrapinterfaces "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"hedge/common/models"

	"reflect"
)

type CommandExecutionService struct {
	service interfaces.ApplicationService

	deviceServiceExecutor DeviceServiceExecutorInterface
	node                  string
	currentNode           models.Node
	// Depending on whether serving node or core, the topic names being subscribed will be different
	subscribeTopic string
	redisClient    redis.RemediateDBClientInterface
	currentNodeId  string
	telemetry      *Telemetry
}

func NewCommandExecutionService(service interfaces.ApplicationService, serviceName string, metricsManager bootstrapinterfaces.MetricsManager, hostName string) *CommandExecutionService {
	cmdExecutionService := new(CommandExecutionService)
	cmdExecutionService.service = service

	// Need to fix this, too many calls here for redis
	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(service)
	cmdExecutionService.redisClient = redis.NewDBClient(dbConfig)
	cmdExecutionService.telemetry = NewTelemetry(service, serviceName, metricsManager, hostName, cmdExecutionService.redisClient)

	cmdExecutionService.deviceServiceExecutor = NewDeviceService(service)
	return cmdExecutionService
}

// IdentifyCommandAndExecute to differentiate between "ServiceRequest , DeviceCommand"
func (cmdExSvc CommandExecutionService) IdentifyCommandAndExecute(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	lc := ctx.LoggingClient()
	lc.Info("Identify Command And Execute ")

	var command models.Command
	ok := true
	if reflect.TypeOf(data) == reflect.TypeOf([]uint8(nil)) {
		err := json.Unmarshal(data.([]byte), &command)
		if err != nil {
			ctx.LoggingClient().Errorf("Error unmarshalling command: %v", err)
			return false, err
		}
	} else {
		command, ok = data.(models.Command)
	}

	if !ok {
		return false, errors.New("type received is not a Command")
	}
	if command.Type == "" {
		return false, errors.New("no Command Type (deviceCommand) in Command")
	}
	if command.CommandParameters == nil {
		return false, errors.New("no Command Parameters present in Command")
	}

	// Get the existing event, if any from local db cache, otherwise build a new one, let executors update it with remediation details and return it
	event, err := cmdExSvc.buildEvent(command)
	if err != nil {
		return false, err
	}

	switch command.Type {

	case client.LabelDeviceCommand:
		// Device Command
		ctx.LoggingClient().Info("Identified the Command as Command Device  %s", command.Type)
		if cmdExSvc.deviceServiceExecutor == nil {
			errMsg := "device command service is not configured here, check the topic you are publishing the command to"
			ctx.LoggingClient().Errorf("%s", errMsg)
			return false, errors.New(errMsg)
		}

		ok, remediation := cmdExSvc.deviceServiceExecutor.CommandExecutor(ctx, command)

		if !ok {
			return false, errors.New("device command execution returned error")
		}

		if event.CorrelationId == "" {
			event.CorrelationId = remediation.Id
		}

		event.Remediations = append(event.Remediations, remediation)
		err = cmdExSvc.telemetry.increment(command)
		if err != nil {
			ctx.LoggingClient().Errorf("%s", err.Error())
			return false, err
		}

	default:
		return false, errors.New(fmt.Sprintf("command Type not supported: %s", command.Type))
	}

	if err != nil {
		lc.Errorf("local save or delete event in remediate returned error: %v", err)
	}
	return true, event
}

func (cmdExSvc CommandExecutionService) buildEvent(command models.Command) (*models.HedgeEvent, error) {
	lc := cmdExSvc.service.LoggingClient()
	event, err := cmdExSvc.redisClient.GetRemediateEventByCorrelationId(command.CorrelationId, lc)
	if err != nil {
		return nil, err
	}
	if event == nil && command.Type == client.LabelCloseRequest && command.RemediationId == "" {
		// Case of
		return nil, errors.New("close dwp request command requires requestId/remediationId or correlationId")
	}
	if event == nil {
		// Open DWP manual request case, so build a new HedgeEvent that will get saved
		ev := models.HedgeEvent{

			Class:      models.BASE_EVENT_CLASS,
			EventType:  "UserAction",
			DeviceName: command.DeviceName,
			Name:       command.Name,
			Msg:        command.Problem,
			Severity:   command.Severity,
			//Priority:       "",
			//Profile:        "",
			SourceNode:    cmdExSvc.currentNodeId,
			CorrelationId: command.CorrelationId,
			Status:        "Open",
			//Location:      "",
			Version: 1,
			//AdditionalData:   nil,
			EventSource:      "commands",
			Labels:           make([]string, 0),
			Remediations:     make([]models.Remediation, 0),
			IsNewEvent:       true,
			IsRemediationTxn: true,
		}

		lc.Infof("New event being created as part of remediation")
		event = &ev
		//return &event, nil

	} else {
		command.EventId = event.Id
		event.IsNewEvent = false
		event.IsRemediationTxn = true
		event.CorrelationId = command.CorrelationId
		//command.RemediationId = event.Remediations[0]
		if command.RemediationId == "" && command.Type == client.LabelCloseRequest {
			// just stick the first remediationId that is the best we can do
			if event.Remediations != nil && len(event.Remediations) > 0 {
				for _, remediation := range event.Remediations {
					if remediation.Id != "" {
						lc.Infof("remediation Id considered for close: %s", remediation.Id)
						command.RemediationId = remediation.Id
						break
					}
				}
			}
			if command.RemediationId == "" {
				return nil, errors.New("close dwp request command requires requestId/remediationId, not found in event remediation")
			}
		}
		if command.DeviceName == "" {
			command.DeviceName = event.DeviceName
		}
		if command.SourceNode == "" {
			command.SourceNode = cmdExSvc.currentNodeId
		}

		// don't allow duplicate tickets etc, need more strict check: TODO:Girish
		if (event.Status == "Open" && command.Type == client.LabelNewRequest) || (event.Status == "Closed" && command.Type == client.LabelCloseRequest) {
			return nil, errors.New("duplicate remediate ticket being created, so rejecting this")
		}

	}

	// Event was getting overwritten when we publish modified event containing remediation-id or remediation steps
	if command.EventId != "" {
		event.Id = command.EventId
	}
	if command.EventType != "" {
		event.EventType = command.EventType
	}
	if command.Type == client.LabelCloseRequest {
		event.Status = "Closed"
	}
	lc.Infof("event to update command outcome: %v", *event)
	return event, nil
}

func (cmdExSvc CommandExecutionService) GetDbClient() redis2.CommonRedisDBInterface {
	return cmdExSvc.redisClient
}
