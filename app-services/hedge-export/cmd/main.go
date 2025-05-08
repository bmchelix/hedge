/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"hedge/app-services/hedge-export/cmd/config"
	"hedge/app-services/hedge-export/cmd/functions"
	"hedge/app-services/hedge-export/internal/dto"
	"hedge/common/client"
	commonConfig "hedge/common/config"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"os"
)

func main() {

	service, ok := pkg.NewAppServiceWithTargetType(client.HedgeExportServiceKey, new(interface{}))
	if !ok {
		os.Exit(-1)
	}
	lc := service.LoggingClient()
	serviceConfig := &config.ServiceConfig{}
	appConfig := dto.NewAppConfig()
	appConfig.LoadAppConfigurations(service)

	EnableExportTelemetryStr, err := service.GetAppSetting("EnableExportTelemetry")
	if err != nil {
		lc.Errorf("EnableExportTelemetry parameter not found in the config")
		EnableExportTelemetryStr = "false"
	}
	enableExportTelemetry := false
	if EnableExportTelemetryStr == "true" {
		enableExportTelemetry = true
	}
	// EnableRateMeasure
	if err := service.LoadCustomConfig(serviceConfig, "ExportConfig"); err != nil {
		lc.Errorf("failed to load custom configuration: %s", err.Error())
		os.Exit(-1)
	}

	exportsvc := functions.NewMetricExportService(appConfig.VictoriaMetricsURL, enableExportTelemetry, serviceConfig.ExportConfig, commonConfig.GetPersistOnError(service))

	subscribeTopics, err := service.GetAppSettingStrings("SubscribeTopics")
	if err != nil {
		lc.Errorf("failed to retrieve subscribedTopic from configuration: %s", err.Error())
		os.Exit(-1)
	}
	lc.Infof("subscribedTopics: %v", subscribeTopics)

	// Pipeline for local timeseries data storage
	if err := service.AddFunctionsPipelineForTopics("local-timeseries-pipeline", subscribeTopics,
		exportsvc.TransformToTimeseries,
		exportsvc.ExportToTSDB,
	); err != nil {
		lc.Errorf("SetDefaultFunctionsPipeline failed: %v\n", err)
		os.Exit(-1)
	}

	if err := service.ListenForCustomConfigChanges(&serviceConfig.ExportConfig, "ExportConfig", exportsvc.ProcessConfigUpdates); err != nil {
		lc.Errorf("unable to watch custom writable configuration: %s", err.Error())
		os.Exit(-1)
	}

	// 5) Lastly, we'll go ahead and tell the SDK to "start"
	err = service.Run()
	if err != nil {
		lc.Errorf("Run returned error: %s", err.Error())
		os.Exit(-1)
	}
	os.Exit(0)
}
