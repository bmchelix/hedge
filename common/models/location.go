/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

import (
	"fmt"
	"strconv"
)

type Location struct {
	Id            string  `json:"id"`
	DisplayName   string  `json:"displayName,omitempty"   codec:"displayName,omitempty"`
	Country       string  `json:"country,omitempty"       codec:"displayName,omitempty"`
	State         string  `json:"state,omitempty"         codec:"country,omitempty"`
	City          string  `json:"city,omitempty"          codec:"city,omitempty"`
	StreetAddress string  `json:"streetAddress,omitempty" codec:"streetAddress,omitempty"`
	ZipCode       string  `json:"zipCode,omitempty"       codec:"zipCode,omitempty"`
	Latitude      float64 `json:"latitude,omitempty"      codec:"latitude,omitempty"`
	Longitude     float64 `json:"longitude,omitempty"     codec:"longitude,omitempty"`
}

func ConvertLocation(iLocation interface{}, deviceName string) (location Location) {
	if loc, ok := iLocation.(map[string]interface{}); !ok {
		fmt.Printf("Error: Invalid loc for %s: %s. Ignore parsing..", deviceName, iLocation)
		location.Id = iLocation.(string)
	} else {
		if loc["id"] != nil {
			location.Id = loc["id"].(string)
		}
		if loc["displayName"] != nil {
			location.DisplayName = loc["displayName"].(string)
		}
		if loc["country"] != nil {
			location.Country = loc["country"].(string)
		}
		if loc["state"] != nil {
			location.State = loc["state"].(string)
		}
		if loc["city"] != nil {
			location.City = loc["city"].(string)
		}
		if loc["streetAddress"] != nil {
			location.StreetAddress = loc["streetAddress"].(string)
		}
		if loc["zipCode"] != nil {
			location.ZipCode = loc["zipCode"].(string)
		}
		if loc["latitude"] != nil {
			location.Latitude, _ = strconv.ParseFloat(loc["latitude"].(string), 64)
		}
		if loc["longitude"] != nil {
			location.Longitude, _ = strconv.ParseFloat(loc["longitude"].(string), 64)
		}
	}

	return location
}
