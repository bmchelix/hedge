package router

import (
	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/twin"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// AddTwinDefinition: Handler to add New Twin definition
func (r *Router) AddTwinDefinition(c echo.Context) error {

	logger := r.edgexSvc.LoggingClient()
	logger.Info("adding twin definition")

	// Make doubly sure the twin data being passed is valid
	twinDefinition := new(twin.DigitalTwinDefinition)
	err := c.Bind(twinDefinition)
	if err != nil {
		return r.SendError(fmt.Sprintf("Error parsing request: %s", err.Error()), "bad request", http.StatusBadRequest, c)
	}

	result, err := r.digitalTwinService.AddTwinDefinition(twinDefinition)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}

	return c.JSON(http.StatusCreated,
		Response{
			Result: Result{
				"status": result.Reason,
				"id":     result.Name,
			},
		},
	)
}

// UpdateTwinDefinition: Handler to add New Twin definition
func (r *Router) UpdateTwinDefinition(c echo.Context) error {

	logger := r.edgexSvc.LoggingClient()
	logger.Info("updating twin definition")

	// Make doubly sure the twin data being passed is valid
	twinDefinition := new(twin.DigitalTwinDefinition)
	err := c.Bind(twinDefinition)
	if err != nil {
		return r.SendError(fmt.Sprintf("Error parsing request: %s", err.Error()), "bad request", http.StatusBadRequest, c)
	}

	result, err := r.digitalTwinService.UpdateTwinDefinition(twinDefinition)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}

	return c.JSON(http.StatusCreated,
		Response{
			Result: Result{
				"status": result.Reason,
				"id":     result.Name,
			},
		},
	)
}

// GetTwinDefinition: Handler/route to Get Twin definition
func (r *Router) GetTwinDefinition(c echo.Context) error {

	name := c.Param("name")
	if name == "" {
		// Use hedge error
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "twin name not prvided")
	}
	result, err := r.digitalTwinService.GetTwinDefinition(name)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}

	return c.JSON(http.StatusOK,
		result.DigitalTwinDefinition,
	)
}

// ListTwinDefinitions: Handler to Get a list of Twin definitions
func (r *Router) ListTwinDefinitions(c echo.Context) error {

	logger := r.edgexSvc.LoggingClient()
	logger.Info("List twin definitions")

	twins, err := r.digitalTwinService.GetTwinDefinitions(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error: %s"+err.Error())
		//return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}

	return c.JSON(http.StatusOK,
		twins,
	)
}

// ListTwinDefinitions: Handler to Get a list of Twin definitions
func (r *Router) ListTwinDevices(c echo.Context) error {

	logger := r.edgexSvc.LoggingClient()
	logger.Info("List twin devices")

	result, err := r.digitalTwinService.GetTwinDevices(c)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}

	return c.JSON(http.StatusOK,
		result.Keys,
		/*		Response{
				Result{
					"twins":  result.Keys,
					"status": "twin devices found",
				},
			},*/
	)
}

// DeleteTwinDefinition: Handler to Get New Twin definition
func (r *Router) DeleteTwinDefinition(c echo.Context) error {
	name := c.Param("name")

	logger := r.edgexSvc.LoggingClient()
	logger.Infof("Delete twin definition for name: %s", name)

	result, err := r.digitalTwinService.DeleteTwinDefinition(c)
	if err != nil {
		return r.SendError(result.ErrorMsg, result.Reason, StatusMap[result.Status], c)
	}
	return c.JSON(http.StatusOK, Response{
		Result: Result{
			"status": "twin definition deleted",
			"id":     name,
		},
	},
	)
}
