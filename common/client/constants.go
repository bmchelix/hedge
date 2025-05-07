/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package client

// Constants related to how services identify themselves in the Service Registry
const (
	ServiceKeyHedgePrefix = "app-hedge-"

	// ServiceNames
	HedgeDataEnrichmentServiceName = "data-enrichment"
	HedgeAdminServiceName          = "hedge-admin"
	HedgeDeviceExtnsServiceName    = "hedge-device-extensions"
	HedgeRemediateServiceName      = "hedge-remediate"
	HedgeMetaSyncServiceName       = "meta-sync"
	HedgeMLManagementServiceName   = "hedge-ml-management"

	// ServiceKeys - note that the service key should start with app- for appservices and with device- for deviceservices
	HedgeDataEnrichmentServiceKey   = "app-data-enrichment"
	HedgeAdminServiceKey            = "app-hedge-admin"
	HedgeDeviceExtnsServiceKey      = "app-hedge-device-extensions"
	HedgeEventServiceKey            = "app-hedge-event"
	HedgeEventPublisherServiceKey   = "app-hedge-event-publisher"
	HedgeExportServiceKey           = "app-hedge-export"
	HedgeRemediateServiceKey        = "app-hedge-remediate"
	HedgeUserAppMgmtServiceKey      = "app-hedge-user-app-mgmt"
	HedgeExportBizDataServiceKey    = "app-export-biz-data"
	HedgeMetaSyncServiceKey         = "app-meta-sync"
	HedgeNatsProxyServiceKey        = "app-nats-proxy"
	HedgeMLManagementServiceKey     = "app-hedge-ml-management"
	HedgeMLBrokerServiceKey         = "app-hedge-ml-broker"
	HedgeMLEdgeAgentServiceKey      = "app-hedge-ml-edge-agent"
	HedgeMLSandboxServiceKey        = "app-hedge-ml-sandbox"
	DeviceVirtual                   = "device-virtual"
	HedgeMetaDataNotifierServiceKey = "app-metadata-notifier"
	DigitalTwinServiceKey           = "app-digital-twin"
	SimulationPipelineServiceKey    = "app-simulation-pipeline"
)

const (
	CommandUpdateTicketId = "UpdateTicketId"
	CommandName           = "Command"
)

const (
	MetricEvent       = "IoTEvent"
	MetricTicket      = "Ticket"
	LabelDeviceName   = "device"
	LabelDeviceLabels = "tags"
	LabelProfileName  = "profile"
	LabelNodeName     = "host"
	// LabelLocation            = "site"
	LabelCorrelationId = "correlation_id"
	LabelEventType     = "type"
	LabelTicketType    = "type"
	LabelEventSummary  = "summary"
	LabelTicketSummary = "summary"
	LabelNameSpace     = "namespace"
	LabelNewRequest    = "NewRequest"
	LabelCloseRequest  = "CloseRequest"
	LabelDeviceCommand = "DeviceCommand"
	LabelStatus        = "status"
	LabelCommandType   = "commandType"
	StatusSuccess      = "Success"
	StatusFail         = "Failed"
	StatusClosed       = "Closed"
)

// Add standard topics names also out here..

/*const (
	TopicEvents     = "BMCEvents"
	TopicCommands   = "BMCCommands"
	TopicCommandLog = "BMCCommandLog"
)*/
