/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"encoding/json"
	inter "github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/go-redsync/redsync/v4"
	gometrics "github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
	"hedge/common/telemetry"
	"reflect"
	"strings"
	"testing"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/stretchr/testify/mock"
	redis "hedge/app-services/hedge-remediate/db"
	"hedge/common/client"
	"hedge/common/models"
	redisMock "hedge/mocks/hedge/app-services/hedge-remediate/db"
	functionsMock "hedge/mocks/hedge/app-services/hedge-remediate/functions"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
)

type fields struct {
	service               interfaces.ApplicationService
	deviceServiceExecutor DeviceServiceExecutorInterface
	node                  string
	currentNode           models.Node
	subscribeTopic        string
	redisClient           redis.RemediateDBClientInterface
	currentNodeId         string
	telemetry             *Telemetry
}

var (
	csU                       *utils.HedgeMockUtils
	csF                       fields
	requestServiceMock        functionsMock.MockRequestServiceInterface
	deviceServiceExecutorMock functionsMock.MockDeviceServiceExecutorInterface
	remediateDBClientMock     redisMock.MockRemediateDBClientInterface
	remediation               models.Remediation
)

func init() {
	csU = utils.NewApplicationServiceMock(map[string]string{"Topic": "SomeTopic", "MetricReportInterval": "10"})
	remediateDBClientMock.On("GetRemediateEventByCorrelationId", mock.Anything, mock.Anything).Return(nil, nil)
	remediateDBClientMock.On("SaveRemediateEvent", mock.Anything).Return(nil)
	remediateDBClientMock.On("DeleteRemediateEvent", mock.Anything).Return(nil)
	remediateDBClientMock.On("GetMetricCounter", mock.Anything).Return(int64(0), nil)
	remediateDBClientMock.On("IncrMetricCounterBy", mock.Anything, mock.Anything).Return(int64(0), nil)
	remediateDBClientMock.On("AcquireRedisLock", mock.Anything).Return(&redsync.Mutex{}, nil)

	remediation = models.Remediation{
		Id:   "1",
		Type: client.LabelTicketType,
	}

	requestServiceMock.On("CommandExecutor", mock.Anything, mock.Anything).Return(true, remediation)
	deviceServiceExecutorMock.On("CommandExecutor", mock.Anything, mock.Anything).Return(true, remediation)

	telemetry := Telemetry{}
	telemetry.commandsExecuted = gometrics.NewCounter()
	telemetry.commandMessages = gometrics.NewCounter()
	telemetry.redisClient = &remediateDBClientMock

	csF = fields{
		service:               csU.AppService,
		deviceServiceExecutor: &deviceServiceExecutorMock,
		node:                  "test_node",
		currentNode:           models.Node{},
		subscribeTopic:        "test_topic",
		redisClient:           &remediateDBClientMock,
		currentNodeId:         "1",
		telemetry:             &telemetry,
	}
}

