/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package driver

import (
	"hedge/device-services/device-virtual/internal/datagenerator"
	"crypto/rand"
	"errors"
	"fmt"
	dsModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"math/big"
)

const (
	typeBool    = "Bool"
	typeInt8    = "Int8"
	typeInt16   = "Int16"
	typeInt32   = "Int32"
	typeInt64   = "Int64"
	typeUint8   = "Uint8"
	typeUint16  = "Uint16"
	typeUint32  = "Uint32"
	typeUint64  = "Uint64"
	typeFloat32 = "Float32"
	typeFloat64 = "Float64"
	typeString  = "String"
	//typeBinary         = "Binary"

)

type virtualDevice struct {
	faultStatus   *faultStatus
	resourceUint  *resourceUint
	resourceFloat *resourceFloat
	resourceBool  *resourceBool
}

func (d *virtualDevice) read(deviceName, deviceResourceName, typeName string, minimum, maximum *float64, db *db) (*dsModels.CommandValue, error) {
	result := &dsModels.CommandValue{}

	// Get the right dataGen by device instance, faultStatus is the only exception since that is
	// a derived Value
	dataGen, dataGenFound := datagenerator.SelectDataGeneration(deviceName, deviceResourceName)

	if dataGenFound {
		// Mulitplier is used for those metrics whose Value depends on the interval ( eg callVolume)
		// We multiple per second data with the polling interval
		var multiplier float64 = 1.0
		var found bool = false
		if dataGen.UseMultiplier {
			multiplier, found = datagenerator.DeviceNameToAutoEventInterval[deviceName]
			if !found {
				multiplier = 1
			}
		}

		var add float64 = 0.0
		if dataGen.DependencySlopMap != nil {
			currentMin := dataGen.Min
			currentMax := dataGen.Max
			for dependentMetric, slope := range dataGen.DependencySlopMap {
				if dependantDataGen, found := datagenerator.DefaultsByMetric[dependentMetric]; found == true {
					// if instance one dataGenFound, use that
					if dataG, present := datagenerator.NewByDeviceAndMetric[deviceName+"#"+dependentMetric]; present {
						dependantDataGen = dataG
					}
					min := dependantDataGen.Min
					max := dependantDataGen.Max
					scale := (currentMax - currentMin) / (max - min)
					add = add + dependantDataGen.Average*slope*scale
					//add = add + (Max-Min)/2*Slope*scale
				}
			}
		}

		// Handle categorical data generation
		if dataGen.DataGeneratorName == "categorical" && len(dataGen.CategoricalValues) > 0 {
			index, err := rand.Int(rand.Reader, big.NewInt(int64(len(dataGen.CategoricalValues))))
			if err != nil {
				return nil, err
			}
			selectedValue := dataGen.CategoricalValues[index.Int64()]

			// Use dsModels.NewCommandValue for string type
			cv, err := dsModels.NewCommandValue(deviceResourceName, typeString, selectedValue)
			if err != nil {
				return nil, fmt.Errorf("failed to create CommandValue: %v", err)
			}
			return cv, nil
		} else if dataGen.DataGeneratorName == "uint" {
			return nil, errors.New("uint datatype for data-generation not implemented as yet")
		} else if dataGen.DataGeneratorName == "bool" {
			return d.resourceBool.value(db, deviceName, deviceResourceName, dataGen.Value)
		} else if dataGen.DataGeneratorName == "float" {
			val, er := d.resourceFloat.value(db, deviceName, deviceResourceName, dataGen.Min, dataGen.Max, add, multiplier)
			return val, er
		}
	}

	if deviceResourceName == "FaultStatus" {
		cv, err := d.faultStatus.value(db, deviceName, deviceResourceName)
		if err == nil {
			cv.Tags = map[string]string{
				"EventMetrics": "FaultStatus",
			}
		}
		return cv, err
	}

	// No datagen or special handling found, so error out
	return result, fmt.Errorf("virtualDevice.read: wrong read type: %s", deviceResourceName)

}

func (d *virtualDevice) handleWrite(metricNameValues []*dsModels.CommandValue, deviceName string) error {
	updateDeviceDataGenConfiguration(metricNameValues, deviceName)
	return nil
}

// Not planned for use as of now since there is a generic API that is exposed in data generator
func updateDeviceDataGenConfiguration(metricNameValues []*dsModels.CommandValue, deviceName string) {
	var metricName string = ""
	var paramNameValue *dsModels.CommandValue
	for _, metricNameValuePair := range metricNameValues {
		name := metricNameValuePair.DeviceResourceName
		if name == "metricName" {
			metricName, _ = metricNameValuePair.StringValue()
		} else {
			paramNameValue = metricNameValuePair
			paramNameValue.DeviceResourceName = metricName
		}
	}
	var newMinMaxAvg float64 = 0
	if paramNameValue != nil {
		newMinMaxAvg, _ = paramNameValue.Float64Value()
	}
	datagenerator.UpdateDataGenConfiguration(deviceName, metricName, newMinMaxAvg)
}

func newVirtualDevice() *virtualDevice {

	return &virtualDevice{
		faultStatus:   &faultStatus{},
		resourceUint:  &resourceUint{},
		resourceBool:  &resourceBool{},
		resourceFloat: &resourceFloat{},
	}
}
