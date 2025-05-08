/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"hedge/edge-ml-service/internal/digital-twin/router"
	"context"
	"fmt"
	"net/http"
	"os"

	commonDtos "github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"

	"hedge/common/client"
	"hedge/common/db"
	"hedge/common/db/redis"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/requests"
)

func main() {
	// 1) Create the service
	service, ok := pkg.NewAppService(client.DigitalTwinServiceKey)
	if !ok {
		fmt.Errorf("error creating digital-twin service, exiting")
		os.Exit(-1)
	}

	// 2) Get the logger
	logger := service.LoggingClient()

	// 3) Create a DB client
	dbConfig := db.NewDatabaseConfig()
	dbConfig.LoadAppConfigurations(service)
	dbClient := redis.CreateDBClient(dbConfig)
	logger.Infof("dbClient: %v", dbClient)

	// 4) Create a router and add routes
	router := router.NewRouter(service, dbClient)
	router.LoadRestRoutes()

	subscribeToNotifications(service)

	// 5) Run the service
	err := service.Run()
	if err != nil {
		errorMsg := fmt.Sprintf("Run returned error: %s\n", err.Error())
		logger.Error(errorMsg)
		//runLoop(errorMsg, time.Second*30)
	}
}

// Subscribe to notifications
func subscribeToNotifications(svc interfaces.ApplicationService) {

	subscription := []requests.AddSubscriptionRequest{{
		BaseRequest: commonDtos.NewBaseRequest(),
		Subscription: dtos.Subscription{
			Name:        "model-deployed",
			Description: "model training finished",
			Receiver:    "digital-twin",
			Channels: []dtos.Address{{
				Type: "REST",
				Host: "localhost",
				Port: 48090,
				RESTAddress: dtos.RESTAddress{
					Path:       router.TwinNotificationPath,
					HTTPMethod: http.MethodPost,
				},
			}},
			Categories: []string{
				"deployment-finished",
			},
			Labels:      []string{"daily"},
			AdminState:  "UNLOCKED",
			ResendLimit: 0,
		},
	}}

	// subscribe to notifications
	subResp, err := svc.SubscriptionClient().Add(context.Background(), subscription)
	if err != nil {
		svc.LoggingClient().Errorf("Subscription addition failed: %s", err.Error())
	}

	// check subscription status
	for _, resp := range subResp {
		if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 409 {
			svc.LoggingClient().Infof("Pre-existing subscriptions : %+v", resp)
		} else {
			svc.LoggingClient().Infof("Subscription addition: %+v", resp)
		}
	}
}