func TestCommandExecutionService_IdentifyCommandAndExecute(t *testing.T) {
	var requestServiceMock6 functionsMock.MockRequestServiceInterface
	var deviceServiceExecutorMock8 functionsMock.MockDeviceServiceExecutorInterface

	requestServiceMock6.On("CommandExecutor", mock.Anything, mock.Anything).Return(false, models.Remediation{})
	deviceServiceExecutorMock8.On("CommandExecutor", mock.Anything, mock.Anything).Return(false, models.Remediation{})

	type args struct {
		ctx  interfaces.AppFunctionContext
		data interface{}
	}

	commandParameters := map[string]interface{}{
		"k1":   1,
		"k2":   2,
		"body": "",
	}

	argsBase := args{
		ctx: csU.AppFunctionContext,
	}

	args1 := argsBase
	args1.data = []byte{1, 2, 3}

	args2 := argsBase
	args2.data = args1

	command3 := models.Command{
		Id: "111",
	}
	args3 := argsBase
	args3.data = command3

	command4 := command3
	command4.Id = "111"
	command4.Type = client.LabelNewRequest

	args4 := argsBase
	args4.data = command4

	command5 := command4
	command5.CommandParameters = commandParameters

	args5 := args4
	args5.data = command5

	command6 := models.Command{
		Id:                "111",
		Type:              client.LabelNewRequest,
		DeviceName:        "Test Device",
		Name:              "Test Name",
		Problem:           "Something bad happened",
		Severity:          models.SEVERITY_MAJOR,
		CorrelationId:     "1",
		CommandParameters: commandParameters,
	}
	args6 := args5
	args6.data = command6

	expEvent6 := models.HedgeEvent{
		Class:            models.BASE_EVENT_CLASS,
		EventType:        "UserAction",
		DeviceName:       command6.DeviceName,
		Name:             command6.Name,
		Msg:              command6.Problem,
		Severity:         command6.Severity,
		SourceNode:       csF.currentNodeId,
		CorrelationId:    command6.CorrelationId,
		Status:           "Open",
		Location:         "",
		Version:          1,
		AdditionalData:   nil,
		EventSource:      "commands",
		Labels:           make([]string, 0),
		Remediations:     []models.Remediation{remediation},
		IsNewEvent:       true,
		IsRemediationTxn: true,
	}

	command7 := command6
	command7.Type = client.LabelDeviceCommand

	args7 := args6
	args7.data = command7

	f7 := csF
	f7.deviceServiceExecutor = nil

	f8 := csF
	f8.deviceServiceExecutor = &deviceServiceExecutorMock8
	args8 := args7
	expEvent8 := expEvent6

	command9 := command6
	command9.Type = client.LabelEventType
	args9 := args6
	args9.data = command9

	jsonCommand, _ := json.Marshal(command7)
	args10 := argsBase
	args10.data = jsonCommand
	expEvent10 := expEvent6

	command11 := command6
	command11.Type = client.LabelCloseRequest
	command11.RemediationId = "111"
	args11 := argsBase
	args11.data = command11
	expEvent11 := expEvent10
	expEvent11.Status = "Closed"

	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
		want1  interface{}
	}{
		{"IdentifyCommandAndExecute Invalid data - Error", csF, args1, false, "invalid character"},
		{"IdentifyCommandAndExecute Invalid command - Error", csF, args2, false, "type received is not a Command"},
		{"IdentifyCommandAndExecute Missing Command Type - Error", csF, args3, false, "no Command Type (deviceCommand) in Command"},
		{"IdentifyCommandAndExecute Missing Command Parameters - Error", csF, args4, false, "no Command Parameters present in Command"},
		//{"IdentifyCommandAndExecute Missing Request Service - Error", f5, args5, false, "DWP service is not configured here, check the topic you are publishing the command"},
		//{"IdentifyCommandAndExecute NewServiceRequest Command Executor - Error", f6, args6, false, "service request creation failed"},
		//{"IdentifyCommandAndExecute NewRequest - Pass", csF, args6, true, "command Type not supported: NewRequest"},
		{"IdentifyCommandAndExecute LabelDeviceCommand Missing Device Service - Error", f7, args7, false, "device command service is not configured here, check the topic you are publishing the command to"},
		{"IdentifyCommandAndExecute LabelDeviceCommand Device Service Executor - Error", f8, args7, false, "device command execution returned error"},
		{"IdentifyCommandAndExecute LabelDeviceCommand - Pass", csF, args8, true, &expEvent8},
		{"IdentifyCommandAndExecute Unsupported Command Type - Error", csF, args9, false, "command Type not supported: type"},
		{"IdentifyCommandAndExecute Json Command - Pass", csF, args10, true, &expEvent10},
		//	{"IdentifyCommandAndExecute LabelCloseServiceRequest - Pass", csF, args11, true, &expEvent11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdExSvc := CommandExecutionService{
				service:               tt.fields.service,
				deviceServiceExecutor: tt.fields.deviceServiceExecutor,
				node:                  tt.fields.node,
				currentNode:           tt.fields.currentNode,
				subscribeTopic:        tt.fields.subscribeTopic,
				redisClient:           tt.fields.redisClient,
				currentNodeId:         tt.fields.currentNodeId,
				telemetry:             tt.fields.telemetry,
			}
			got, got1 := cmdExSvc.IdentifyCommandAndExecute(tt.args.ctx, tt.args.data)
			if !got {
				if err, res := got1.(error); res {
					errMessage := tt.want1.(string)
					if !strings.Contains(err.Error(), errMessage) {
						t.Errorf("IdentifyCommandAndExecute error message '%s', got '%s'", tt.want1, err.Error())
					}
				} else {
					t.Errorf("IdentifyCommandAndExecute() expected error result, got '%s'", reflect.TypeOf(got1))
					return
				}
			} else {
				if !reflect.DeepEqual(got1, tt.want1) {
					t.Errorf("IdentifyCommandAndExecute() got1 = %v, want %v", got1, tt.want1)
				}
			}
		})
	}
}

