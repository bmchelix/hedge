package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"hedge/edge-ml-service/pkg/dto/job"
	ttlcache "github.com/jellydator/ttlcache/v3"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

var cache = ttlcache.New[string, bool](
	ttlcache.WithTTL[string, bool](1*time.Minute),
	ttlcache.WithCapacity[string, bool](100),
)
var cacheMutex = sync.Mutex{}

func init() {
	go cache.Start()
}

// This function handles a notification from the Edge-ML service. It checks if the training job has
// completed successfully and then retrieves the simulation definition from the database.
// If successful, a REST API call is made to the ML broker to kick off a prediction.
func (r *Router) RouteNotify(c echo.Context) error {

	r.edgexSvc.LoggingClient().Info("In RouteNotify")
	var err error
	defer errors.Wrap(err, "routeNotify: ")

	reader := c.Request().Body
	body, err := io.ReadAll(reader)
	if err != nil {
		r.edgexSvc.LoggingClient().Errorf("RouteNotify: %s", err.Error())
		return c.JSON(http.StatusInternalServerError, err.Error())
	}

	r.edgexSvc.LoggingClient().Debugf("RouteNotify: %+v", string(body))

	jobDetails := job.TrainingJobDetails{}
	err = json.Unmarshal(body, &jobDetails)
	if err != nil {
		r.edgexSvc.LoggingClient().Errorf("error: %s", err.Error())
		return c.JSON(http.StatusInternalServerError, err.Error())
	}

	jobName := jobDetails.Name
	cacheMutex.Lock()
	if result := cache.Get(jobName); result != nil {
		return c.JSON(http.StatusOK, "Notification processed")
	} else {
		cache.Set(jobName, true, ttlcache.DefaultTTL)
	}
	cacheMutex.Unlock()

	r.edgexSvc.LoggingClient().Debugf("jobDetails: %+v", jobDetails)
	simulationDefinitionName := jobDetails.SimulationDefinitionName
	statusCode := jobDetails.StatusCode

	r.edgexSvc.LoggingClient().Infof("Job status is %s", job.JobStatusMap[statusCode])
	completed := true
	if statusCode != job.TrainingCompleted {
		completed = false
	}

	if !completed {
		msg := fmt.Sprintf("Job status is %s", job.JobStatusMap[statusCode])
		r.edgexSvc.LoggingClient().Info(msg)
		return c.JSON(http.StatusExpectationFailed, msg)
	}

	err = r.Orchestrator.RunPredictions(simulationDefinitionName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, "Prediction in progress")
}
