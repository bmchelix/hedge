/*******************************************************************************
 * Copyright 2018 Dell Inc.
 * (c) Copyright 2020-2025 BMC Software, Inc.
 *
 * Contributors: BMC Software, Inc. - BMC Helix Edge
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/
package main

import (
	"fmt"
	"os"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"

	"hedge/app-services/hedge-admin/internal/config"
	"hedge/app-services/hedge-admin/internal/router"
	"hedge/common/client"
)

func main() {

	service, ok := pkg.NewAppService(client.HedgeAdminServiceKey)
	if !ok {
		fmt.Printf("Failed to start App Service: %s\n", client.HedgeAdminServiceKey)
		os.Exit(-1)
	}
	lc := service.LoggingClient()

	appConfig := config.NewAppConfig()
	appConfig.LoadAppConfigurations(service)

	r := router.NewRouter(service, appConfig)
	r.LoadRestRoutes()
	err := r.RegisterCurrentNode()
	if err != nil {
		lc.Error("RegisterCurrentNode returned error: %s", err.Error())
	} else {
		lc.Infof("Successfully added node to current node's database")
	}

	// 5) Lastly, we'll go ahead and tell the SDK to "start"
	err = service.Run()
	if err != nil {
		lc.Errorf("Run returned error: %v", err)
		os.Exit(-1)
	}

	// Do any required cleanup here

	os.Exit(0)

}
