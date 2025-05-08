/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"hedge/app-services/data-enrichment/cache"
	"hedge/app-services/data-enrichment/config"
	"hedge/app-services/data-enrichment/functions"
	"hedge/app-services/data-enrichment/router"
	common "hedge/common/client"
	commonconfig "hedge/common/config"
	"hedge/common/models"
	"hedge/common/service"
	"hedge/common/telemetry"
	"encoding/gob"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"
)

type enrich struct {
	service               interfaces.ApplicationService
	lc                    logger.LoggingClient
	configChanged         chan bool
	storeAndForwardConfig *config.WritableConfig
	mqttSender            service.MqttSender
	pubsub                *functions.DataEnricher
}

func main() {
	gob.Register([]*models.HedgeEvent{})

	app := enrich{}

	code := app.CreateAndRunAppService(common.HedgeDataEnrichmentServiceKey, pkg.NewAppService)
	os.Exit(code)
}

func (app *enrich) CreateAndRunAppService(serviceKey string, newServiceFactory func(string) (interfaces.ApplicationService, bool)) int {
	var ok bool
	app.service, ok = newServiceFactory(serviceKey)
	if !ok {
		return -1
	}

	app.lc = app.service.LoggingClient()
	metricsManager, err := telemetry.NewMetricsManager(app.service, common.HedgeDataEnrichmentServiceName)
	if err != nil {
		app.lc.Errorf("Run returned error: %s", err.Error())
		return -1
	}

	metricsManager.Run()

	app.storeAndForwardConfig = &config.WritableConfig{}
	if err := app.service.LoadCustomConfig(app.storeAndForwardConfig, "Writable/StoreAndForward"); err != nil {
		app.lc.Errorf("failed load custom configuration: %s", err.Error())
		return -1
	}
	persistOnError := app.storeAndForwardConfig.Writable.StoreAndForward.Enabled
	app.lc.Infof("Initial value of StoreAndForward Enabled: %t:", persistOnError)

	mqttTopic, err := app.service.GetAppSetting("MqttTopic")
	if err != nil {
		app.lc.Errorf("failed to retrieve MqttTopic from configuration: %s", err.Error())
		return -1
	}
	mqttConfig, err := commonconfig.BuildMQTTSecretConfig(app.service, mqttTopic, common.HedgeDataEnrichmentServiceName)
	if err != nil {
		app.lc.Errorf("failed to create MQTT configuration: %s, exiting", err.Error())
		return -1
	}

	app.mqttSender = service.NewMQTTSecretSender(mqttConfig, persistOnError)

	nodeId, _ := commonconfig.GetCurrentNodeIdAndHost(app.service)
	if nodeId == "" {
		app.lc.Errorf("Could not get the running nodeId from hedge-admin, exiting downsampling will not work")
		return -1
	}

	memCache, err := cache.NewMemCache(app.service, common.Client, nodeId)
	if err != nil {
		app.lc.Errorf("Could not create cache instance exiting, error: %s", err)
		return -1
	}

	app.pubsub, err = functions.NewDataEnricher(app.service, metricsManager.MetricsMgr, memCache, common.HedgeDataEnrichmentServiceName, nodeId)
	if app.pubsub == nil {
		app.lc.Errorf("failed to create DataEnricher instance: %s", err.Error())
		return -1
	}

	r := router.NewRouter(app.service, memCache)
	r.LoadRoute()

	subscribedTopics, err := app.service.GetAppSettingStrings("SubscribeTopics")
	if err != nil {
		app.lc.Errorf("failed to retrieve subscribedTopic from configuration: %s", err.Error())
		return -1
	}

	// Listen for StoreAndForward_Enabled changes
	if err := app.service.ListenForCustomConfigChanges(app.storeAndForwardConfig, "Writable/StoreAndForward", app.ProcessStoreAndForwardConfigUpdates); err != nil {
		app.lc.Errorf("unable to watch Writable/StoreAndForward configuration: %s", err.Error())
		return -1
	}

	err = app.service.AddFunctionsPipelineForTopics("Enrich", subscribedTopics,
		app.pubsub.EnrichData,
		app.pubsub.PublishEnrichedDataInternally,
		app.pubsub.TransformToAggregatesAndEvents,
		app.pubsub.SendToEventPublisher,
		app.mqttSender.MQTTSend,
	)
	if err != nil {
		app.lc.Errorf("AddFunctionsPipelineForTopic returned error: %s", err.Error())
		return -1
	}

	go func() {
		monitorMemory, err := app.service.GetAppSetting("monitorMemory")
		if err != nil {
			app.lc.Warnf("monitorMemory is not set, defaulting to false")
			monitorMemory = "false"
		}

		if monitorMemory == "true" {
			ticker := time.NewTicker(4 * time.Minute)
			quit := make(chan struct{})
			go func() {
				for {
					select {
					case <-ticker.C:
						var memStats runtime.MemStats
						runtime.ReadMemStats(&memStats)
						app.lc.Infof("Total allocated memory (in bytes): %d\n", memStats.Alloc)
						app.lc.Infof("Heap memory (in bytes): %d\n", memStats.HeapAlloc)
						app.lc.Infof("Number of garbage collections: %d\n", memStats.NumGC)
					case <-quit:
						ticker.Stop()
						return
					}
				}
			}()
		}

	}()

	if err := app.service.Run(); err != nil {
		app.lc.Errorf("Run returned error: %s", err.Error())
		return -1
	}

	return 0
}

func (app *enrich) ProcessStoreAndForwardConfigUpdates(rawWritableDataConfig interface{}) {
	updated := config.WritableConfig{}
	if err := app.service.LoadCustomConfig(&updated, "Writable/StoreAndForward"); err != nil {
		app.lc.Errorf("failed load Writable/StoreAndForward configuration from Configuration Provider: %s", err.Error())
		return
	}
	lc := app.lc
	previousConfig := app.storeAndForwardConfig
	app.lc.Infof("previous StoreAndForward_Enabled value: %t", previousConfig.Writable.StoreAndForward.Enabled)
	app.storeAndForwardConfig = &updated

	if previousConfig.Writable.StoreAndForward.Enabled == updated.Writable.StoreAndForward.Enabled {
		lc.Info("no StoreAndForward_Enabled changes detected")
		return
	} else {
		lc.Infof("StoreAndForward_Enabled value changed to %t", updated.Writable.StoreAndForward.Enabled)
	}

	app.mqttSender.SetPersistOnError(updated.Writable.StoreAndForward.Enabled)
	lc.Infof("PersistOnError value updated for mqttSender, new value: %v", updated.Writable.StoreAndForward.Enabled)
}
