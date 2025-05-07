// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018-2019 IOTech Ltd
// (c) Copyright 2020-2025 BMC Software, Inc.
//
// Contributors: BMC Software, Inc. - BMC Helix Edge
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/startup"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"hedge/common/client"
	deviceVirtual "hedge/device-services/device-virtual"
	"hedge/device-services/device-virtual/internal/driver"

	"runtime/debug"
	"time"
)

var err error

func start() {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered in %+v, stacktrace: %s", r, string(debug.Stack()))
		}
	}()
	d := driver.NewVirtualDriver()
	startup.Bootstrap(client.DeviceVirtual, deviceVirtual.Version, d)
}

func main() {

	sleepTime := time.Second * 30
	loggingClient := logger.NewClient(client.DeviceVirtual, "INFO")
	retryCount := 0
	for retryCount < 10 {
		start()
		if err != nil {
			loggingClient.Error(fmt.Sprintf("Error starting service: %+v, retrying after %d seconds\n", err, sleepTime/time.Second))
		}
		time.Sleep(sleepTime)
		retryCount++
	}
}
