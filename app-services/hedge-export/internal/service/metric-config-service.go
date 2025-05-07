/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/app-services/hedge-export/internal/db/redis"
	"hedge/app-services/hedge-export/internal/dto"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

type MetricConfigService struct {
	edgexSdk interfaces.ApplicationService
	dbClient *redis.DBClient
}

func NewMetricConfigService(service interfaces.ApplicationService, dbClient *redis.DBClient) *MetricConfigService {
	metricService := new(MetricConfigService)
	metricService.edgexSdk = service
	metricService.dbClient = dbClient
	return metricService
}

func (service MetricConfigService) AddMetricExportData(metricConfig dto.MetricExportConfig) (string, error) {
	configName := metricConfig.Metric + ":" + metricConfig.Profile
	id, err := service.dbClient.AddMetricExportData(metricConfig, configName)
	if err != nil {
		service.edgexSdk.LoggingClient().Warn(err.Error())
		return id, err
	}
	return id, nil
}

func (service MetricConfigService) UpdateMetricExportData(metricConfig dto.MetricExportConfig) (string, error) {
	configName := metricConfig.Metric + ":" + metricConfig.Profile
	id, err := service.dbClient.UpdateMetricExportData(metricConfig, configName)
	if err != nil {
		service.edgexSdk.LoggingClient().Warn(err.Error())
		return id, err
	}
	return id, nil
}

func (service MetricConfigService) GetAllMetricExportData() ([]dto.MetricExportConfig, error) {
	allMetricConfigs, err := service.dbClient.GetAllMetricExportData()
	if err != nil {
		service.edgexSdk.LoggingClient().Warn(err.Error())
		return allMetricConfigs, err
	}
	return allMetricConfigs, nil
}

func (service MetricConfigService) GetMetricExportData(id string) (dto.MetricExportConfig, error) {

	config, err := service.dbClient.GetMetricExportDataById(id)
	if err != nil {
		service.edgexSdk.LoggingClient().Warn(err.Error())
		return config, err
	}
	return config, nil
}

func (service MetricConfigService) DeleteMetricExportData(metricConfig string) error {

	err := service.dbClient.DeleteMetricExportData(metricConfig)
	if err != nil {
		service.edgexSdk.LoggingClient().Warn(err.Error())
		return err
	}
	return nil
}

func (service MetricConfigService) SetFrequencyExportData(intvl string) error {
	err := service.dbClient.SetIntervalExportData(intvl)
	if err != nil {
		return err
	}
	return nil
}

func (service MetricConfigService) GetFrequencyExportData() (interface{}, error) {
	freq, err := service.dbClient.GetIntervalExportData()
	if err != nil {
		return nil, err
	}
	return freq, nil
}
