package router

import (
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/labstack/echo/v4"
	"net/http"
)

func (r *Router) addMLRoutes() {
	r.addAddAlgorithmRoute()
	r.addUpdateAlgorithmRoute()
	r.addGetAlgorithmByNameRoute()
	r.addDeleteAlgorithmRoute()
	r.addChangeAlgorithmStatusRoute()
	r.addGetAlgorithmPredictionPortRoute()
	r.addGetPredictionTemplateByTypeRoute()
	r.addGetAlgorithmTypesRoute()
	r.addCreateMLModelConfigurationRoute()
	r.addUpdateMLModelConfigurationRoute()
	r.addGetModelConfigurationRoute()
	r.addDeleteModelConfigurationRoute()
	r.addUploadAndValidateTrainingDataRoute()
	r.addCreateTrainingDataExportRoute()
	r.addGetTrainingDataExportStatusRoute()
	r.addGetTrainingDataExportRoute()
	r.addCreateTrainingDataSampleRoute()
	r.addGetTrainingDataSampleStatusRoute()
	r.addGetTrainingDataSampleRoute()
	r.addSubmitTrainingJobRoute()
	r.addGetTrainingJobsRoute()
	r.addDeleteTrainingJobRoute()
	r.addGetModelsRoute()
	r.addUploadAndRegisterModelRoute()
	r.addDownloadModelFileRoute()
	r.addAddMLEventConfigurationRoute()
	r.addUpdateMLEventConfigurationRoute()
	r.addGetMLEventConfigurationRoute()
	r.addDeleteMLEventConfigurationRoute()
	r.addDeployOrUndeployModelRoute()
	r.addGetModelDeploymentStatusRoute()
	r.addGetTimeSeriesLabelsRoute()
}

// @Summary      Create and Register Algorithm
// @Description  Registers a new machine learning training algorithm.
// @Tags         ML Management - Algorithms
// @Param        generateDigest  query    bool                 false "Whether to generate a digest for the algorithm."
// @Param        Body            body     config.MLAlgorithmDefinition true "Algorithm definition details."
// @Success      201             "Algorithm successfully created."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm [post]
func (r *Router) addAddAlgorithmRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.addAlgorithm(c)
		},
		http.MethodPost)
}

// @Summary      Update Algorithm
// @Description  Updates an existing machine learning training algorithm.
// @Tags         ML Management - Algorithms
// @Param        generateDigest  query    bool                 false "Whether to regenerate the digest for the updated algorithm."
// @Param        Body            body     config.MLAlgorithmDefinition true "Updated algorithm definition details."
// @Success      201             "Algorithm successfully updated."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm [put]
func (r *Router) addUpdateAlgorithmRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.updateAlgorithm(c)
		},
		http.MethodPut)
}

// @Summary      Get Algorithm by Name
// @Description  Retrieves detailed information about a specific Machine Learning algorithm by its name.
// @Tags         ML Management - Algorithms
// @Param        algorithmName  path     string  true   "Algorithm Name"  // Name of the ML algorithm to retrieve
// @Param        summaryOnly    query    bool    false  "Summary Only"    // Whether to return only summary information (default: false)
// @Success      200            {object} config.MLAlgorithmDefinition "Algorithm details"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/{algorithmName} [get]
func (r *Router) addGetAlgorithmByNameRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/:algorithmName",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getAlgorithmByName(c)
		},
		http.MethodGet)
}

// @Summary      Delete Algorithm
// @Description  Deletes a machine learning algorithm by its name.
// @Tags         ML Management - Algorithms
// @Param        algorithmName  path     string  true   "Name of the algorithm to delete."
// @Success      200            {string} string  "Algorithm successfully deleted."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/{algorithmName} [delete]
func (r *Router) addDeleteAlgorithmRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/:algorithmName",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.deleteAlgorithm(c)
		},
		http.MethodDelete)
}

// @Summary      Change Algorithm Status
// @Description  Enables or disables a machine learning algorithm by its name.
// @Tags         ML Management - Algorithms
// @Param        algorithmName  path     string  true   "Name of the algorithm to update the status for."
// @Param        isEnabled      query    boolean true   "Boolean indicating whether to enable or disable the algorithm."
// @Success      200            {string} string  "Algorithm status successfully updated."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/{algorithmName}/status [patch]
func (r *Router) addChangeAlgorithmStatusRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/:algorithmName/status",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.changeAlgorithmStatus(c)
		},
		http.MethodPatch)
}

