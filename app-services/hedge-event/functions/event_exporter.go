/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"errors"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/util"
)

type Exporter struct {
	elasticExporter ElasticExporterInterface
	//elasticEnabled   bool
	//adeExporter      ADEExporterInterface
	//adeExportEnabled bool
	// service          interfaces.ApplicationService
}

func NewExporter(service interfaces.ApplicationService) *Exporter {
	exp := new(Exporter)
	exp.elasticExporter = NewElasticExporter(service)
	return exp

}

func (exp *Exporter) StoreEventToLocalElastic(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {

	lc := ctx.LoggingClient()
	lc.Info("saving event to elasticsearch")

	if data == nil {
		lc.Error("no event data Received")
		return false, errors.New("no Data Received")
	}

	ok, _ := exp.elasticExporter.SaveEventToElastic(ctx, data)
	if !ok {
		lc.Errorf("error saving to local elastic, payload %v", data)
		return false, nil
	}

	// Pass-thru when ADE is enabled
	// whether there is an error or not, return true if ADE export is enabled since it is the next step of the pipeline
	return true, data
}

func Print(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	if data == nil {
		// We didn't receive a result
		return false, nil
	}

	dataToPrint, err := util.CoerceType(data)
	if err != nil {
		return false, err
	}
	lc := ctx.LoggingClient()
	lc.Infof("Read data from pipeline: %s", string(dataToPrint))
	return true, data
}
