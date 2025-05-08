package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/dto/ml_model"
)

func (r *Router) submitTrainingJob(c echo.Context) *echo.HTTPError {

	lc := r.service.LoggingClient()
	lc.Info("Request to submit a training job")

	// 1. Get Job Submission Details
	var jobSubmissionDetails job.JobSubmissionDetails
	err := json.NewDecoder(c.Request().Body).Decode(&jobSubmissionDetails)
	if err != nil {
		lc.Errorf("Failed to decode request body into JobSubmissionDetails struct  %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Error parsing request body")
	}

	// 2. Check whether algorithm is enabled
	hedgeErr := r.validateMlAlgorithmEnabled(
		jobSubmissionDetails.MLAlgorithm,
		fmt.Sprintf(
			"Submit training job for model config %s",
			jobSubmissionDetails.MLModelConfigName,
		),
	)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}

	// 3. Verify that trainer image exists in registry, validate digest value
	hedgeErr = r.validateTrainerImageDigest(jobSubmissionDetails.MLAlgorithm)
	switch {
	case hedgeErr != nil:
		// Bypass trainer image digest validation that fails due to timeouts
		return hedgeErr.ConvertToHTTPError()
	case len(jobSubmissionDetails.Name) == 0:
		lc.Errorf(
			"job name is empty for job: %s",
			jobSubmissionDetails.Name,
		)
		return echo.NewHTTPError(http.StatusConflict, "Job name should not be empty")
	}
	lc.Infof("Successfully validated trainer image for algo %s", jobSubmissionDetails.MLAlgorithm)

	// 4. Build the job
	job, jobErr := r.trainingJobService.BuildJobConfig(jobSubmissionDetails)
	if jobErr != nil {
		lc.Errorf("Failed to build job %s. Error: %v", jobSubmissionDetails.Name, jobErr)
		switch {
		case jobErr.IsErrorType(hedgeErrors.ErrorTypeNotFound):
			return echo.NewHTTPError(
				http.StatusNotFound,
				"Unable to build job, model config "+jobSubmissionDetails.MLModelConfigName+" not found",
			)
		default:
			return echo.NewHTTPError(
				http.StatusInternalServerError,
				"Failed to build job "+jobSubmissionDetails.Name,
			)
		}
	}

	// 5. Submit the job
	jobErr = r.trainingJobService.SubmitJob(job)
	if jobErr != nil {
		lc.Errorf("Failed to submit a training job: %v", err)
		return jobErr.ConvertToHTTPError()
	}

	// 6. Return result
	_ = c.NoContent(http.StatusAccepted)
	return nil
}

func (r *Router) getTrainingJobs(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().Info("Get training job by query")

	// filters
	algorithm := c.QueryParam("algorithm")
	jobStatus := c.QueryParam("jobStatus")
	mlModelConfigName := c.QueryParam("mlModelConfigName")

	// flag to join deployment status for each job
	includeDeployments := c.QueryParam("includeDeployments")
	if includeDeployments == "" {
		includeDeployments = "false"
	}

	includeDeployStatus, err := strconv.ParseBool(includeDeployments)
	if err != nil {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"invalid query param 'includeDeployments' value, must be 'true' or 'false'",
		)
	}

	// fetch deployments
	var deploymentsStatus []ml_model.ModelDeploymentStatus
	if includeDeployStatus {
		if deploymentsStatus, err = r.trainingJobService.GetModelDeploymentsByConfig(algorithm, mlModelConfigName, true); err != nil {
			return echo.NewHTTPError(
				http.StatusInternalServerError,
				"failed to get deployments for training jobs",
			)
		}
	}

	// fetch training jobs
	trainingJobs, err := r.trainingJobService.GetJobSummary(mlModelConfigName, jobStatus)
	if err != nil {
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"failed to get training job summary",
		)
	}

	// join for each training job all related deployments
	result := make([]*job.TrainingJobSummary, 0)
	for _, trainingJob := range trainingJobs {
		// filter by algorithm name if provided
		if algorithm != "" && algorithm != trainingJob.MLAlgorithm {
			continue
		}

		result = append(result, &trainingJob)
		for _, deployStatus := range deploymentsStatus {
			if trainingJob.Name == deployStatus.ModelName {
				// clear duplicates, training job already hold those values
				deployStatus.MLAlgorithm = ""
				deployStatus.ModelName = ""
				deployStatus.MLModelConfigName = ""

				trainingJob.Deployments = append(trainingJob.Deployments, deployStatus)
			}
		}
	}

	if err = c.JSON(http.StatusOK, result); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return nil
}

func (r *Router) deleteTrainingJob(c echo.Context) *echo.HTTPError {
	r.service.LoggingClient().
		Info("Delete training job")
	// alternative to edgexSdk.LoggingClient.Info("TEST")
	jobName := c.Param("jobName")
	jobDetails, err := r.trainingJobService.GetJob(jobName)
	if err != nil {
		r.service.LoggingClient().Error("Failed to get training job: " + err.Error())
		return echo.NewHTTPError(
			http.StatusInternalServerError,
			"Deleting training job failed. Failed to get its details",
		)
	}
	hedgeErr := r.validateMlAlgorithmEnabled(
		jobDetails.MLAlgorithm,
		fmt.Sprintf("Deleting training job %s", jobName),
	)
	if hedgeErr != nil {
		return hedgeErr.ConvertToHTTPError()
	}
	err = r.trainingJobService.DeleteMLTrainingJob(jobName)
	if err != nil {
		r.service.LoggingClient().Errorf("Failed to delete training job: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to delete training job")
	} else {
		_ = c.NoContent(http.StatusOK)
	}
	return nil
}
