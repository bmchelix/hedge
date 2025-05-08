package twin

import "hedge/edge-ml-service/pkg/dto/config"

type DigitalTwinDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	//
	// New approach is to define simulation as MLModelDefinition, this should ease training and predictions
	//
	MLModelConfig config.MLModelConfig `json:"mlModelConfig"`
	// Connections can be kept in here if the ML training and prediction for each connected
	// twin is separate
	Connections []Connection `json:"connections,omitempty"`
	Status      StatusType   `json:"status,omitempty"`
	// Add reference to scenes in here, 1 or more of them can be referred in here
	//
	// Entities and ExternalParameters should be removed
	//
	Entities           []Entity            `json:"entities,omitempty"`
	ExternalParameters []ExternalParameter `json:"externalParameters,omitempty"`
}

// EntityType can be Device or it can be another Twin
type EntityType int

const (
	Device EntityType = iota
	Twin
)

type Entity struct {
	EntityType EntityType `json:"entityType"`
	Name       string     `json:"name"`
}

type Connection struct {
	FromTwin       string         `json:"fromTwin,omitempty"`
	TargetTwin     string         `json:"targetTwin,omitempty"`
	ConnectionType ConnectionType `json:"connectionType"`
	// FromTwin 2 TargetTwin Parameter Map
	Parameters map[string]string `json:"parameters,omitempty"`
}

// Define Connection Type
type ConnectionType int

const (
	ConnectionTypeHierarchy ConnectionType = iota
	ConnectionTypeDirect
)

// Define Status type
type StatusType int

const (
	StatusTypeEnabled StatusType = iota
	StatusTypeDisabled
)

type RunType int

const (
	PeriodicRun RunType = iota
	OneTimeRun
)

type RunControl int

const (
	UserDriven RunControl = iota
	RealTime
)

type SimulationDefinition struct {
	Name                 string              `json:"name"`
	TwinDefinitionName   string              `json:"twinDefinitionName"`
	SimulationParameters []ExternalParameter `json:"simulationParameters"`
	RunType              `json:"runType,omitempty"`
	RunControl           `json:"runControl,omitempty"`
}

type RunStatus int

const (
	Running RunStatus = iota
	Done
)

type SimulationRun struct {
	ID                   string    `json:"id"`
	Date                 int64     `json:"date"`
	Status               RunStatus `json:"status"`
	SimulationDefinition `json:"simulationDefinition"`
}

type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type ExternalParameter struct {
	Type string `json:"type" default:"METRIC"` // METRIC, METADATA
	Name string `json:"name"`
	//Units              string `json:"units,omitempty"`
	IsInput bool  `json:"isInput"`
	Range   Range `json:"range,omitempty"`
}
