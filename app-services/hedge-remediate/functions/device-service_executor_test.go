/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	cl "hedge/common/client"
	"hedge/common/models"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"reflect"
	"testing"
)

type DeviceServiceFields struct {
	service interfaces.ApplicationService
}

var (
	dseU       *utils.HedgeMockUtils
	dseF       DeviceServiceFields
	httpClient *utils.MockClient
)

func init() {
	dseU = utils.NewApplicationServiceMock(map[string]string{"Topic": "SomeTopic", "MetricReportInterval": "10"})
	dseF = DeviceServiceFields{
		service: dseU.AppService,
	}
	httpClient = utils.NewMockClient()
	cl.Client = httpClient
	httpClient.RegisterExternalMockRestCall("success", "post", nil, 200)
	httpClient.RegisterExternalMockRestCall("force http error", "post", nil, 500, errors.New("http error"))
	httpClient.RegisterExternalMockRestCall("force error code", "post", nil, 500)
}

func TestDeviceServiceExecutor_CommandExecutor(t *testing.T) {

	type args struct {
		ctx     interfaces.AppFunctionContext
		command models.Command
	}

	argsBase := args{
		ctx: dseU.AppFunctionContext,
	}

	commandBase := models.Command{
		Id:            "111",
		RemediationId: "2",
		Type:          cl.LabelNewRequest,
	}

	args1 := argsBase
	args1.command = commandBase
	expRemediation1 := models.Remediation{
		Id:           args1.command.RemediationId,
		Type:         args1.command.Type,
		Status:       cl.StatusFail,
		ErrorMessage: "Method Not Found",
	}

	command2 := commandBase
	command2.CommandParameters = map[string]interface{}{"method": "post"}
	args2 := argsBase
	args2.command = command2
	expRemediation2 := models.Remediation{
		Id:           args1.command.RemediationId,
		Type:         args1.command.Type,
		Status:       cl.StatusFail,
		ErrorMessage: "Payload for the HTTP Call Not Found",
	}

	unsupportedJson := make(chan int)
	command3 := commandBase
	command3.CommandParameters = map[string]interface{}{"method": "post", "body": unsupportedJson}
	args3 := argsBase
	args3.command = command3
	expRemediation3 := models.Remediation{
		Id:           args1.command.RemediationId,
		Type:         args1.command.Type,
		Status:       cl.StatusFail,
		ErrorMessage: "Payload for the HTTP Call is not Valid Json",
	}

	command4 := commandBase
	command4.CommandParameters = map[string]interface{}{"method": "post", "body": "some body"}
	command4.CommandURI = "force http error"
	args4 := argsBase
	args4.command = command4
	expRemediation4 := models.Remediation{
		Id:           args1.command.RemediationId,
		Type:         args1.command.Type,
		Status:       cl.StatusFail,
		ErrorMessage: "http error",
	}

	command5 := commandBase
	command5.CommandParameters = map[string]interface{}{"method": "post", "body": "some body"}
	command5.CommandURI = "force error code"
	args5 := argsBase
	args5.command = command5
	expRemediation5 := models.Remediation{
		Id:           args1.command.RemediationId,
		Type:         args1.command.Type,
		Status:       cl.StatusFail,
		ErrorMessage: "call to device service failed",
	}

	command6 := commandBase
	command6.CommandParameters = map[string]interface{}{"method": "post", "body": "some body"}
	command5.CommandURI = "success"
	args6 := argsBase
	args6.command = command5
	expRemediation6 := models.Remediation{
		Id:     args1.command.RemediationId,
		Type:   args1.command.Type,
		Status: cl.StatusSuccess,
	}

	tests := []struct {
		name   string
		fields DeviceServiceFields
		args   args
		want   bool
		want1  models.Remediation
	}{
		{"CommandExecutor Method Not Found - Error", dseF, args1, false, expRemediation1},
		{"CommandExecutor Payload Not Found - Error", dseF, args2, false, expRemediation2},
		{"CommandExecutor Payload Not Found - Error", dseF, args3, false, expRemediation3},
		{"CommandExecutor Rest Call Failed - Error", dseF, args4, false, expRemediation4},
		{"CommandExecutor Rest Call Device service Failed - Error", dseF, args5, false, expRemediation5},
		{"CommandExecutor - Pass", dseF, args6, true, expRemediation6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DeviceServiceExecutor{
				service: tt.fields.service,
			}
			got, got1 := d.CommandExecutor(tt.args.ctx, tt.args.command)
			if got != tt.want {
				t.Errorf("CommandExecutor() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("CommandExecutor() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestDeviceServiceExecutor_restCall(t *testing.T) {
	type args struct {
		methodName string
		URL        string
		data       []byte
	}

	argsBase := args{
		methodName: "post",
	}

	args1 := argsBase
	args1.URL = "force http error"
	args1.data = nil
	expErr1 := errors.New("http error")

	args2 := argsBase
	args2.URL = "force error code"
	args2.data = nil
	expErr2 := errors.New("call to device service failed")

	args3 := argsBase
	args3.URL = "success"
	args1.data = nil

	tests := []struct {
		name       string
		fields     DeviceServiceFields
		args       args
		wantResult []byte
		wantErr    bool
		expErr     error
	}{
		{"restCall Http Error - Error", dseF, args1, nil, true, expErr1},
		{"restCall Http Error Code - Error", dseF, args2, nil, true, expErr2},
		{"restCall - Success", dseF, args3, nil, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DeviceServiceExecutor{
				service: tt.fields.service,
			}
			gotResult, err := d.restCall(tt.args.methodName, tt.args.URL, tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("restCall() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("restCall() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestNewDeviceService(t *testing.T) {
	type args struct {
		service interfaces.ApplicationService
	}

	args1 := args{
		service: dseU.AppService,
	}
	expResult1 := new(DeviceServiceExecutor)
	expResult1.service = args1.service

	tests := []struct {
		name string
		args args
		want DeviceServiceExecutorInterface
	}{
		{"TestNewDeviceService - Success", args1, expResult1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewDeviceService(tt.args.service); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDeviceService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildDeviceCmdRemediationEntry(t *testing.T) {
	type args struct {
		valid      bool
		command    models.Command
		errMessage string
	}

	command := models.Command{
		RemediationId: "1",
		Type:          cl.LabelNewRequest,
	}

	args1 := args{
		valid:   true,
		command: command,
	}
	expRemediation1 := models.Remediation{
		Id:     command.RemediationId,
		Type:   command.Type,
		Status: cl.StatusSuccess,
	}

	args2 := args{
		valid:      false,
		command:    command,
		errMessage: "test error message",
	}

	expRemediation2 := models.Remediation{
		Id:           command.RemediationId,
		Type:         command.Type,
		Status:       cl.StatusFail,
		ErrorMessage: args2.errMessage,
	}
	tests := []struct {
		name string
		args args
		want models.Remediation
	}{
		{"buildDeviceCmdRemediationEntry - Build Success Status", args1, expRemediation1},
		{"buildDeviceCmdRemediationEntry - Build Fail Status", args2, expRemediation2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDeviceCmdRemediationEntry(tt.args.valid, tt.args.command, tt.args.errMessage); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildDeviceCmdRemediationEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}
