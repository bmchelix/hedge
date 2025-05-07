/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

// the below structure used to fetch metric data from timeseries db
type MetricResult struct {
	Metric map[string]interface{} `json:"metric,omitempty" codec:"metric,omitempty"`
	Values []interface{}          `json:"value,omitempty" codec:"values,omitempty"`
}

type TimeSeriesData struct {
	ResultType string         `json:"resultType,omitempty" codec:"resultType,omitempty"`
	Result     []MetricResult `json:"result,omitempty" codec:"result,omitempty"`
}

type TimeSeriesResponse struct {
	Status string         `json:"status,omitempty" codec:"status,omitempty"`
	Data   TimeSeriesData `json:"data,omitempty" codec:"data,omitempty"`
}