// @Summary      Get Algorithm Prediction Port
// @Description  Retrieve the prediction port associated with a specific machine learning algorithm.
// @Tags         ML Management - Algorithms
// @Param        algorithmName  path     string  true   "Name of the algorithm whose prediction port is being retrieved."
// @Success      200            {object} PredictionPortResponse  "Prediction port details."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/{algorithmName}/port [get]
func (r *Router) addGetAlgorithmPredictionPortRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/:algorithmName/port",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getAlgothmPredictionPort(c)
		},
		http.MethodGet)
}

// @Summary      Get Prediction Template By Type
// @Description  Retrieve a template for prediction data based on the specified algorithm type.
// @Tags         ML Management - Algorithms
// @Param        algorithmType  query    string  true   "Type of the algorithm for which the prediction template is being requested."
// @Success      200   {object} map[string]config.PredictionPayloadTemplate  "Prediction template details."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/data/template [get]
func (r *Router) addGetPredictionTemplateByTypeRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/data/template",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getPredictionTemplateByType(c)
		},
		http.MethodGet)
}

// @Summary      Get Algorithm Types
// @Description  Retrieve a list of supported machine learning algorithm types.
// @Tags         ML Management - Algorithms
// @Success      200   {array}  string  "List of supported algorithm types."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/data/types [get]
func (r *Router) addGetAlgorithmTypesRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/data/types",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getAlgorithmTypes(c)
		},
		http.MethodGet)
}

// @Summary      Create ML Model Configuration
// @Description  Create a new machine learning model configuration. This endpoint accepts a configuration for a new ML model, performs validation, checks for existing configurations, and then saves it.
// @Tags         ML Management - Model Configurations
// @Param        body  body     config.MLModelConfig  true   "ML Model Configuration to be created"
// @Param        sampleSize  query    string  false   "Sample size to be used for training. Defaults to 100."
// @Param        generatePreview  query    string  false   "Flag to indicate if a preview training sample should be generated. Accepts 'true' or 'yes'."
// @Success      201   {string}   string   "ML model configuration successfully created."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/config [post]
func (r *Router) addCreateMLModelConfigurationRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/config",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.createMLModelConfiguration(c)
		},
		http.MethodPost)
}

// @Summary      Update ML Model Configuration
// @Description  Updates an existing machine learning model configuration. This endpoint accepts an updated configuration for an existing ML model, performs validation, and saves the updated configuration.
// @Tags         ML Management - Model Configurations
// @Param        body  body     config.MLModelConfig  true   "Updated ML Model Configuration"
// @Param        sampleSize  query    string  false   "Sample size for training. Defaults to 100."
// @Param        generatePreview  query    string  false   "Flag indicating whether to generate a preview of the training sample. Accepts 'true' or 'yes'."
// @Success      200   {string}   string   "ML model configuration successfully updated."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/config [put]
func (r *Router) addUpdateMLModelConfigurationRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/config",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.updateMLModelConfiguration(c)
		},
		http.MethodPut)
}

// @Summary      Get ML Model Configuration
// @Description  Fetches the configuration for a specific machine learning model by algorithm name and model configuration name.
// @Tags         ML Management - Model Configurations
// @Param        algorithmName   path     string  true   "Name of the algorithm for which the model configuration is being requested."
// @Param        mlModelConfigName   path     string  true   "Name of the machine learning model configuration to retrieve."
// @Param        summaryOnly    query    bool    false  "Summary Only"    // Whether to return only summary information (default: false)
// @Success      200   {array}   []config.MLModelConfig  "The requested ML model configuration"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/{algorithmName}/config/{mlModelConfigName} [get]
func (r *Router) addGetModelConfigurationRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/:algorithmName/config/:mlModelConfigName",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getModelConfiguration(c)
		},
		http.MethodGet)
}

