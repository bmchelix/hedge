/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package driver

import (
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"math/rand"
	"strconv"
	"time"

	dsModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
)

type resourceBool struct{}

func (rb *resourceBool) value(db *db, deviceName, deviceResourceName, value string) (*dsModels.CommandValue, error) {
	result := &dsModels.CommandValue{}
	var newValueBool bool
	newValueBool, err := strconv.ParseBool(value)
	// If constant boolean value defined, go with that - no need to change it
	if err != nil {
		enableRandomization, currentValue, _, err := db.getVirtualResourceData(deviceName, deviceResourceName)
		if err != nil {
			return result, err
		}

		//if dataGeneratorByMetricName[]
		if enableRandomization {
			rand.Seed(time.Now().UnixNano())
			newValueBool = rand.Int()%2 == 0
		} else {
			if newValueBool, err = strconv.ParseBool(currentValue); err != nil {
				return result, err
			}
		}
	}

	result, err = dsModels.NewCommandValue(deviceResourceName, common.ValueTypeBool, newValueBool)
	if err != nil {
		return result, err
	}
	if err := db.updateResourceValue(result.ValueToString(), deviceName, deviceResourceName, false); err != nil {
		return result, err
	}

	return result, nil
}
