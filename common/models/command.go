/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

// Command Used for
// 1) Create DWP Service Request
// 2) Command to Device Service
type Command struct {
	Id                string                 `json:"id"                           codec:"id"`
	DeviceName        string                 `json:"device_name"                  codec:"deviceName"`                  // Complusory
	Type              string                 `json:"type"                         codec:"type"`                        // Complusory //"NewServiceRequest ,CloseServiceRequest, DeviceCommand"
	ExecutionNodeType string                 `json:"execution_nodetype,omitempty" codec:"executionNodeType,omitempty"` // Complusory //"Edge , Core"
	Name              string                 `json:"name,omitempty"               codec:"name,omitempty"`              // command name
	CommandParameters map[string]interface{} `json:"command_parameters,omitempty" codec:"commandParameters,omitempty"`
	Problem           string                 `json:"problem,omitempty"            codec:"problem,omitempty"`    // event msg
	SourceNode        string                 `json:"source_node,omitempty"        codec:"sourceNode,omitempty"` // Original Node from which the event got triggered/generated
	Severity          string                 `json:"severity,omitempty"           codec:"severity,omitempty"`
	CommandURI        string                 `json:"command_uri,omitempty"        codec:"commandURI,omitempty"`    // DeviceServiceURL,DWP URL Mobility19
	EventId           string                 `json:"event_id,omitempty"           codec:"eventId,omitempty"`       // Event.Id
	EventType         string                 `json:"event_type,omitempty"         codec:"eventType,omitempty"`     // EventType to be passed from event
	CorrelationId     string                 `json:"correlation_id,omitempty"     codec:"correlationId,omitempty"` // Complusory
	Created           int64                  `json:"created,omitempty"            codec:"created,omitempty"`
	RemediationId     string                 `json:"remediation_id,omitempty"     codec:"remediationId,omitempty"`
	Status            string                 `json:"status,omitempty"             codec:"status,omitempty"` // "Command Execution Status - Success , Failed"
}