// @Summary      Delete ML Model Configuration
// @Description  Deletes a specific machine learning model configuration by algorithm name and model configuration name.
// @Tags         ML Management - Model Configurations
// @Param        algorithmName   path     string  true   "Name of the algorithm for which the model configuration will be deleted."
// @Param        mlModelConfigName   path     string  true   "Name of the machine learning model configuration to be deleted."
// @Success      200   {string}   "Model configuration deleted successfully"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/algorithm/{algorithmName}/config/{mlModelConfigName} [delete]
func (r *Router) addDeleteModelConfigurationRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/algorithm/:algorithmName/config/:mlModelConfigName",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.deleteModelConfiguration(c)
		},
		http.MethodDelete)
}

// @Summary      Upload and Validate Training Data
// @Description  Uploads and validates training data for a given machine learning model configuration and algorithm.
// @Tags         ML Management - Training Data
// @Param        mlModelConfigName  query   string  true   "Name of the ML model configuration to associate with the training data."
// @Param        mlAlgorithm        query   string  true   "Name of the machine learning algorithm to be used for validation."
// @Param        file               formData file    true   "Training data file to be uploaded and validated."
// @Success      202   {string}   "Training data successfully uploaded and validated"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_data/validate [post]
func (r *Router) addUploadAndValidateTrainingDataRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_data/validate",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.uploadAndValidateTrainingData(c)
		},
		http.MethodPost)
}

// @Summary      Generate a new training data file so it can be downloaded later
// @Description  Creates an asynchronous training data export for the specified machine learning model configuration and algorithm.
// @Tags         ML Management - Training Data
// @Param        jobSubmissionDetails  body   job.JobSubmissionDetails  true   "Job submission details for creating the training data export"
// @Param        generateConfigOnly     query  bool   false  "Flag to indicate whether only the configuration should be generated, without the actual training data export."
// @Success      202   {string}   "Training data export request accepted and being processed asynchronously."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_data/export [post]
func (r *Router) addCreateTrainingDataExportRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_data/export",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.createTrainingDataExport(c)
		},
		http.MethodPost)
}

// @Summary      Get Training Data Export Status
// @Description  Retrieves the status of a training data export job. Can be used to check the progress or completion of a previously requested export job.
// @Tags         ML Management - Training Data
// @Success      200   {object}  map[string]interface{}  "Returns the status of the training data export job"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_data/export/status [get]
func (r *Router) addGetTrainingDataExportStatusRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_data/export/status",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getTrainingDataExportStatus(c)
		},
		http.MethodGet)
}

// @Summary      Get Training Data Export
// @Description  Retrieves the exported training data for a specific model configuration and algorithm. Can be used to download the export or view export details.
// @Tags         ML Management - Training Data
// @Success      200   {file}  "Training data export file"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_data/export [get]
func (r *Router) addGetTrainingDataExportRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_data/export",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getTrainingDataExport(c)
		},
		http.MethodGet)
}

// @Summary      Create Training Data Sample
// @Description  Creates a sample of training data for a specific ML algorithm and model configuration. This request is asynchronous, and the training data sample creation will be processed in the background.
// @Tags         ML Management - Training Data
// @Param        mlModelConfigName  query  string  true  "Name of the machine learning model configuration"
// @Param        mlAlgorithm        query  string  true  "Name of the machine learning algorithm"
// @Param        sampleSize         query  string  false  "Size of the training data sample to create (default is 100)"
// @Success      202  {object}  string  "Accepted, the request to create the sample has been accepted"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_data/sample [post]
func (r *Router) addCreateTrainingDataSampleRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_data/sample",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.createTrainingDataSample(c)
		},
		http.MethodPost)
}

// @Summary      Get Training Data Sample Status
// @Description  Retrieves the current status of a training data sample process. This endpoint is used to check if the sample creation process has completed or is still running.
// @Tags         ML Management - Training Data
// @Success      200  {object}  string  "Status of the training data sample process"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_data/status [get]
func (r *Router) addGetTrainingDataSampleStatusRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_data/status",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getTrainingDataSampleStatus(c)
		},
		http.MethodGet)
}

// @Summary      Get Training Data Sample
// @Description  Retrieves a specific training data sample. This endpoint returns the details of the training data sample that was created.
// @Tags         ML Management - Training Data
// @Success      200  {object}  string  "The details of the training data sample"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_data [get]
func (r *Router) addGetTrainingDataSampleRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_data",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getTrainingDataSample(c)
		},
		http.MethodGet)
}

