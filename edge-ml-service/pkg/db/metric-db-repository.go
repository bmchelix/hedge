/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package db

import (
	"hedge/common/service"
	"hedge/edge-ml-service/pkg/dto/config"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"io/ioutil"
	"net/http"
)

type MetricDataDbInterface interface {
	GetMetricDataDb(service interfaces.ApplicationService, connectionConfig *config.MLMgmtConfig, dataSourceProvider service.DataStoreProvider) MetricDataDbInterface
	GetTSLabels(label string) ([]byte, error)
}

var MetricDataDbInterfaceImpl MetricDataDbInterface

type MetricDataDb struct {
	service            interfaces.ApplicationService
	connectionConfig   *config.MLMgmtConfig
	dataSourceProvider service.DataStoreProvider
	client             *http.Client
}

func NewMetricDataDb(service interfaces.ApplicationService, connectionConfig *config.MLMgmtConfig, dataSourceProvider service.DataStoreProvider) MetricDataDbInterface {
	// For unit test case writing, we override the MetricDataDbInterfaceImpl
	if MetricDataDbInterfaceImpl == nil {
		MetricDataDbInterfaceImpl = &MetricDataDb{}
	}

	return MetricDataDbInterfaceImpl.GetMetricDataDb(service, connectionConfig, dataSourceProvider)
}

func (m *MetricDataDb) GetMetricDataDb(service interfaces.ApplicationService, connectionConfig *config.MLMgmtConfig, dataSourceProvider service.DataStoreProvider) MetricDataDbInterface {
	metricDataDb := &MetricDataDb{
		service:            service,
		connectionConfig:   connectionConfig,
		client:             &http.Client{},
		dataSourceProvider: dataSourceProvider,
	}
	return metricDataDb
}

func (db *MetricDataDb) GetTSLabels(label string) ([]byte, error) {

	metricDataUrl := db.dataSourceProvider.GetDataURL() + "/label/" + label + "/values"

	request, err := http.NewRequest(http.MethodGet, metricDataUrl, nil)

	if err != nil {
		db.service.LoggingClient().Error(err.Error())
		return nil, err
	}
	db.dataSourceProvider.SetAuthHeader(request)
	resp, err := db.client.Do(request)

	if err != nil {
		db.service.LoggingClient().Error(err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		db.service.LoggingClient().Error(err.Error())
		return nil, err
	}
	return body, nil
}
