package router

import (
	"hedge/app-services/data-enrichment/cache"
	"hedge/app-services/metadata-notifier/pkg"
	service2 "hedge/common/service"
	"bytes"
	"context"
	"encoding/json"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	commonDtos "github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/requests"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
)

type Router struct {
	service           interfaces.ApplicationService
	cache             cache.Manager
	deviceInfoService *service2.DeviceInfoService
}

func NewRouter(service interfaces.ApplicationService, cache cache.Manager) *Router {
	router := new(Router)
	router.service = service
	router.cache = cache

	mdsUrl, err := service.GetAppSetting("MetaDataServiceUrl")
	if err != nil && len(mdsUrl) != 1 {
		service.LoggingClient().Errorf("failed to retrieve ApiServer from configuration: %s", err.Error())
		return nil
	}
	router.deviceInfoService = service2.GetDeviceInfoService(mdsUrl).LoadProfileAndLabels()

	return router
}

func (r Router) LoadRoute() {
	r.service.AddCustomRoute("/api/v3/metadata/notify", interfaces.Authenticated, func(c echo.Context) error {
		bodyBytes, err := io.ReadAll(c.Request().Body)
		if err != nil {
			http.Error(c.Response(), err.Error(), http.StatusBadRequest)
			r.service.LoggingClient().Error(err.Error())
			return err
		}
		decoder := json.NewDecoder(bytes.NewBuffer(bodyBytes))
		var metaEnrich pkg.MetaEvent
		err = decoder.Decode(&metaEnrich)

		switch metaEnrich.Type {
		case pkg.ProfileType:
			profileName := metaEnrich.Name
			// force fetch profile and down sampling to update cache
			r.cache.UpdateProfileTags(profileName, true)
			// update linked devices (because profile can hold device attributes that possible to update on all devices)
			go r.UpdateLinkedDevices(profileName)
			r.cache.UpdateProfileDownSampling(profileName, true)
		case pkg.DeviceType:
			deviceName := metaEnrich.Name
			// force fetch device tags to update cache
			r.cache.UpdateDeviceTags(deviceName, true)
		case pkg.NodeType:
			// force fetch raw data to update cache
			r.cache.UpdateNodeRawData(true)
		}
		return nil
	}, http.MethodPut)

	subscription := []requests.AddSubscriptionRequest{{
		BaseRequest: commonDtos.NewBaseRequest(),
		Subscription: dtos.Subscription{
			Name:        "data-enrichment-notify",
			Description: "metadata sync finished",
			Receiver:    "data-enrichment",
			Channels: []dtos.Address{{
				Type: "REST",
				Host: "data-enrichment",
				Port: 59740,
				RESTAddress: dtos.RESTAddress{
					Path:       "/api/v3/metadata/notify",
					HTTPMethod: http.MethodPut,
				},
			}},
			Categories: []string{
				"sync-finished",
			},
			AdminState: "UNLOCKED",
		},
	}}

	// subscribe to notifications
	subResp, err := r.service.SubscriptionClient().Add(context.Background(), subscription)
	if err != nil {
		r.service.LoggingClient().Errorf("failed subscription setup for notifications: %v", err)
	}

	// check subscription status
	for _, resp := range subResp {
		// check subscription created or already exist
		if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 409 {
			r.service.LoggingClient().Errorf("failed subscription setup for notifications: %v", r)
		}
	}
}

func (r Router) UpdateLinkedDevices(profileName string) {
	offset := 0
	limit := 100

	for {
		resp, err := r.service.DeviceClient().DevicesByProfileName(context.Background(), profileName, offset, limit)
		if err != nil {
			r.service.LoggingClient().Errorf("failed fetching linked devices for profile '%s', offset: %d limit: %d err: %s", profileName, offset, limit, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			r.service.LoggingClient().Errorf("failed fetching linked devices for profile '%s', offset: %d limit: %d status: %d", profileName, offset, limit, resp.StatusCode)
		}

		// update cache tags for linked devices
		for _, d := range resp.Devices {
			r.cache.UpdateDeviceTags(d.Name, true)
		}

		if len(resp.Devices) < limit {
			break
		}

		// Increment the offset to fetch the next page
		offset += limit
	}
}
