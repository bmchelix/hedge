/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package connector

import (
	"hedge/app-services/hedge-device-extensions/internal/util"
	"strconv"
	"strings"
)

func GetEventCountByDevices(victoriaUrl string, devices []string, edgeNode string) (map[string]int64, error) {
	var eventMap map[string]int64
	query := buildEventQuery(devices, edgeNode)

	timeSeriesResponse, err := util.GetTimeSeriesResponse(victoriaUrl, query)
	if err != nil {
		return eventMap, err
	}
	eventMap = make(map[string]int64, len(timeSeriesResponse.Data.Result))

	for _, sample := range timeSeriesResponse.Data.Result {
		deviceName := sample.Metric["device"].(string)
		countStr := sample.Values[1].(string)
		count, _ := strconv.ParseInt(countStr, 10, 64)
		eventMap[deviceName] = count
	}
	return eventMap, nil
}

func buildEventQuery(devices []string, edgeNode string) string {
	var query string

	query = "sum by(device) (IoTEvent{"

	if len(edgeNode) > 0 {
		query += "edgeNode=\"" + edgeNode + "\","
	}

	if len(devices) > 0 {
		query += "device=~\"" + strings.Join(devices, "|") + "\""
	}
	query += "}[2w])"

	return query
}
