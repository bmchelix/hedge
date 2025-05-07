// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2019-2020 IOTech Ltd
// (c) Copyright 2020-2025 BMC Software, Inc.
//
// Contributors: BMC Software, Inc. - BMC Helix Edge
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"hedge/device-services/device-virtual/internal/config"
	"hedge/device-services/device-virtual/internal/datagenerator"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces"
	dsModels "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/labstack/echo/v4"
	"io"

	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	_ "modernc.org/ql/driver"
)

type VirtualDriver struct {
	sdk                         interfaces.DeviceServiceSDK
	lc                          logger.LoggingClient
	asyncCh                     chan<- *dsModels.AsyncValues
	deviceCh                    chan<- []dsModels.DiscoveredDevice
	virtualDevicesServiceConfig *config.VirtualDeviceConfiguration
	virtualDevices              sync.Map
	db                          *db
	locker                      sync.Mutex
}

var once sync.Once
var driver *VirtualDriver

func NewVirtualDriver() interfaces.ProtocolDriver {
	once.Do(func() {
		driver = new(VirtualDriver)
	})
	return driver
}

func (d *VirtualDriver) Initialize(sdk interfaces.DeviceServiceSDK) error {
	d.sdk = sdk
	d.lc = sdk.LoggingClient()
	d.asyncCh = sdk.AsyncValuesChannel()

	d.deviceCh = sdk.DiscoveredDeviceChannel()
	d.virtualDevicesServiceConfig = &config.VirtualDeviceConfiguration{}
	d.db = getDb()

	if err := d.db.openDb(); err != nil {
		d.lc.Info(fmt.Sprintf("Create db connection failed: %v", err))
		return err
	}

	if err := initResourceTable(d); err != nil {
		return fmt.Errorf("initial virtual resource table failed: %v", err)
	}

	if err := sdk.LoadCustomConfig(d.virtualDevicesServiceConfig, "Hedge"); err != nil {
		return fmt.Errorf("unable to load 'Hedge' custom configuration: %s", err.Error())
	}

	devices := sdk.Devices()
	for _, device := range devices {
		err := prepareResources(d, device.Name)
		if err != nil {
			return fmt.Errorf("prepare virtual resources failed: %v", err)
		}
	}

	return nil
}

func (d *VirtualDriver) Discover() error {
	d.lc.Infof("Discovering new Virtual Device")
	proto := make(map[string]models.ProtocolProperties)
	proto["other"] = map[string]any{"Address": "wind-turbine-09", "Port": "309"}

	// Generate the device name...
	d.locker.Lock()
	defer func() {
		driver.locker.Unlock()
	}()

	windTowerName, err := getNextDeviceName("Dis_Alta_")
	if err != nil {
		d.lc.Errorf("Error getting next deviceName during discovery: %v", err)
		return err
	}

	discoveredDevice := dsModels.DiscoveredDevice{
		Name:        windTowerName,
		Protocols:   proto,
		Description: "Wind Tower discovered:" + windTowerName,
		Labels:      []string{"LoadTest", "Discovery", "WindTower", "Energy"},
	}

	res := []dsModels.DiscoveredDevice{discoveredDevice}

	d.lc.Infof("Discovery function discovered a new virtual device. will be added if this passes provision watcher rules: %v\n", res)
	//time.Sleep(time.Duration(d.serviceConfig.SimpleCustom.Writable.DiscoverSleepDurationSecs) * time.Second)
	d.deviceCh <- res

	return nil
}

func (d *VirtualDriver) Start() error {
	d.addRoute()
	return nil
}

func (d *VirtualDriver) ValidateDevice(device models.Device) error {
	return nil
}

func (d *VirtualDriver) retrieveDevice(deviceName string) (vdv *virtualDevice, err error) {
	vd, ok := d.virtualDevices.LoadOrStore(deviceName, newVirtualDevice())
	if vdv, ok = vd.(*virtualDevice); !ok {
		err = fmt.Errorf("retrieve virtualDevice by name: %s, the returned Value has to be a reference of "+
			"virtualDevice struct, but got: %s", deviceName, reflect.TypeOf(vd))
		d.lc.Error(err.Error())
	}
	return vdv, err
}

func (d *VirtualDriver) addRoute() {
	d.addDataGenerationUpdateRoute()
	d.addRestDataIngestionRoute()
}

func (d *VirtualDriver) handleCommandExecution(c echo.Context) error {
	// ExecuteCommands all Commands one by one and return status
	// Read the body json
	bytes, err := io.ReadAll(c.Request().Body)
	if err == nil {
		var commands []datagenerator.Command
		err = json.Unmarshal(bytes, &commands)
		d.lc.Info(fmt.Sprintf("commands received: %v", commands))
		// For each command, take the lock, execute the command one by one
		// ExecuteCommands wait command without a lock
		err = datagenerator.ExecuteCommands(commands)
	}
	if err != nil {
		http.Error(c.Response(), err.Error(), http.StatusBadRequest)
		d.lc.Error(err.Error())
		return err
	}
	c.Response().WriteHeader(http.StatusAccepted)
	return nil
}

