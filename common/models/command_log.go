/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

type CommandExecutionLog struct {
	Id            string   `json:"id,omitempty"            codec:"id,omitempty"`
	DeviceName    string   `json:"deviceName,omitempty"    codec:"deviceName,omitempty"`
	Profile       string   `json:"profile,omitempty"       codec:"profile,omitempty"`
	CommandType   string   `json:"commandType,omitempty"   codec:"commandType,omitempty"` //  Ticket Type ( WO, INC, CRQ)
	Problem       string   `json:"problem,omitempty"       codec:"problem,omitempty"`     // Event msg // command msg
	TicketId      string   `json:"ticketId,omitempty"      codec:"ticketId,omitempty"`    // Req Id
	Status        string   `json:"status,omitempty"        codec:"status,omitempty"`
	Summary       string   `json:"summary,omitempty"       codec:"summary,omitempty"` //
	EventId       string   `json:"eventId,omitempty"       codec:"eventId,omitempty"`
	Severity      string   `json:"severity,omitempty"      codec:"severity,omitempty"`
	Customer      string   `json:"Customer,omitempty"      codec:"Customer,omitempty"`
	SLA           string   `json:"SLA,omitempty"           codec:"SLA,omitempty"`
	RefLink       string   `json:"refLink,omitempty"       codec:"refLink,omitempty"`
	CorrelationId string   `json:"correlationId,omitempty" codec:"correlationId,omitempty"` // deviceName_<src>eventId, deviceName_<src>ticketId
	Created       int64    `json:"created,omitempty"       codec:"created,omitempty"`
	Modified      int64    `json:"modified,omitempty"      codec:"modified,omitempty"`
	TicketType    string   `json:"ticketType,omitempty"    codec:"ticketType,omitempty"`
	DeviceLabels  []string `json:"devicelabels,omitempty"  codec:"devicelabels,omitempty"`
}
