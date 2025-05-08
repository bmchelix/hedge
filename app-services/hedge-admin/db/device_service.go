/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package db

import (
	"strings"

	db2 "hedge/common/db"
	hedgeErrors "hedge/common/errors"
	models2 "hedge/common/models"
)

// Storage Structure
// md|ds|hx:ext:<servicename> ->  Stores protocol details for a service

func (dbClient *DBClient) SaveProtocols(
	dsps models2.DeviceServiceProtocols,
) ([]string, hedgeErrors.HedgeError) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	var dspKeys []string
	for dsName, dsProtocols := range dsps {
		dspKey := db2.DeviceServiceExt + ":" + dsName
		dbClient.Logger.Infof("Redis hash to be created at: %s", dspKey)

		for _, dsProtocol := range dsProtocols {
			err := conn.Send(
				"HSET",
				dspKey,
				dsProtocol.ProtocolName,
				"["+strings.Join(dsProtocol.ProtocolProperties, ",")+"]",
			)
			if err != nil {
				dbClient.Logger.Infof(
					"Error while saving Service Protocol in DB [Key=%s, Protocol=%s, Properties=%s]: %v",
					dspKey,
					dsProtocol.ProtocolName,
					dsProtocol.ProtocolProperties,
					err,
				)
				return dspKeys, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError,
					"Error saving protocol")
			}
		}
		dspKeys = append(dspKeys, dspKey)
	}

	return dspKeys, nil
}
