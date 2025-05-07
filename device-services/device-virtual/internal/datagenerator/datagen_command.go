/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package datagenerator

import (
	"errors"
	"fmt"
	"time"
)

type Command struct {
	Name       string            `json:"name" codec:"name"`
	Parameters map[string]string `json:"parameters" codec:"parameters"`
}

func ExecuteCommands(commands []Command) error {
	// First do a trial run to validate the input data
	for _, command := range commands {
		err := executeCommand(command, true)
		if err != nil {
			return err
		}
	}

	// Actual execution this time
	go func() {
		for _, command := range commands {
			// We should also take a lock in here
			err := executeCommand(command, false)
			if err != nil {
				fmt.Errorf("error executing the commands: %v", err)
			}
		}
	}()

	return nil
}

func executeCommand(command Command, trialRun bool) error {
	fmt.Printf("%v\n", command)
	parameters := command.Parameters

	// Check Command validity then lock and update
	device := parameters["device"]
	metric := parameters["metric"]
	/*	if !trialRun && command.Name != "Wait" {
		locker.Lock()
		defer func() {
			locker.Unlock()
		}()
	}*/

	switch command.Name {
	case "ChangeAverage":
		if len(DefaultsByMetric[metric].CategoricalValues) != 0 {
			return errors.New("ChangeAverage is not applicable for categorical data")
		}
		if newAverage, found := parameters["average"]; found {
			newMinMaxAvg, err := ParseStrToFloat(newAverage, 64)
			if err != nil {
				return err
			}
			err = validateDeviceAndMetric(device, metric)
			if err != nil {
				return err
			}
			if !trialRun {
				UpdateDataGenConfiguration(device, metric, newMinMaxAvg)
			}
		} else {
			return errors.New("parameter average not found for ChangeAverage Command")
		}
	case "ChangeMinMax":
		if len(DefaultsByMetric[metric].CategoricalValues) != 0 {
			return errors.New("ChangeMinMax is not applicable for categorical data")
		}
		min, max, err := getMinMaxParameters(parameters)
		if err != nil {
			return err
		}
		if !trialRun {
			UpdateDataGenConfigurationMinMax(device, metric, (min+max)/2, min, max)
		}
	case "ChangeBoolValue":
		boolValue := parameters["value"]
		if boolValue == "" {
			return errors.New("Empty bool value")
		}
		if !trialRun {
			updateBoolValueForDataGen(device, metric, boolValue)
		}
	case "ChangeValue":
		newValue := parameters["value"]
		if newValue == "" {
			return errors.New("parameter value not found for ChangeValue Command")
		}
		if len(DefaultsByMetric[metric].CategoricalValues) != 0 {
			return errors.New("ChangeValue is only applicable for categorical data")
		}
		err := validateDeviceAndMetric(device, metric)
		if err != nil {
			return err
		}
		if !trialRun {
			updateCategoricalValueForDataGen(device, metric, newValue)
		}
	case "Wait":
		//var waitDurationInSeconds int64
		//waitDurationInSeconds, _ := strconv.ParseInt(parameters["waitDurationInSeconds"],10, 64)
		duration, err := time.ParseDuration(parameters["waitDuration"]) // input of form 20s, 2m etc
		if err == nil && !trialRun {
			time.Sleep(duration)
		} else {
			return err
		}

	case "ResetValue":
		err := validateDeviceAndMetric(device, metric)
		if err != nil {
			return err
		}
		if !trialRun {
			resetDataGenValue(device, metric)
		}
	case "ChangeDefault":
		if len(DefaultsByMetric[metric].CategoricalValues) != 0 {
			return errors.New("ChangeDefault is not applicable for categorical data")
		}
		err := validateDeviceAndMetric(device, metric)
		if err != nil {
			return err
		}
		min, max, err := getMinMaxParameters(parameters)
		if err != nil {
			return err
		}
		if !trialRun {
			changeDefault(device, metric, min, max)
		}

	case "RepeatDefaultAndNewValues":
		duration, err := time.ParseDuration(parameters["repeatInterval"])
		if err == nil {
			repeatInterval := int64(duration.Seconds())
			average, err := ParseStrToFloat(parameters["average"], 64)
			if err != nil {
				return err
			}
			if !trialRun {
				UpdateDataGenConfiguration(device, metric, average)
				updateToggleIntervalForDataGen(device, metric, repeatInterval)
			}
		}
	default:
		return fmt.Errorf("unsupported command %s", command.Name)
	}
	return nil
}

func getMinMaxParameters(parameters map[string]string) (float64, float64, error) {
	min, err := ParseStrToFloat(parameters["min"], 64)
	if err != nil {
		return 0, 0, err
	}
	max, err := ParseStrToFloat(parameters["max"], 64)
	if err != nil {
		return 0, 0, err
	}
	return min, max, nil
}

func validateDeviceAndMetric(device string, metric string) error {
	if len(device) < 1 || len(metric) < 1 {
		return errors.New("device or metric parameters cannot be empty")
	}
	// Validate that device & metric exist
	if DefaultsByMetric[metric] == nil {
		return errors.New("No data generator found for metric:" + metric)
	}
	return nil
}

func changeDefault(device string, metric string, min float64, max float64) {
	defaultConfigByMetric := DefaultsByMetric[metric]
	defaultConfigByMetric.Max = max
	defaultConfigByMetric.Min = min
	defaultConfigByMetric.DefaultAvgValue = (max + min) / 2.0
}

func resetDataGenValue(device string, metric string) error {
	lookupKey := buildDeviceMetricLookup(device, metric)
	delete(NewDeviceMetricToToggleInterval, lookupKey)
	delete(NewByDeviceAndMetric, lookupKey)
	return nil
}

func updateCategoricalValueForDataGen(device string, metric string, newValue string) {
	lookupKey := buildDeviceMetricLookup(device, metric)
	if NewByDeviceAndMetric == nil {
		NewByDeviceAndMetric = make(map[string]*Configuration, 25)
	}

	if _, ok := NewByDeviceAndMetric[lookupKey]; !ok {
		dataGenFromProfile := DefaultsByMetric[metric]
		if dataGenFromProfile != nil {
			NewByDeviceAndMetric[lookupKey] = &Configuration{
				DataGeneratorName: dataGenFromProfile.DataGeneratorName,
				CategoricalValues: dataGenFromProfile.CategoricalValues,
				Value:             dataGenFromProfile.Value,
			}
		}
	}

	dataGenForDevice := NewByDeviceAndMetric[lookupKey]

	if dataGenForDevice == nil {
		return
	}

	dataGenForDevice.Value = newValue
}