// @Summary      Submit a Training Job
// @Description  Submits a new training job to the system for execution. The request body should contain the details of the job, including the algorithm and model configuration. The job is validated and then submitted to the training system.
// @Tags         ML Management - Training Job
// @Param        jobSubmissionDetails  body  job.JobSubmissionDetails  true  "Details of the training job to be submitted"
// @Success      202  {string}  "Accepted, job submitted successfully"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_job [post]
func (r *Router) addSubmitTrainingJobRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_job",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.submitTrainingJob(c)
		},
		http.MethodPost)
}

// @Summary      Get Training Jobs
// @Description  Retrieves a list of training job summaries, optionally filtered by algorithm, job status, or model configuration name.
// @Tags         ML Management - Training Jobs
// @Param        algorithm          query    string  false  "Filter by algorithm name"
// @Param        jobStatus          query    string  false  "Filter by job status (e.g., completed, running)"
// @Param        mlModelConfigName  query    string  false  "Filter by ML Model configuration name"
// @Param        includeDeployments query    bool    false  "Include related deployment status for each training job (default: false)"
// @Success      200  {array}  job.TrainingJobSummary  "List of training jobs"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_job [get]
func (r *Router) addGetTrainingJobsRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_job",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getTrainingJobs(c)
		},
		http.MethodGet)
}

// @Summary      Delete a Training Job
// @Description  Deletes a specific training job by its name. This operation will remove the job and its associated data. The request requires the job name as a parameter in the URL.
// @Tags         ML Management - Training Job
// @Param        jobName  path  string  true  "The name of the training job to be deleted"
// @Success      200     {string}  "Successfully deleted the training job"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/training_job/{jobName} [delete]
func (r *Router) addDeleteTrainingJobRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/training_job/:jobName",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.deleteTrainingJob(c)
		},
		http.MethodDelete)
}

// @Summary      Get Trained Models
// @Description  Retrieves a list of trained models, optionally filtered by algorithm, configuration name, or version preference.
// @Tags         ML Management - Models
// @Param        mlAlgorithm        query    string  false  "Filter by ML algorithm name"
// @Param        mlModelConfigName  query    string  false  "Filter by ML Model configuration name"
// @Param        latestVersionsOnly query    bool    false  "Retrieve only the latest versions (default: true)"
// @Success      200                {array}  ml_model.MLModel  "List of trained models"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/model [get]
func (r *Router) addGetModelsRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/model",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getModels(c)
		},
		http.MethodGet)
}

// @Summary      Upload and Register Trained Model
// @Description  Uploads a trained model file and registers it with the specified training job and model configuration.
// @Tags         ML Management - Model
// @Accept       multipart/form-data
// @Produce      json
// @Param        mlModelConfigName  query  string  true  "Model Configuration Name"
// @Param        mlAlgorithm        query  string  true  "ML Algorithm"
// @Param        jobName            query  string  true  "Job Name"
// @Param        description        query  string  false "Job Description"
// @Param        modelFile          formData file  true "Trained model file to upload and register"
// @Success      201     {object}  nil      "Model uploaded and registered successfully"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/model/modelFile [post]
func (r *Router) addUploadAndRegisterModelRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/model/modelFile",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.uploadAndRegisterModel(c)
		},
		http.MethodPost)
}

// @Summary      Download Trained Model File
// @Description  Downloads a trained model file based on the provided ML model configuration and algorithm.
// @Tags         ML Management - Models
// @Accept       json
// @Produce      octet-stream
// @Param        mlModelConfigName query    string  true  "The ML Model Configuration Name (required)"
// @Param        mlAlgorithm       query    string  true  "The ML Algorithm Name (required)"
// @Success      200               "Binary file of the trained model"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/model/modelFile [get]
func (r *Router) addDownloadModelFileRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/model/modelFile",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.downloadModelFile(c)
		},
		http.MethodGet)
}

// @Summary      Add Machine Learning Event Configuration
// @Description  Creates and adds a new Machine Learning event configuration. This configures a specific event for a given ML model and algorithm.
// @Tags         ML Management - Event Configuration
// @Param        config  body  config.MLEventConfig  true  "Machine Learning Event Configuration"
// @Success      201     {object}  config.MLEventConfig  "ML Event Configuration created successfully"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/event_config [post]
func (r *Router) addAddMLEventConfigurationRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/event_config",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.addMLEventConfiguration(c)
		},
		http.MethodPost)
}