func (d *VirtualDriver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest) (res []*dsModels.CommandValue, err error) {
	d.locker.Lock()
	defer func() {
		d.locker.Unlock()
	}()

	vd, err := d.retrieveDevice(deviceName)
	if err != nil {
		return nil, err
	}

	res = make([]*dsModels.CommandValue, len(reqs))

	for i, req := range reqs {
		if dr, ok := d.sdk.DeviceResource(deviceName, req.DeviceResourceName); ok {
			if strings.Contains(deviceName, "Bool") {
				fmt.Errorf("boolean device")
			}
			if v, err := vd.read(deviceName, req.DeviceResourceName, dr.Properties.ValueType, dr.Properties.Minimum, dr.Properties.Maximum, d.db); err != nil {
				return nil, err
			} else {
				res[i] = v
			}
		} else {
			return nil, fmt.Errorf("cannot find device resource %s from device %s in cache", req.DeviceResourceName, deviceName)
		}
	}

	/*	if deviceName == "Dis_Alta_2" {
		d.lc.Infof("ts: %d/n", res[0].Origin)
		t := time.Unix(res[0].Origin/1000000000, 0)
		fmt.Printf("%d-%02d-%02dT%02d:%02d:%02d-00:00\n",
			t.Year(), t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second())
	}*/
	return res, nil
}

func (d *VirtualDriver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest,
	params []*dsModels.CommandValue) error {
	d.locker.Lock()
	defer func() {
		d.locker.Unlock()
	}()

	vd, err := d.retrieveDevice(deviceName)
	if err != nil {
		return err
	}

	if err := vd.handleWrite(params, deviceName); err != nil {
		return err
	}

	return nil
}

func (d *VirtualDriver) Stop(force bool) error {
	d.lc.Info("VirtualDriver.Stop: hedge-device-virtual driver is stopping...")
	if err := d.db.closeDb(); err != nil {
		d.lc.Error(fmt.Sprintf("ql DB closed ungracefully, error: %e", err))
	}
	return nil
}

func (d *VirtualDriver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Infof("a new Device is added: %s", deviceName)
	err := prepareResources(d, deviceName)
	return err
}

func (d *VirtualDriver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debug(fmt.Sprintf("Device %s is updated", deviceName))
	err := deleteResources(d, deviceName)
	if err != nil {
		return err
	} else {
		return prepareResources(d, deviceName)
	}
}

func (d *VirtualDriver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.lc.Debug(fmt.Sprintf("Device %s is removed", deviceName))
	err := deleteResources(d, deviceName)
	return err
}

func initResourceTable(driver *VirtualDriver) error {
	if err := driver.db.exec(SqlDropTable); err != nil {
		driver.lc.Info(fmt.Sprintf("Drop table failed: %v", err))
		return err
	}

	if err := driver.db.exec(SqlCreateTable); err != nil {
		driver.lc.Info(fmt.Sprintf("Create table failed: %v", err))
		return err
	}

	return nil
}

// This is being done to allow multile instances of the service running and enable them to generate unique ids across the service instances
func getNextDeviceName(prefix string) (string, error) {

	fileName := "data/lastid"
	fmt.Printf("\n\nReading a file in %s\n", fileName)

	// The ioutil package contains inbuilt
	// methods like ReadFile that reads the
	// filename and returns the contents.
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Errorf("failed reading data from file, error: %v", err)
		return "", err
	}

	lastCountStr := string(data)
	// Convert to int and increment the value
	i, err := strconv.Atoi(lastCountStr)
	fmt.Printf("\tLast Counter was: %d\n", i)
	i += 1
	data = []byte(strconv.Itoa(i))

	err = ioutil.WriteFile(fileName, data, 0644)
	if err != nil {
		fmt.Errorf("\tFailed writing to file, error: %v", err)
		return "", err
	}

	deviceName := prefix + string((data))
	return deviceName, err
}

// Build the default data generator based on profile specification
func prepareResources(driver *VirtualDriver, deviceName string) error {
	driver.locker.Lock()
	defer func() {
		driver.locker.Unlock()
	}()

	device, err := driver.sdk.GetDeviceByName(deviceName)
	if err != nil {
		return err
	}

	profile, err := driver.sdk.GetProfileByName(device.ProfileName)
	if err != nil {
		return err
	}
	datagenerator.BuildDeviceToAutoIntervalMap(device, deviceName)

	// DeviceResource is the metric
	for _, dr := range profile.DeviceResources {
		if dr.Properties.ReadWrite != common.ReadWrite_R && dr.Properties.ReadWrite != common.ReadWrite_RW {
			continue
		}

		datagenerator.BuildDataGeneratorFromAttributes(dr, dr.Name)

		/*
			d.Name <-> VIRTUAL_RESOURCE.deviceName
			dr.Name <-> VIRTUAL_RESOURCE.CommandName, VIRTUAL_RESOURCE.ResourceName
			ro.DeviceResource <-> VIRTUAL_RESOURCE.DeviceResourceName
			dr.Properties.Value.Type <-> VIRTUAL_RESOURCE.DataType
			dr.Properties.Value.DefaultValue <-> VIRTUAL_RESOURCE.Value
		*/
		if dr.Properties.ValueType == common.ValueTypeBinary {
			continue
		}
		if err := driver.db.exec(SqlInsert, device.Name, dr.Name, dr.Name, true, dr.Properties.ValueType,
			dr.Properties.DefaultValue); err != nil {
			driver.lc.Info(fmt.Sprintf("Insert one row into db failed: %v", err))
			return err
		}
	}
	// TODO another for loop to update the ENABLE_RANDOMIZATION field of virtual resource by device resource
	//  "EnableRandomization_{ResourceName}"

	return nil
}

func deleteResources(driver *VirtualDriver, deviceName string) error {
	driver.locker.Lock()
	defer func() {
		driver.locker.Unlock()
	}()

	if err := driver.db.exec(SqlDelete, deviceName); err != nil {
		driver.lc.Info(fmt.Sprintf("Delete Telco resources of device %s failed: %v", deviceName, err))
		return err
	} else {
		return nil
	}
}
