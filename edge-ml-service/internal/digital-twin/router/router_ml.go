package router

import (
	"hedge/edge-ml-service/pkg/dto/twin"
	"encoding/json"
	"net/http"

	"hedge/common/db"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

func (r *Router) RunTraining(c echo.Context) error {

	name := c.Param("name")
	simulationName := c.Param("simulationName")

	if simulationName == "" {
		return c.JSON(http.StatusBadRequest, "simulationName must be specified")
	}

	response, err := r.Orchestrator.SubmitTrainingJob(name, simulationName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusAccepted, response)

}

func (r *Router) getSimulationDefinition(simulationName string) (twin.SimulationDefinition, error) {
	key := db.SimulationDefinition + ":" + simulationName

	definition, err := r.digitalTwinService.DBLayer.DBGetSimulationDefinition(key)
	if err != nil {
		return twin.SimulationDefinition{}, errors.Wrap(err, "error getting simulation Definition from DB")
	}
	simulationDefinition := twin.SimulationDefinition{}
	err = json.Unmarshal([]byte(definition), &simulationDefinition)
	if err != nil {
		return twin.SimulationDefinition{}, errors.Wrap(err, "error parsing simulation definition")
	}
	return simulationDefinition, nil
}
