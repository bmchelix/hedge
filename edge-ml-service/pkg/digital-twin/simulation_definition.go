package digital_twin

import (
	"hedge/edge-ml-service/pkg/dto/twin"
	"encoding/json"
	"fmt"
	"strings"

	"hedge/common/db"
	"github.com/labstack/echo/v4"
)

func (p *DigitalTwinService) AddSimulationDefinition(simulationDefinition *twin.SimulationDefinition) (DigitalTwinServiceResponse, error) {

	logger := p.LoggingClient
	logger.Info("adding twin definition")

	name := simulationDefinition.Name
	pattern := db.SimulationDefinition + ":*"
	names, err := p.DBLayer.DBGetSimulationDefinitions(pattern)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("Error getting simulation definitions: %s", err.Error()),
			Reason:   "internal error",
			Status:   InternalError,
		}, err
	}
	for _, definitionName := range names {
		jsonData, err := p.DBLayer.DBGetSimulationDefinition(definitionName)
		if err != nil {
			return DigitalTwinServiceResponse{
				ErrorMsg: fmt.Sprintf("Error getting simulation definition: %s", err.Error()),
				Reason:   "internal error",
				Status:   InternalError,
			}, err
		}
		simulationDefinition := twin.SimulationDefinition{}
		err = json.Unmarshal([]byte(jsonData), &simulationDefinition)
		if err != nil {
			return DigitalTwinServiceResponse{
				ErrorMsg: fmt.Sprintf("Error unmarshalling JSON data: %s", err.Error()),
				Reason:   "internal error",
				Status:   InternalError,
			}, err
		}

		if simulationDefinition.Name == definitionName {
			return DigitalTwinServiceResponse{
				Status: OK,
				Name:   strings.Replace(definitionName, db.SimulationDefinition+":", "", -1),
				Reason: "returning pre-existing definition",
			}, nil
		}
	}

	jsonData, err := json.Marshal(simulationDefinition)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("Error marshaling JSON data: %s", err.Error()),
			Reason:   "internal server error, invalid twin definition",
			Status:   InternalError,
		}, err
	}
	logger.Infof("Parsed twin definition: %s", string(jsonData))

	err = p.DBLayer.DBAddSimulationDefinition(jsonData, name)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: err.Error(),
			Reason:   "internal server error",
			Status:   InternalError,
		}, err
	}

	return DigitalTwinServiceResponse{
		Reason: "twin definition added successfully",
		Name:   name,
		Status: OK,
	}, nil

}

func (p *DigitalTwinService) GetSimulationDefinitionFromDB(c echo.Context) (DigitalTwinServiceResponse, error) {

	logger := p.LoggingClient
	definitionName := c.Param("name")
	logger.Infof("Getting definition by id: %s", definitionName)

	key := db.SimulationDefinition + ":" + definitionName

	result, err := p.DBLayer.DBGetSimulationDefinition(key)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("error getting twin definition from db: %v", err.Error()),
			Reason:   "error getting twin definition in REDIS",
			Status:   InternalError,
		}, err
	}

	simulationDefinition := new(twin.SimulationDefinition)
	err = json.Unmarshal([]byte(result), simulationDefinition)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("error getting twin definition db: %s", err.Error()),
			Reason:   "error getting twin definition in REDIS",
			Status:   InternalError,
		}, err
	}
	logger.Debugf("Definition found for ID: %s, %s\n", definitionName, simulationDefinition)

	return DigitalTwinServiceResponse{
		Name:                 definitionName,
		SimulationDefinition: simulationDefinition,
	}, nil
}

func (p *DigitalTwinService) GetAllSimulationDefinitions(c echo.Context) (DigitalTwinServiceResponse, error) {

	twinDefinitionKey := db.SimulationDefinition + ":*"
	keysList, err := p.DBLayer.DBGetSimulationDefinitions(twinDefinitionKey)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("error getting twin definition from db: %v", err.Error()),
			Reason:   "error getting twin definition in REDIS",
			Status:   InternalError,
		}, err
	}

	for i, key := range keysList {
		keysList[i] = strings.Replace(key, db.SimulationDefinition+":", "", -1)
	}

	return DigitalTwinServiceResponse{
		Keys:   keysList,
		Status: OK,
	}, nil
}

func (p *DigitalTwinService) DeleteSimulationDefinition(c echo.Context) (DigitalTwinServiceResponse, error) {

	dbLayer := p.DBLayer
	key := c.Param("name")

	logger := p.LoggingClient
	err := dbLayer.DBDeleteSimulationDefinition(key)
	if err != nil {
		return DigitalTwinServiceResponse{
			ErrorMsg: fmt.Sprintf("error deleting twin definition db: %s", err.Error()),
			Reason:   "error deleting twin definition in REDIS",
			Status:   InternalError,
		}, err
	}
	logger.Debugf("Twin definition deleted for ID: %s\n", key)

	return DigitalTwinServiceResponse{
		Name:   key,
		Reason: "twin definition deleted",
	}, nil
}
