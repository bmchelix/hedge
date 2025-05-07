/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package driver

import (
	dsModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
)

type faultStatus struct{}

var controlCallDropPercentage int16 = 10
var controlCallDropIntervals int16 = 30

// Initial setting about 1/10th of the call drops
var controlFaultsPercentage int16 = controlCallDropPercentage / 10
var controlFaultsTrendSalt int16 = 5
var controlFaultsIntervals int16 = int16(controlCallDropIntervals * 100)
var totalFaultCount int16 = 0

func (rb *faultStatus) value(db *db, deviceName, deviceResourceName string) (*dsModels.CommandValue, error) {
	//result := &dsModels.CommandValue{}
	var newValueBool bool

	// Compute the interval after which we generate a fault
	// Faults distributed uniformly across the time interval for now
	// Will need to update for random distribution
	var faultCountInterval float32 = 0
	if controlCallDropPercentage > 0 {
		faultCountInterval = 100.0 / float32(controlFaultsPercentage)
	}
	if faultCountInterval >= 1.0 && totalFaultCount%(int16(faultCountInterval)) == 0 {
		newValueBool = true
	} else {
		newValueBool = false
	}
	totalFaultCount++
	if totalFaultCount >= controlFaultsIntervals {
		totalFaultCount = 0
	}

	//now := time.Now().UnixNano()
	//var err error
	return dsModels.NewCommandValue(deviceResourceName, common.ValueTypeBool, newValueBool)
	//return result, nil
}
