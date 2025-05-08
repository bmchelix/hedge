package models

type PredictionDataType string

const (
	FLOAT   PredictionDataType = "float"
	INT     PredictionDataType = "int"
	STRING  PredictionDataType = "string"
	BOOLEAN PredictionDataType = "bool"
)

type MLPrediction struct {
	Id                   string                   `json:"id,omitempty"                    codec:"id,omitempty"`
	EntityType           string                   `json:"entity_type,omitempty"           codec:"entity_type,omitempty"`
	EntityName           string                   `json:"entity_name,omitempty"           codec:"entity_name,omitempty"`
	Devices              []string                 `json:"devices,omitempty"               codec:"devices,omitempty"`
	PredictionMessage    string                   `json:"prediction_message,omitempty"    codec:"prediction_message,omitempty"`
	MLAlgoName           string                   `json:"ml_algo_name,omitempty"          codec:"ml_algo_name,omitempty"`
	MLAlgorithmType      string                   `json:"ml_algorithm_type,omitempty"     codec:"ml_algorithm_type,omitempty"`
	ModelName            string                   `json:"model_name,omitempty"            codec:"model_name,omitempty"`
	Prediction           interface{}              `json:"prediction,omitempty"            codec:"prediction,omitempty"`
	PredictionThresholds map[string]interface{}   `json:"prediction_thresholds,omitempty" codec:"prediction_thresholds,omitempty"`
	InputDataByDevice    []map[string]interface{} `json:"input_data_by_device,omitempty"  codec:"input_data_by_device,omitempty"`
	Created              int64                    `json:"created,omitempty"               codec:"created,omitempty"`
	Modified             int64                    `json:"modified,omitempty"              codec:"modified,omitempty"`
	CorrelationId        string                   `json:"correlation_id,omitempty"        codec:"correlation_id,omitempty"`
	PredictionDataType   `json:"prediction_datatype,omitempty" codec:"prediction_datatype,omitempty"`
	PredictionName       string         `json:"prediction_name,omitempty"       codec:"prediction_name,omitempty"`
	Tags                 map[string]any `json:"tags,omitempty"                  codec:"tags,omitempty"`
}
