package digital_twin

import (
	"hedge/common/db"
	"hedge/edge-ml-service/pkg/dto/twin"
	"fmt"
	"github.com/labstack/echo/v4"
)

func (p *DigitalTwinService) AddTwinDefinition(twinDefinition *twin.DigitalTwinDefinition) (DigitalTwinServiceResponse, error) {

	err := p.DBLayer.DBAddTwinDefinition(*twinDefinition)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: err.Error(),
			Reason:   "unable to add twin definition",
			Status:   InternalError,
		}, err
	}

	return DigitalTwinServiceResponse{
		Reason: "twin definition added successfully",
		Name:   twinDefinition.Name,
		Status: OK,
	}, nil

}

func (p *DigitalTwinService) UpdateTwinDefinition(twinDefinition *twin.DigitalTwinDefinition) (DigitalTwinServiceResponse, error) {

	err := p.DBLayer.DBUpdateTwinDefinition(*twinDefinition)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: err.Error(),
			Reason:   "unable to add twin definition",
			Status:   InternalError,
		}, err
	}

	return DigitalTwinServiceResponse{
		Reason: "twin definition added successfully",
		Name:   twinDefinition.Name,
		Status: OK,
	}, nil

}

func (p *DigitalTwinService) GetTwinDefinition(twinDefinitionName string) (DigitalTwinServiceResponse, error) {

	logger := p.LoggingClient

	logger.Infof("getting definition by name: %s", twinDefinitionName)

	definition, err := p.DBLayer.DBGetTwinDefinition(twinDefinitionName)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("error getting twin definition from db: %v", err.Error()),
			Reason:   "error getting twin definition in REDIS",
			Status:   InternalError,
		}, err
	}

	logger.Debugf("Definition found for ID: %s, %s\n", twinDefinitionName, definition)

	return DigitalTwinServiceResponse{
		Name:                  twinDefinitionName,
		DigitalTwinDefinition: definition,
	}, nil
}

// Get all the digital twin definitions
func (p *DigitalTwinService) GetTwinDefinitions(c echo.Context) ([]*twin.DigitalTwinDefinition, error) {
	twins, err := p.DBLayer.DBGetTwinDefinitions()
	return twins, err
}

func (p *DigitalTwinService) GetTwinDevices(c echo.Context) (DigitalTwinServiceResponse, error) {

	name := c.Param("name")
	if name == "" {
		return DigitalTwinServiceResponse{
			ErrorMsg: "error no twin name specified",
			Reason:   "error getting twin definitions in REDIS",
			Status:   InternalError,
		}, fmt.Errorf("error: %s", "no twin name specified")
	}

	key := db.TwinDefinition + ":" + name
	definition, err := p.DBLayer.DBGetTwinDefinition(key)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("error getting twin definition from db: %v", err.Error()),
			Reason:   "error getting twin definition in REDIS",
			Status:   InternalError,
		}, err
	}

	devices := make([]string, 0)
	for _, entity := range definition.Entities {
		if entity.EntityType == twin.Device {
			devices = append(devices, entity.Name)
		}
	}

	return DigitalTwinServiceResponse{
		Keys: devices,
	}, nil

}

func (p *DigitalTwinService) DeleteTwinDefinition(c echo.Context) (DigitalTwinServiceResponse, error) {

	dbLayer := p.DBLayer
	key := c.Param("name")

	logger := p.LoggingClient
	err := dbLayer.DBDeleteTwinDefinition(key)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("error deleting twin definition db: %s", err.Error()),
			Reason:   "error deleteing twin definition in REDIS",
			Status:   InternalError,
		}, err
	}
	logger.Debugf("Twin definition deleted for name: %s\n", key)

	return DigitalTwinServiceResponse{
		Name:   key,
		Reason: "twin definition deleted",
	}, nil
}
