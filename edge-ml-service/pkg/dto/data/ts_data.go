/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package data

type TSDataElement struct {
	Profile string
	// Actual Features of type MetaData that will be used for training/inferencing as well as for grouping in case of multiple profiles
	MetaData        map[string]string
	Timestamp       int64
	MetricName      string
	Value           interface{}
	GroupByMetaData map[string]string
	DeviceName      string
	// To create Anomaly event, we need to have context data when generating event, so keep a copy of the tags from readings
	Tags map[string]interface{}
}

// Data structure to hold the data to pass it onto inference, but then keep a context so we can create event with right context
type InferenceData struct {
	GroupName  string
	Devices    []string
	TimeStamp  int64
	Data       [][]interface{}
	OutputData map[string]interface{}
	Tags       map[string]interface{}
}
