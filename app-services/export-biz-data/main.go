/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"hedge/app-services/export-biz-data/functions"
	"hedge/common/client"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"os"
)

var s string

func main() {
	service, ok := pkg.NewAppServiceWithTargetType(client.HedgeExportBizDataServiceKey, &s)
	if !ok {
		os.Exit(-1)
	}
	lc := service.LoggingClient()
	interval, _ := service.GetAppSetting("TriggerInterval")
	srv, _ := service.GetAppSetting("TriggerServer")
	// Call Http function to trigger pipeline
	go functions.HttpTrigger(interval, srv, nil)

	contextData := functions.NewContextData(service)
	if contextData == nil {
		lc.Error("unable to connect to database, exiting")
		os.Exit(-1)
	}

	err := service.SetDefaultFunctionsPipeline(
		contextData.QueryNewBizData,
		contextData.HttpSend,
	)
	if err != nil {
		lc.Errorf("SetDefaultFunctionsPipeline returned error: %s", err.Error())
		os.Exit(-1)
	}

	err = service.Run()
	if err != nil {
		lc.Error("Run returned error: ", err.Error())
		os.Exit(-1)
	}

	os.Exit(0)
}
