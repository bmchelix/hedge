/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package util

import (
	"hedge/common/client"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"encoding/json"
	"fmt"
	"net/http"
)

var HttpClient client.HTTPClient

func init() {
	if HttpClient == nil {
		HttpClient = &http.Client{}
	}
}

func GetTimeSeriesResponse(dbUrl string, baseQuery string) (dto.TimeSeriesResponse, hedgeErrors.HedgeError) {
	var timeSeriesResponse dto.TimeSeriesResponse

	errorMessage := fmt.Sprintf("Error getting time series response")

	iotUrl := dbUrl + "/api/v1/query"
	request, err := http.NewRequest("GET", iotUrl, nil)
	if err != nil {
		return timeSeriesResponse, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	request.Header.Set("Content-type", "application/json")

	q := request.URL.Query()
	if len(baseQuery) == 0 {
		return timeSeriesResponse, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("%s: %s", errorMessage, "Error with Training data, training job will be skipped"))
	}
	q.Add("query", baseQuery) // Add a new value to the set
	// Add time range and steps

	request.URL.RawQuery = q.Encode() // Encode and assign back to the original query.

	resp, err := HttpClient.Do(request)
	if err != nil {
		return timeSeriesResponse, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&timeSeriesResponse)
	if err != nil || timeSeriesResponse.Status == "error" {
		return timeSeriesResponse, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	return timeSeriesResponse, nil

}
