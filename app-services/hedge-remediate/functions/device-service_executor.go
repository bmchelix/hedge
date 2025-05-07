/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	cl "hedge/common/client"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"

	"hedge/common/models"
)

type DeviceServiceExecutorInterface interface {
	CommandExecutor(ctx interfaces.AppFunctionContext, command models.Command) (bool, models.Remediation)
}

type DeviceServiceExecutor struct {
	service interfaces.ApplicationService
}

func NewDeviceService(service interfaces.ApplicationService) DeviceServiceExecutorInterface {
	deviceService := new(DeviceServiceExecutor)
	deviceService.service = service
	return deviceService
}

func (d DeviceServiceExecutor) CommandExecutor(ctx interfaces.AppFunctionContext, command models.Command) (bool, models.Remediation) {
	var valid = true
	var errorMessage = ""

	var methodName string
	if command.CommandParameters != nil && command.CommandParameters["method"] != nil {
		methodName = command.CommandParameters["method"].(string)
	} else {
		remediationEntry := buildDeviceCmdRemediationEntry(false, command, "Method Not Found")
		ctx.LoggingClient().Infof("new remediation entry : %v", remediationEntry)
		return false, remediationEntry
	}

	body, ok := command.CommandParameters["body"]
	if !ok {
		remediationEntry := buildDeviceCmdRemediationEntry(false, command, "Payload for the HTTP Call Not Found")
		ctx.LoggingClient().Infof("new remediation entry : %v", remediationEntry)
		return false, remediationEntry
	}

	data, err := json.Marshal(body)
	if err != nil {
		remediationEntry := buildDeviceCmdRemediationEntry(false, command, "Payload for the HTTP Call is not Valid Json")
		ctx.LoggingClient().Infof("new remediation entry : %v", remediationEntry)
		return false, remediationEntry
	}

	_, err = d.restCall(methodName, command.CommandURI, data)
	if err != nil {
		valid = false
		errorMessage = err.Error()
		ctx.LoggingClient().Errorf("Device Command Execution failed, error: %v", err)
	} else {
		ctx.LoggingClient().Infof("Output of Device Command Execution success")
	}

	remediationEntry := buildDeviceCmdRemediationEntry(valid, command, errorMessage)
	ctx.LoggingClient().Infof("new remediation entry : %v", remediationEntry)

	return valid, remediationEntry
}

func (d DeviceServiceExecutor) restCall(methodName string, URL string, data []byte) (result []byte, err error) {

	req, err := http.NewRequest(methodName, URL, bytes.NewBuffer(data))
	//SET JWT token for rest Calls
	req.Header.Add("Content-Type", "application/json")

	// Get Request details
	resp, err := cl.Client.Do(req)
	if err != nil {
		d.service.LoggingClient().Errorf("error = %s \n", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 500 || resp.StatusCode == 400 || resp.StatusCode == 403 {
		d.service.LoggingClient().Errorf("call to device service failed: %v", resp)
		return nil, errors.New("call to device service failed")
	}

	return nil, err
}

func buildDeviceCmdRemediationEntry(valid bool, command models.Command, errMessage string) models.Remediation {
	remediation := models.Remediation{
		Id:   command.RemediationId,
		Type: command.Type,
	}

	// If Ticket Generation failed
	if valid {
		remediation.Status = cl.StatusSuccess
	} else {
		remediation.Status = cl.StatusFail
		remediation.ErrorMessage = errMessage
	}

	return remediation
}
