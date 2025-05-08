/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package models

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	ConsulKVServiceProtocolName  = "ProtocolName"
	ConsulKVServiceProtocolProps = "ProtocolProperties"
)

type DeviceServiceProtocols map[string]DeviceProtocols

type DeviceProtocols []DeviceProtocol

type DeviceProtocol struct {
	ProtocolName       string   `json:"protocolName"`
	ProtocolProperties []string `json:"protocolProperties"`
}

type ConsulKV struct {
	Key         string
	Value       string
	LockIndex   int
	Flags       int
	CreateIndex int
	ModifyIndex int
}

func ConsulKVtoDSProtocols(consulKVs []ConsulKV) (DeviceProtocols, error) {
	dps := DeviceProtocols{}

	for _, kv := range consulKVs {
		var dp DeviceProtocol
		var keypath = kv.Key // e.g. edgex/devices/2.0/device-virtual/Hedge/Protocols/0/ProtocolName
		ss := strings.Split(keypath, "/")

		keyName := ss[len(ss)-1]
		keyIndex, err := strconv.Atoi(ss[len(ss)-2])
		if err != nil {
			fmt.Println(
				"Failed while extracting the index (RAW: " + ss[len(ss)-2] + "). Error: " + err.Error(),
			)
			return dps, err
		}

		var keyValB64 = kv.Value
		keyValByte, err := base64.StdEncoding.DecodeString(keyValB64)
		if err != nil {
			fmt.Println(
				"Failed during Base64 decoding (RAW: " + keyValB64 + "). Error: " + err.Error(),
			)
			return dps, err
		}
		keyVal := string(keyValByte)

		if keyName == ConsulKVServiceProtocolName {
			dp.ProtocolName = keyVal
		} else if keyName == ConsulKVServiceProtocolProps {
			// trim array symbols '[]' and remove spaces around ',' separator
			dp.ProtocolProperties = strings.Split(strings.Trim(strings.ReplaceAll(keyVal, " ", ""), "[]"), ",")
		}

		if len(dps) < keyIndex+1 {
			dps = append(dps, dp)
		} else {
			if dp.ProtocolName != "" {
				dps[keyIndex].ProtocolName = dp.ProtocolName
			} else if dp.ProtocolProperties != nil {
				dps[keyIndex].ProtocolProperties = dp.ProtocolProperties
			}
		}
	}

	return dps, nil
}

func (dsps DeviceServiceProtocols) IsEmpty() bool {
	return reflect.DeepEqual(dsps, DeviceServiceProtocols{})
}
