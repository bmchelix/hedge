package router

import (
	"hedge/edge-ml-service/pkg/dto/twin"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
)

// AddSimulationDefinition: Handler to add New Twin Simulation definition
func (r *Router) AddSimulationDefinition(c echo.Context) error {

	logger := r.edgexSvc.LoggingClient()
	logger.Info("adding simulation definition")

	simulationDefinition := new(twin.SimulationDefinition)
	err := c.Bind(simulationDefinition)
	if err != nil {
		return r.SendError(fmt.Sprintf("error parsing request: %s", err.Error()), "bad request", http.StatusBadRequest, c)
	}

	//TODO: Validate that the mlConfig do exist by calling ml-management API
	result, err := r.digitalTwinService.AddSimulationDefinition(simulationDefinition)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}

	return c.JSON(http.StatusOK,
		Response{
			Result: Result{
				"status": result.Reason,
				"id":     result.Name,
			},
		},
	)
}

// GetSimulationDefinition: Handler to Get New Twin Simulation definition
func (r *Router) GetSimulationDefinition(c echo.Context) error {

	logger := r.edgexSvc.LoggingClient()
	definitionName := c.Param("name")

	if definitionName == "all" {
		logger.Info("getting all simulation definitions")

		result, err := r.digitalTwinService.GetAllSimulationDefinitions(c)
		if err != nil {
			return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
		}

		return c.JSON(http.StatusOK,
			Response{
				Result: Result{
					"names": result.Keys,
				},
			},
		)

	}

	logger.Infof("getting simulation definition by name: %s", definitionName)
	result, err := r.digitalTwinService.GetSimulationDefinitionFromDB(c)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}

	return c.JSON(http.StatusOK,
		Response{
			Result: Result{
				"name":       result.Name,
				"definition": result.SimulationDefinition,
			},
		},
	)
}

// ListSimulationDefinitions: Handler to Get a list of Twin Simulation definitions
func (r *Router) ListSimulationDefinitions(c echo.Context) error {

	logger := r.edgexSvc.LoggingClient()
	logger.Info("list simulation definitions")

	result, err := r.digitalTwinService.GetAllSimulationDefinitions(c)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}

	return c.JSON(http.StatusOK,
		Response{
			Result{
				"twins":  result.Keys,
				"status": "simulation definitions found",
			},
		},
	)

}

// DeleteSimulationDefinition: Handler to Get New Twin Simulation definition
func (r *Router) DeleteSimulationDefinition(c echo.Context) error {

	logger := r.edgexSvc.LoggingClient()

	definitionName := c.Param("name")
	logger.Infof("delete simulation definition for id: %s", definitionName)

	result, err := r.digitalTwinService.DeleteSimulationDefinition(c)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}
	return c.JSON(http.StatusOK, Response{
		Result: Result{
			"status": "simulation definition deleted",
			"id":     definitionName,
		},
	},
	)
}