// @Summary      Update Machine Learning Event Configuration
// @Description  Updates an existing Machine Learning event configuration for a given ML model and event name. This allows modification of an existing event configuration.
// @Tags         ML Management - Event Configuration
// @Param        eventName  path  string  true  "Event Name"  // The name of the ML event to be updated
// @Param        config     body  config.MLEventConfig  true  "Machine Learning Event Configuration"
// @Success      201        {object}  config.MLEventConfig  "ML Event Configuration updated successfully"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/event_config/{eventName} [put]
func (r *Router) addUpdateMLEventConfigurationRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/event_config/:eventName",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.updateMLEventConfiguration(c)
		},
		http.MethodPut)
}

// @Summary      Get ML Event Configuration
// @Description  Fetches the event configuration for a specific ML model configuration, algorithm, and event.
// @Tags         ML Management - Event Configuration
// @Param        mlModelConfigName query    string  true  "The ML Model Configuration Name (required)"
// @Param        mlAlgorithm       query    string  true  "The ML Algorithm Name (required)"
// @Param        eventName         query    string  true  "The Event Name (required)"
// @Success      200               {object} config.MLEventConfig  "ML Event Configuration"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/event_config [get]
func (r *Router) addGetMLEventConfigurationRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/event_config",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getMLEventConfiguration(c)
		},
		http.MethodGet)
}

// @Summary      Delete Machine Learning Event Configuration
// @Description  Deletes a specific Machine Learning event configuration based on provided ML model configuration name, algorithm, and event name.
// @Tags         ML Management - Event Configuration
// @Param        mlModelConfigName  query  string  true  "ML Model Configuration Name"  // The name of the ML model configuration
// @Param        mlAlgorithm        query  string  true  "ML Algorithm"                // The algorithm associated with the model
// @Param        eventName          query  string  true  "Event Name"                  // The name of the event to delete
// @Success      200                {string}  "ML Event Configuration deleted successfully"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/event_config [delete]
func (r *Router) addDeleteMLEventConfigurationRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/event_config",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.deleteMLEventConfiguration(c)
		},
		http.MethodDelete)
}

// @Summary      Deploy or Undeploy Machine Learning Model
// @Description  Deploys or undeploys a trained Machine Learning model to target nodes based on the provided deployment command.
// @Tags         ML Management - Model Deployment
// @Param        modelDeployCommand  body  ml_model.ModelDeployCommand  true  "Model Deployment Command"  // Command details for deploying or undeploying the model
// @Success      202                 {string}  "Model deployment request accepted"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/model_deployment [post]
func (r *Router) addDeployOrUndeployModelRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/model_deployment",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.deployOrUndeployModel(c)
		},
		http.MethodPost)
}

// @Summary      Get Model Deployment Status
// @Description  Retrieves the status of model deployments for a specific ML model configuration and algorithm.
// @Tags         ML Management - Model Deployment
// @Accept       json
// @Produce      json
// @Param        mlModelConfigName  query    string  true   "The ML Model Configuration Name (required)"
// @Param        mlAlgorithm        query    string  true   "The ML Algorithm Name (required)"
// @Param        deploymentStatus   query    string  false  "Filter by deployment status (optional)"
// @Param        groupByNodes       query    bool    false  "Group deployment status by nodes (default: true)"
// @Success      200                {array}  ml_model.ModelDeploymentStatus  "List of model deployment statuses"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/model_deployment [get]
func (r *Router) addGetModelDeploymentStatusRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/model_deployment",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getModelDeploymentStatus(c)
		},
		http.MethodGet)
}

// @Summary      Get Time Series Labels
// @Description  Fetches time series labels for a given label identifier.
// @Tags         ML Management - Time Series Data
// @Param        label  path  string  true  "Label identifier"
// @Success      200    {array}  string  "List of time series labels"
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			409			{object}	error	"{"message":"Error message"}"
// @Failure			417			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/ml_management/tsdata/label/{label} [get]
func (r *Router) addGetTimeSeriesLabelsRoute() {
	_ = r.service.AddCustomRoute(
		"/api/v3/ml_management/tsdata/label/:label",
		interfaces.Authenticated,
		func(c echo.Context) error {
			return r.getTimeSeriesLabels(c)
		},
		http.MethodGet)
}
