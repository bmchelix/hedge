/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package dto

import (
	"fmt"
	"os"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

type AppConfig struct {
	VictoriaMetricsURL string
	HelixExportUrl     string
	//HelixSecretKey       string
	ExportBatchFrequency string
}

func NewAppConfig() *AppConfig {
	appConfig := new(AppConfig)
	return appConfig
}

func (appConfig *AppConfig) LoadAppConfigurations(edgexSdk interfaces.ApplicationService) {

	victoriaMetricsURL, err := edgexSdk.GetAppSetting("VictoriaMetricsURL")
	if err != nil {
		edgexSdk.LoggingClient().Error(err.Error())
	} else {
		appConfig.VictoriaMetricsURL = victoriaMetricsURL
		edgexSdk.LoggingClient().Infof("Local VictoriaMetricsURL %s", victoriaMetricsURL)
	}

	adeTenantUrl, err := edgexSdk.GetAppSetting("ADE_TENANT_URL")
	if err != nil {
		edgexSdk.LoggingClient().Error("Error reading ADE_TENANT_URL: %s", err.Error())
	} else {
		// Optional URL for testing with local URL: Do not delete this -- appConfig.HelixExportUrl = fmt.Sprintf("%s%s", "http://localhost:8428", "/api/v1/write")
		appConfig.HelixExportUrl = fmt.Sprintf("%s/%s", adeTenantUrl, "metrics-gateway-service/api/v1.0/prometheus")
		edgexSdk.LoggingClient().Infof("HelixExportUrl %s", appConfig.HelixExportUrl)
	}
	if adeTenantUrl == "" && victoriaMetricsURL == "" {
		edgexSdk.LoggingClient().Errorf("Both VictoriaMetricsURL and ADE_TENANT_URL are null, Exiting...")
		os.Exit(-1)
	}

	//appConfig.ExportBatchFrequency = exportBatchFrequency[0]
}
