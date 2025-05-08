/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

type WritableConfig struct {
	Writable struct {
		StoreAndForward StoreAndForwardConfig `toml:"StoreAndForward"`
	} `toml:"Writable"`
}

type StoreAndForwardConfig struct {
	Enabled       bool   `toml:"Enabled"`
	RetryInterval string `toml:"RetryInterval"`
	MaxRetryCount int    `toml:"MaxRetryCount"`
}

func (a *StoreAndForwardConfig) UpdateFromRaw(rawConfig interface{}) bool {
	configuration, ok := rawConfig.(*StoreAndForwardConfig)
	if !ok {
		return false //errors.New("unable to cast raw config to type 'AppCustomConfig'")
	}

	*a = *configuration

	return true
}

func (w *WritableConfig) UpdateFromRaw(rawConfig interface{}) bool {
	configuration, ok := rawConfig.(*WritableConfig)
	if !ok {
		return false //errors.New("unable to cast raw config to type 'AppCustomConfig'")
	}

	*w = *configuration

	return true
}
