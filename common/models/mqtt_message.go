/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

type MQTTmessage struct {
	MessageDefinition MessageDefinition `json:"messageDefinition"`
	Payload           interface{}       `json:"payload,omitempty"`
}

type MessageDefinition struct {
	TargetService string `json:"targetService"`
	TargetURL     string `json:"targetURL"`
	HTTPMethod    string `json:"httpMethod"`
	CorrelationID string `json:"correlationId"`
}
