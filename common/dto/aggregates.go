package dto

type Metrics struct {
	IsCompressed bool        `json:"isCompressed" codec:"isCompressed"`
	MetricGroup  MetricGroup `json:"metricGroup" codec:"metricGroup"`
}

type Data struct {
	Name      string `json:"name" codec:"name"`
	TimeStamp int64  `json:"timeStamp" codec:"timeStamp"` // Timestamp in nanoseconds
	Value     string `json:"value" codec:"value"`
	ValueType string `json:"valueType" codec:"valueType"`
}

type MetricGroup struct {
	Tags    map[string]any `json:"tags" codec:"tags"`
	Samples []Data         `json:"samples" codec:"samples"`
}
