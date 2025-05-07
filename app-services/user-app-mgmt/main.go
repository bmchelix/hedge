//
// Copyright (c) 2020 Intel Corporation
// Copyright (c) 2021 One Track Consulting
// (c) Copyright 2020-2025 BMC Software, Inc.
//
// Contributors: BMC Software, Inc. - BMC Helix Edge
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package main

import (
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"hedge/app-services/user-app-mgmt/pkg/router"
	"hedge/common/client"
	"os"
	"strings"
)

func main() {

	// 1) First thing to do is to create an instance of the app service and initialize it.
	appService, ok := pkg.NewAppService(client.HedgeUserAppMgmtServiceKey)
	if !ok {
		appService.LoggingClient().Error("SDK initialization failed.")
		os.Exit(-1)
	}

	// 2) shows how to access the application's specific configuration settings.
	//deviceNames, err := appService.GetAppSettingStrings("Database")
	usrAppSettings := appService.ApplicationSettings()
	appService.LoggingClient().Infof("App settings list: %v", usrAppSettings)

	usrAppSvcRouter := router.NewRouter(appService, usrAppSettings)
	if usrAppSvcRouter == nil {
		appService.LoggingClient().Error("Could not create router, exiting")
		os.Exit(-1)
	}
	usrAppSvcRouter.RegisterRoutes()

	enableGrafanaSync := false
	if enableGrafanaSyncStr, ok := usrAppSettings["Enable_Grafana_sync"]; ok {
		if strings.EqualFold(enableGrafanaSyncStr, "true") {
			enableGrafanaSync = true
		}
	}

	// Start the job to poll for dashboards and update resource database with dashboard names only for Hedge Grafana
	if enableGrafanaSync {
		usrAppSvcRouter.CallGrafanaRoutine(usrAppSettings)
	}

	// 4) Lastly, we'll go ahead and tell the service to "start" and begin listening for events
	// to trigger the pipeline.
	err := appService.Run()
	if err != nil {
		appService.LoggingClient().Errorf("Run returned error: %s", err.Error())
		os.Exit(-1)
	}

	// Do any required cleanup here
	os.Exit(0)
}

//https://biliot-aiops-trial.dsmlab.bmc.com/tsws/monitoring/api/v1.0/authprofiles
//{"name":"MyTest","id":"","description":"","userGroups":"781267225881629","roles":[],"allowedObjects":{"MOPOLICYCONFIGTYPE":{"unrestrictedAccess":false,"rbacObjects":[]},"SOLUTION":{"unrestrictedAccess":true,"rbacObjects":[]},"ACL":{"unrestrictedAccess":true,"rbacObjects":[]},"DEVICE":{"unrestrictedAccess":true,"rbacObjects":[]},"TAG":{"unrestrictedAccess":true,"rbacObjects":[]}}}
