/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package telemetry

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	sdkinterfaces "github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/metrics"
)

type MetricsManager struct {
	wg         sync.WaitGroup
	Ctx        context.Context
	MetricsMgr interfaces.MetricsManager
}

func NewMetricsManager(service sdkinterfaces.ApplicationService, serviceName string) (*MetricsManager, error) {
	lc := service.LoggingClient()

	interval, err := service.GetAppSetting("MetricReportInterval")
	if err != nil {
		lc.Errorf("failed to retrieve MetricReportInterval from configuration: %s", err.Error())
		return nil, err
	}

	d, err := strconv.Atoi(interval)
	if err != nil {
		lc.Errorf("failed to retrieve PublishTopicPrefix from configuration: %s", err.Error())
		return nil, err
	}
	duration := time.Duration(d) * time.Second

	baseTopic, err := service.GetAppSetting("MetricPublishTopicPrefix")
	if err != nil {
		lc.Errorf("failed to retrieve PublishTopicPrefix from configuration: %s", err.Error())
		return nil, err
	}
	tags := make(map[string]string)
	reporter := NewMQTTMetricReporter(service, baseTopic, serviceName, tags)

	mmgr := MetricsManager{}
	mmgr.Ctx = context.Background()
	defer mmgr.Ctx.Done()

	mmgr.MetricsMgr = metrics.NewManager(service.LoggingClient(), duration, reporter)
	if mmgr.MetricsMgr == nil {
		lc.Errorf("failed to create metrics manager")
		return nil, fmt.Errorf("Failed to create metrics manager")
	}
	return &mmgr, nil
}

func (s *MetricsManager) Run() {
	s.MetricsMgr.Run(s.Ctx, &s.wg)
}
