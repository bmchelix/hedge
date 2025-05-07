/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

const BASE_EVENT_CLASS = "OT_EVENT"

// CRITICAL, MAJOR, MINOR, WARNING, INFO, OK, UNKNOWN
const (
	SEVERITY_CRITICAL = "CRITICAL"
	SEVERITY_MAJOR    = "MAJOR"
	SEVERITY_MINOR    = "MINOR"
)

type HedgeEvent struct {
	Id         string `json:"id,omitempty"               codec:"id,omitempty"`
	Class      string `json:"class,omitempty"            codec:"class,omitempty"`      // EVENT, ANOMALY, ALARM, COMMAND
	EventType  string `json:"event_type,omitempty"       codec:"event_type,omitempty"` // CreateTicket, UpdateTicket,
	DeviceName string `json:"device_name,omitempty"      codec:"device_name,omitempty"`
	Name       string `json:"name,omitempty"             codec:"name,omitempty"`
	Msg        string `json:"msg,omitempty"              codec:"msg,omitempty"`
	Severity   string `json:"severity,omitempty"         codec:"severity,omitempty"`
	Priority   string `json:"priority,omitempty"         codec:"priority,omitempty"`
	Profile    string `json:"profile,omitempty"          codec:"profile,omitempty"`
	SourceNode string `json:"source_node,omitempty"      codec:"source_node,omitempty"` // Original Node from which the event got triggered/generated
	Status     string `json:"status,omitempty"           codec:"status,omitempty"`
	// Consider removing RelatedMetrics since the actualValues consider the name value pair, so this is already accounted for

	RelatedMetrics []string               `json:"related_metrics,omitempty"   codec:"related_metrics,omitempty"`
	Thresholds     map[string]interface{} `json:"thresholds,omitempty"        codec:"thresholds,omitempty"`
	ActualValues   map[string]interface{} `json:"actual_values,omitempty"`
	ActualValueStr string                 `json:"actual_values_str,omitempty"`
	Unit           string                 `json:"unit,omitempty"             codec:"unit,omitempty"`
	Location       string                 `json:"location,omitempty"         codec:"location,omitempty"`
	Version        int64                  `json:"version,omitempty"          codec:"version,omitempty"`
	AdditionalData map[string]string      `json:"additional_data,omitempty"  codec:"additional_data,omitempty" xml:"-"`
	EventSource    string                 `json:"event_source,omitempty"     codec:"event_source,omitempty"`
	CorrelationId  string                 `json:"correlation_id,omitempty"   codec:"correlation_id,omitempty"`
	Labels         []string               `json:"labels,omitempty"             codec:"labels,omitempty"`
	Remediations   []Remediation          `json:"remediations,omitempty"     codec:"remediations,omitempty"`
	Created        int64                  `json:"created,omitempty"          codec:"created,omitempty"`
	Modified       int64                  `json:"modified,omitempty"         codec:"modified,omitempty"`
	// We use this flag to indicate whether the same event is being updated or is it a first time. Updates will come when severity changes or status changes from open to closed
	IsNewEvent       bool `json:"new_event"                  codec:"new_event"`
	IsRemediationTxn bool `json:"remediation_txn"            codec:"remediation_txn"`
}

type Remediation struct {
	// Type of remediation, Ticket Creation, DeviceCommands
	Id           string `json:"id,omitempty"            codec:"id,omitempty"`
	Type         string `json:"type,omitempty"          codec:"type,omitempty"`
	Summary      string `json:"summary,omitempty"       codec:"summary,omitempty"`
	Status       string `json:"status,omitempty"        codec:"status,omitempty"`
	ErrorMessage string `json:"error_message,omitempty" codec:"errorMessage,omitempty"`
}
