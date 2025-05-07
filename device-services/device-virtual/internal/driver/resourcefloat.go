/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package driver

import (
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"math/rand"
	"strconv"
	"time"

	dsModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
)

type resourceFloat struct{}

func (rf *resourceFloat) value(db *db, deviceName, deviceResourceName string, minimum, maximum, add, multiplier float64) (*dsModels.CommandValue, error) {

	result := &dsModels.CommandValue{}

	/*	enableRandomization, currentValue, dataType, err := db.getVirtualResourceData(deviceName, deviceResourceName)
		if err != nil {
			return result, err
		}
	*/
	//now := time.Now().UnixNano()
	//rand.Seed(time.Now().UnixNano())
	var newValueFloat float64
	//var bitSize int
	min, max := minimum, maximum
	var err error

	//result, err = dsModels.NewFloat64Value(deviceResourceName, now, newValueFloat*float64(multiplier) + add)
	newValueFloat = randomFloat(min, max)
	origin := time.Now().UnixNano()
	result, err = dsModels.NewCommandValueWithOrigin(deviceResourceName, common.ValueTypeFloat64, (newValueFloat+add)*multiplier, origin)

	if err != nil {
		return result, err
	}
	//err = db.updateResourceValue(strconv.FormatFloat(newValueFloat*float64(multiplier)+add, 'e', -1, bitSize), deviceName, deviceResourceName, false)
	//err = db.updateResourceValue(strconv.FormatFloat(newValueFloat+add, 'e', -1, bitSize), deviceName, deviceResourceName, false)
	return result, err
}

func randomFloat(min, max float64) float64 {
	rand.Seed(time.Now().UnixNano())
	if max > 0 && min < 0 {
		var negativePart float64
		var positivePart float64
		negativePart = rand.Float64() * min
		positivePart = rand.Float64() * max
		return negativePart + positivePart
	} else {
		return rand.Float64()*(max-min) + min
	}
}

/*func parseFloatMinimumMaximum(minimum, maximum, dataType string) (float64, float64, error) {
	var err, err1, err2 error
	var min, max float64
	switch dataType {
	case typeFloat32:
		min, err1 = datagenerator.ParseStrToFloat(minimum, 32)
		max, err2 = datagenerator.ParseStrToFloat(maximum, 32)
		if max <= min || err1 != nil || err2 != nil {
			err = fmt.Errorf("minimum:%s maximum:%s not in valid range, use default Value", minimum, maximum)
		}
	case typeFloat64:
		min, err1 = datagenerator.ParseStrToFloat(minimum, 64)
		max, err2 = datagenerator.ParseStrToFloat(maximum, 64)
		if max <= min || err1 != nil || err2 != nil {
			err = fmt.Errorf("minimum:%s maximum:%s not in valid range, use default Value", minimum, maximum)
		}
	}
	return min, max, err
}*/

func (rf *resourceFloat) write(param *dsModels.CommandValue, deviceName string, db *db) error {
	switch param.Type {
	case common.ValueTypeFloat32:
		if v, err := param.Float32Value(); err == nil {
			return db.updateResourceValue(strconv.FormatFloat(float64(v), 'e', -1, 32), deviceName, param.DeviceResourceName, true)
		} else {
			return fmt.Errorf("resourceFloat.write: %v", err)
		}
	case common.ValueTypeFloat64:
		if v, err := param.Float64Value(); err == nil {
			return db.updateResourceValue(strconv.FormatFloat(float64(v), 'e', -1, 64), deviceName, param.DeviceResourceName, true)
		} else {
			return fmt.Errorf("resourceFloat.write: %v", err)
		}
	default:
		return fmt.Errorf("resourceFloat.write: unknown device resource: %s", param.DeviceResourceName)
	}
}
