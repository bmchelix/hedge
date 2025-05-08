package config

type MLAlgorithmSummary struct {
	Name                  string `json:"name"` // Algo name
	Description           string `json:"description"`
	Type                  string `json:"type"`
	Enabled               bool   `json:"enabled"` //true,false
	ConfiguredModelCounts int64  `json:"configuredModelCounts"`
	IsOotb                bool   `json:"isOotb"`
	MLDigestsValid        bool   `json:"mlDigestsValid"`
	ErrorMessage          string `json:"errorMessage"`
}
