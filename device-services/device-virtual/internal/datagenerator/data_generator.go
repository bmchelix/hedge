/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package datagenerator

import (
	"crypto/rand"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const UninitializedValue = 9999999

type Configuration struct {
	DataGeneratorName    string // Name of the data generator implementation eg float, uint, bool
	ReferenceDataGenName string // reference data generator against which the % to be computed
	DefaultAvgValue      float64
	UseMultiplier        bool     //true/false
	Min                  float64  // string // Per sec Min
	Max                  float64  // string // Per sec Max
	Average              float64  // Current Average, applies to non-bool types
	Value                string   // applicable for boolean or any other dataGenerator that needs to generate constant Value
	CategoricalValues    []string // for string values
	DependsOn            string
	DependencySlopMap    map[string]float64
}

type ToggleDurationConfiguration struct {
	ConfiguredToggleDurationInSecs int64
	StartTime                      int64
	IsDefaultDataGen               bool
}

// Contains a map of deviceResource/metric->Configuration with its Parameters
// This config is picked from profile yaml file for data-generation
var DefaultsByMetric map[string]*Configuration = make(map[string]*Configuration)

// To control metric values per device, we need a new structure by device+metric
// We make a copy of the DefaultsByMetric first time when the device instance metric is being read
var NewByDeviceAndMetric map[string]*Configuration

var NewDeviceMetricToToggleInterval map[string]ToggleDurationConfiguration = make(map[string]ToggleDurationConfiguration)

var DeviceNameToAutoEventInterval map[string]float64 = make(map[string]float64)

//var TelcoCallDropImpactedStateLookup map[string]float64 = make(map[string]float64)

func UpdateDataGenParameters(min float64, dataGen *Configuration, max float64) {
	if min == UninitializedValue {
		min = dataGen.Min
	}
	if max == UninitializedValue {
		max = dataGen.Max
	}
	dataGen.Min = min
	dataGen.Max = max
	dataGen.Average = (min + max) / 2
}

func BuildDeviceToAutoIntervalMap(device models.Device, deviceName string) {
	if len(device.AutoEvents) > 0 {
		frequencyWithUnit := device.AutoEvents[0].Interval
		if strings.HasSuffix(frequencyWithUnit, "s") {
			frequency := strings.Split(frequencyWithUnit, "s")
			scheduledInterval, err := strconv.ParseUint(frequency[0], 10, 64)
			if err == nil {
				DeviceNameToAutoEventInterval[deviceName] = float64(scheduledInterval)
			}
		}
	}
}

// Build Default datagenerator based on inputs supplied in the form of attribites in the profile definition
// Inputs : Resource ie the metric details that includes attributes and the name of the metric
func BuildDataGeneratorFromAttributes(dr models.DeviceResource, metricName string) {
	dataGeneratorIntfc, found := dr.Attributes["dataGenerator"]
	if found {
		dataGenerator := dataGeneratorIntfc.(string)
		var min, max, average float64 = UninitializedValue, UninitializedValue, UninitializedValue
		categoricalValues := []string{}

		if dr.Attributes["min"] != nil && dr.Attributes["max"] != nil {
			min, _ = strconv.ParseFloat(dr.Attributes["min"].(string), 64)
			max, _ = strconv.ParseFloat(dr.Attributes["max"].(string), 64)
			average = (min + max) / 2
		}

		useMultiplier := false
		if dr.Attributes["useMultiplier"] == "true" {
			useMultiplier = true
		}
		// Parse categoricalValues if present
		if dr.Attributes["categoricalValues"] != nil {
			valuesStr := dr.Attributes["categoricalValues"].(string)
			categoricalValues = strings.Split(valuesStr, ",")
		}

		val := ""
		if dr.Attributes["value"] != nil {
			val = dr.Attributes["value"].(string)
		}
		dataGenConfig := &Configuration{
			DataGeneratorName: dataGenerator,
			UseMultiplier:     useMultiplier,
			DefaultAvgValue:   average,
			Min:               min, //dr.Attributes["Min"],
			Max:               max, // dr.Attributes["Max"],
			Average:           average,
			Value:             val,
			//DependsOn:         dr.Attributes["dependsOn"],
			CategoricalValues: categoricalValues,
			DependencySlopMap: parseDependencySlopes(dr),
		}
		DefaultsByMetric[metricName] = dataGenConfig
	}
}

func UpdateDataGenConfiguration(deviceName string, metricName string, newMinMaxAvg float64) {
	UpdateDataGenConfigurationMinMax(deviceName, metricName, newMinMaxAvg, UninitializedValue, UninitializedValue)
}

func UpdateDataGenConfigurationMinMax(deviceName string, metricName string, newMinMaxAvg float64, min float64, max float64) {
	lookupKey := buildDeviceMetricLookup(deviceName, metricName)

	if NewByDeviceAndMetric == nil {
		NewByDeviceAndMetric = make(map[string]*Configuration, 25)
	}

	if _, ok := NewByDeviceAndMetric[lookupKey]; !ok {
		// Copy from initial definition

		dataGenFromProfile := DefaultsByMetric[metricName]

		if dataGenFromProfile != nil {
			NewByDeviceAndMetric[lookupKey] = &Configuration{
				DataGeneratorName:    dataGenFromProfile.DataGeneratorName,
				ReferenceDataGenName: dataGenFromProfile.ReferenceDataGenName,
				Average:              dataGenFromProfile.DefaultAvgValue,
				DefaultAvgValue:      dataGenFromProfile.DefaultAvgValue,
				UseMultiplier:        dataGenFromProfile.UseMultiplier,
				Min:                  dataGenFromProfile.Min,
				Max:                  dataGenFromProfile.Max,
				Value:                dataGenFromProfile.Value,
				DependsOn:            dataGenFromProfile.DependsOn,
				DependencySlopMap:    dataGenFromProfile.DependencySlopMap,
			}
		} else {
			//Device or Data generator for device not found, so ignore updates
		}
	}

	// There is a copy in against the deviceName+metric now, just update with new Value now

	dataGenForDevice := NewByDeviceAndMetric[lookupKey]

	if dataGenForDevice == nil {
		//Nothing to update since the device either doesn't exist or there is no datagenerator for it
		return
	}

	if min == UninitializedValue || max == UninitializedValue {
		min, max = getNewMinMax(newMinMaxAvg, dataGenForDevice, metricName)
	}

	UpdateDataGenParameters(min, dataGenForDevice, max)
}

func updateBoolValueForDataGen(device string, metric string, boolValue string) {
	lookupKey := buildDeviceMetricLookup(device, metric)
	if NewByDeviceAndMetric == nil {
		NewByDeviceAndMetric = make(map[string]*Configuration, 25)
	}

	if _, ok := NewByDeviceAndMetric[lookupKey]; !ok {
		// Copy from initial definition
		dataGenFromProfile := DefaultsByMetric[metric]
		if dataGenFromProfile != nil {
			NewByDeviceAndMetric[lookupKey] = &Configuration{
				DataGeneratorName:    dataGenFromProfile.DataGeneratorName,
				ReferenceDataGenName: dataGenFromProfile.ReferenceDataGenName,
				UseMultiplier:        dataGenFromProfile.UseMultiplier,
				Value:                dataGenFromProfile.Value,
			}
		} else {
			//Device or Data generator for device not found, so ignore updates
		}
	}

	// There is a copy in against the deviceName+metric now, just update with new Value now
	dataGenForDevice := NewByDeviceAndMetric[lookupKey]

	if dataGenForDevice == nil {
		//Nothing to update since the device either doesn't exist or there is no datagenerator for it
		return
	}

	dataGenForDevice.Value = boolValue
}

func updateToggleIntervalForDataGen(device string, metric string, toggleIntervalInMinutes int64) {
	lookupKey := buildDeviceMetricLookup(device, metric)

	toggerConfig := ToggleDurationConfiguration{
		ConfiguredToggleDurationInSecs: toggleIntervalInMinutes * 60,
		StartTime:                      time.Now().Unix(),
		IsDefaultDataGen:               false,
	}
	NewDeviceMetricToToggleInterval[lookupKey] = toggerConfig
}

func getNewMinMax(newMinMaxAvg float64, dataGen *Configuration, metricName string) (float64, float64) {

	// First get the default values of max and min as per profile since
	// all measurements are based on default/base values

	defaultDataGen := DefaultsByMetric[metricName]

	interval := defaultDataGen.Max - defaultDataGen.Min

	var min float64
	var max float64

	delta := newMinMaxAvg - defaultDataGen.DefaultAvgValue
	//percentChg = delta /dataGen.DefaultAvgValue*booster
	min = defaultDataGen.Min + delta
	if min < 0 {
		min = 0.0
	}
	max = min + interval
	return min, max
}

func ParseStrToFloat(str string, bitSize int) (float64, error) {
	if f, err := strconv.ParseFloat(str, bitSize); err != nil {
		return f, err
	} else {
		return f, nil
	}
}

func SelectDataGeneration(device string, metric string) (Configuration, bool) {
	var dataGen Configuration
	var dataGenFound bool = false

	// Calculate lookupKey once and reuse it
	lookupKey := buildDeviceMetricLookup(device, metric)

	if NewByDeviceAndMetric != nil {
		if dG, found := NewByDeviceAndMetric[lookupKey]; found {
			dataGen = *dG
			dataGenFound = true

			// Check the elapsed time for toggle & toggle if expired
			if toggleCfg, found := NewDeviceMetricToToggleInterval[lookupKey]; found {
				elapsedTime := time.Now().Unix() - toggleCfg.StartTime
				if elapsedTime > toggleCfg.ConfiguredToggleDurationInSecs {
					toggleCfg.IsDefaultDataGen = !toggleCfg.IsDefaultDataGen
					toggleCfg.StartTime = time.Now().Unix()
					NewDeviceMetricToToggleInterval[lookupKey] = toggleCfg
				}
				if toggleCfg.IsDefaultDataGen {
					dataGen = *DefaultsByMetric[metric]
				}
			}
		}
	}

	// If dataGen not found in NewByDeviceAndMetric, fall back to DefaultsByMetric
	if !dataGenFound {
		if defaultGen, found := DefaultsByMetric[metric]; found {
			dataGen = *defaultGen
			dataGenFound = true
		}
	}

	// Handle CategoricalValues if present
	if dataGenFound && len(dataGen.CategoricalValues) > 0 {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(dataGen.CategoricalValues))))
		if err == nil {
			dataGen.Value = dataGen.CategoricalValues[index.Int64()]
		}
	} else if dataGenFound {
		// Handle numerical value generation (e.g., using average, min, and max)
		dataGen.Value = fmt.Sprintf("%f", dataGen.Average)
	}

	return dataGen, dataGenFound
}

func buildDeviceMetricLookup(device string, metric string) string {
	return device + "#" + metric
}

func parseDependencySlopes(dr models.DeviceResource) map[string]float64 {
	dependencyToSlopeMap := make(map[string]float64)
	dependencyList := dr.Attributes["dependsOn"]
	slopesList := dr.Attributes["slope"]

	if dependencyList != nil && slopesList != nil {
		dependencyTokens := strings.Split(dependencyList.(string), ",")
		slopesTokens := strings.Split(slopesList.(string), ",")

		for i := 0; i < len(dependencyTokens) && i < len(slopesTokens); i++ {
			slope, err := strconv.ParseFloat(slopesTokens[i], 64)
			if err == nil {
				dependencyToSlopeMap[dependencyTokens[i]] = slope
			}
		}
	}

	return dependencyToSlopeMap
}