func TestCommandExecutionService_buildEvent(t *testing.T) {
	var remediateDBClientMock3 redisMock.MockRemediateDBClientInterface
	var remediateDBClientMock4 redisMock.MockRemediateDBClientInterface

	event1 := models.HedgeEvent{
		Id:           "111",
		Remediations: []models.Remediation{remediation},
		DeviceName:   "Device1",
		Status:       "Open",
	}

	event2 := event1
	event2.Remediations = []models.Remediation{{}}

	remediateDBClientMock3.On("GetRemediateEventByCorrelationId", mock.Anything, mock.Anything).Return(&event1, nil)
	remediateDBClientMock4.On("GetRemediateEventByCorrelationId", mock.Anything, mock.Anything).Return(&event2, nil)

	f3 := csF
	f3.redisClient = &remediateDBClientMock3

	f4 := csF
	f4.redisClient = &remediateDBClientMock4

	command1 := models.Command{
		Type:          client.LabelCloseRequest,
		RemediationId: "",
	}

	command2 := models.Command{
		Type:          client.LabelNewRequest,
		DeviceName:    "Test Device",
		Name:          "Test Name",
		Problem:       "Something bad happened",
		Severity:      models.SEVERITY_MAJOR,
		CorrelationId: "1",
	}

	type args struct {
		command models.Command
	}
	args1 := args{
		command: command1,
	}
	args2 := args{
		command: command2,
	}
	args3 := args1
	args4 := args1
	args5 := args2

	expErr1 := "close dwp request command requires requestId/remediationId or correlationId"
	expErr4 := "close dwp request command requires requestId/remediationId, not found in event remediation"
	expErr5 := "duplicate remediate ticket being created, so rejecting this"

	expEvent2 := models.HedgeEvent{
		Class:            models.BASE_EVENT_CLASS,
		EventType:        "UserAction",
		DeviceName:       command2.DeviceName,
		Name:             command2.Name,
		Msg:              command2.Problem,
		Severity:         command2.Severity,
		SourceNode:       csF.currentNodeId,
		CorrelationId:    command2.CorrelationId,
		Status:           "Open",
		Location:         "",
		Version:          1,
		AdditionalData:   nil,
		EventSource:      "commands",
		Labels:           make([]string, 0),
		Remediations:     make([]models.Remediation, 0),
		IsNewEvent:       true,
		IsRemediationTxn: true,
	}

	expEvent3 := event1
	expEvent3.IsNewEvent = false
	expEvent3.Status = "Closed"
	expEvent3.IsRemediationTxn = true

	tests := []struct {
		name        string
		fields      fields
		args        args
		want        *models.HedgeEvent
		wantErr     bool
		expectedErr string
	}{
		{"buildEvent New Close Event missing Remediation Id - Error", csF, args1, nil, true, expErr1},
		{"buildEvent New Event - Pass", csF, args2, &expEvent2, false, ""},
		{"buildEvent Existing Event - Pass", f3, args3, &expEvent3, false, ""},
		{"buildEvent Existing Event missing Remediation Id - Error", f4, args4, nil, true, expErr4},
		{"buildEvent Existing Event duplicate ticket - Error", f4, args5, nil, true, expErr5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdExSvc := CommandExecutionService{
				service:        tt.fields.service,
				node:           tt.fields.node,
				currentNode:    tt.fields.currentNode,
				subscribeTopic: tt.fields.subscribeTopic,
				redisClient:    tt.fields.redisClient,
				currentNodeId:  tt.fields.currentNodeId,
				telemetry:      tt.fields.telemetry,
			}
			got, err := cmdExSvc.buildEvent(tt.args.command)
			if err != nil {
				if tt.wantErr == false {
					t.Errorf("buildEvent() error = %v, wantErr %v", err, tt.wantErr)
					return
				} else if errMsg := err.Error(); errMsg != tt.expectedErr {
					t.Errorf("Expected error message '%s', got '%s'", tt.expectedErr, errMsg)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildEvent() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTelemetry_Increment(t *testing.T) {
	commandMessagesCounter := gometrics.NewCounter()
	commandsExecutedCounter := gometrics.NewCounter()

	tel := &Telemetry{
		commandMessages:  commandMessagesCounter,
		commandsExecuted: commandsExecutedCounter,
		redisClient:      &remediateDBClientMock,
	}
	tests := []struct {
		name               string
		command            models.Command
		expectedMessages   int64
		expectedExecutions int64
	}{
		{
			name: "Single message under MaxMessageSize",
			command: models.Command{
				CommandParameters: map[string]interface{}{
					"body": []interface{}{1, 2, 3},
				},
			},
			expectedMessages:   1,
			expectedExecutions: 1,
		},
		{
			name: "Single message exactly MaxMessageSize",
			command: models.Command{
				CommandParameters: map[string]interface{}{
					"body": make([]interface{}, telemetry.MaxMessageSize),
				},
			},
			expectedMessages:   1,
			expectedExecutions: 1,
		},
		{
			name: "Multiple messages over MaxMessageSize",
			command: models.Command{
				CommandParameters: map[string]interface{}{
					"body": make([]interface{}, telemetry.MaxMessageSize+5),
				},
			},
			expectedMessages:   2,
			expectedExecutions: 1,
		},
		{
			name: "Invalid body type",
			command: models.Command{
				CommandParameters: map[string]interface{}{
					"body": "invalidType",
				},
			},
			expectedMessages:   0,
			expectedExecutions: 0,
		},
		{
			name: "Missing body",
			command: models.Command{
				CommandParameters: map[string]interface{}{},
			},
			expectedMessages:   0,
			expectedExecutions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commandMessagesCounter.Clear()
			commandsExecutedCounter.Clear()

			tel.increment(tt.command)
			if got := commandMessagesCounter.Count(); got != tt.expectedMessages {
				t.Errorf("commandMessages counter = %v, want %v", got, tt.expectedMessages)
			}
			if got := commandsExecutedCounter.Count(); got != tt.expectedExecutions {
				t.Errorf("commandsExecuted counter = %v, want %v", got, tt.expectedExecutions)
			}
		})
	}
}

func TestNewTelemetry(t *testing.T) {
	type args struct {
		service        interfaces.ApplicationService
		serviceName    string
		metricsManager inter.MetricsManager
		hostName       string
		redisClient    redis.RemediateDBClientInterface
	}

	expectedTelemetry := &Telemetry{
		commandsExecuted: gometrics.NewCounter(),
		commandMessages:  gometrics.NewCounter(),
		redisClient:      &remediateDBClientMock,
	}
	metricsManager, _ := telemetry.NewMetricsManager(csU.AppService, "HedgeRemediateServiceName")

	arg1 := args{
		service:        csU.AppService,
		serviceName:    "Test Service",
		metricsManager: metricsManager.MetricsMgr,
		hostName:       "localhost",
		redisClient:    &remediateDBClientMock,
	}
	tests := []struct {
		name string
		args args
		want *Telemetry
	}{
		{"NewTelemetry - Passed", arg1, expectedTelemetry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.name, "Failed") {
				got := NewTelemetry(tt.args.service, tt.args.serviceName, tt.args.metricsManager, tt.args.hostName, tt.args.redisClient)
				if got != nil {
					t.Errorf("NewTelemetry() == %v, want %v", got, tt.want)
				}
			} else {
				got := NewTelemetry(tt.args.service, tt.args.serviceName, tt.args.metricsManager, tt.args.hostName, tt.args.redisClient)

				if !reflect.DeepEqual(got.commandsExecuted.Count(), tt.want.commandsExecuted.Count()) {
					t.Errorf("NewTelemetry().commandsExecuted.Count() = %v, want %v", got.commandsExecuted.Count(), tt.want.commandsExecuted.Count())
				}
				if !reflect.DeepEqual(got.commandMessages.Count(), tt.want.commandMessages.Count()) {
					t.Errorf("NewTelemetry().commandMessages.Count() = %v, want %v", got.commandMessages.Count(), tt.want.commandMessages.Count())
				}
			}
		})
	}
}

func TestGetDbClient(t *testing.T) {
	cs := &CommandExecutionService{
		redisClient: &remediateDBClientMock,
	}
	dbClient := cs.GetDbClient()
	assert.Equal(t, &remediateDBClientMock, dbClient, "Expected the returned DB client to be the mock client")
}
