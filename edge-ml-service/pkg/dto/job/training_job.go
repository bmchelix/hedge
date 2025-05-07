/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package job

import (
	"hedge/edge-ml-service/pkg/dto/ml_model"
	"time"

	"hedge/edge-ml-service/pkg/dto/config"
)

type JobStatus int

const (
	New JobStatus = iota
	TrainingDataCollected
	UploadingTrainingData
	TrainingInProgress
	TrainingCompleted
	Failed
	Cancelled
	Terminated
	Deprecated
)

var JobStatusMap = map[JobStatus]string{
	TrainingDataCollected: "Training Data Collected",
	UploadingTrainingData: "Uploading Training Data",
	TrainingInProgress:    "Training In Progress",
	TrainingCompleted:     "Training Completed",
	Failed:                "Failed",
	Cancelled:             "Cancelled",
	Terminated:            "Terminated",
	Deprecated:            "Deprecated",
}

const (
	TrainingDataCollecting = "training"
	ExportDataCollecting   = "export"
)

func (js JobStatus) String() string {
	return [...]string{"New", "TrainingDataCollected", "UploadingTrainingData", "TrainingInProgress", "TrainingCompleted", "Failed", "Cancelled", "Terminated", "Deprecated"}[js]
}

type JobSubmissionDetails struct {
	Name               string `json:"name"`
	Description        string `json:"description"`
	MLAlgorithm        string `json:"mlAlgorithm"`
	MLModelConfigName  string `json:"mlModelConfigName"`
	UseUploadedData    bool   `json:"useUploadedData,omitempty"`
	NotifyOnCompletion bool   `json:"notifyOnCompletion,omitempty"`
	// TODO: Vijay, Consider removing SimulationDefinitionName, so far didn't find any use for it
	SimulationDefinitionName string `json:"simulationDefinitionName,omitempty"`
}

type TrainingJobSummary struct {
	Name                 string                           `json:"name"`
	MLAlgorithm          string                           `json:"mlAlgorithm"`
	MLModelConfigName    string                           `json:"mlModelConfigName"`
	TrainingProviderName string                           `json:"trainingProviderName"`
	Status               string                           `json:"status,omitempty"`
	StartTime            int64                            `json:"startTime,omitempty"`
	EndTime              int64                            `json:"endTime,omitempty"`
	Msg                  string                           `json:"msg,omitempty"`
	ModelNameVersion     string                           `json:"modelNameVersion,omitempty"`
	EstimatedDuration    int64                            `json:"estimatedDuration,omitempty"`
	Deployments          []ml_model.ModelDeploymentStatus `json:"deployments,omitempty"`
}

type TrainingJobDetails struct {
	Name              string                `json:"name"`
	MLAlgorithm       string                `json:"mlAlgorithm"`
	MLModelConfigName string                `json:"mlModelConfigName"`
	MLModelConfig     *config.MLModelConfig `json:"-"`
	Enabled           bool                  `json:"enabled,omitempty"` //Yes,No
	StatusCode        JobStatus             `json:"statusCode,omitempty"`
	Status            string                `json:"status,omitempty"`
	StartTime         int64                 `json:"startTime,omitempty"`
	EndTime           int64                 `json:"endTime,omitempty"`
	Msg               string                `json:"msg,omitempty"`
	ModelNameVersion  string                `json:"modelNameVersion,omitempty"`

	// Training Job configuration
	JobServiceConfig interface{}

	UseUploadedData bool `json:"useUploadedData,omitempty"`

	// Notifications
	NotifyOnCompletion bool `json:"notifyOnCompletion,omitempty"`
	// TODO: Vijay, Check if really need SimulationDefinitionName & if we can remove it
	SimulationDefinitionName string `json:"simulationDefinitionName,omitempty"`

	// Specific to GCP or probably other cloud providers
	BucketName        string   `json:"bucketName,omitempty"`
	JobDir            string   `json:"jobDir,omitempty"`
	JobRegion         string   `json:"jobRegion,omitempty"`
	PackageUris       []string `json:"packageUris,omitempty"`
	ModelName         string   `json:"modelName,omitempty"`
	TrainingFramework string   `json:"trainingFramework,omitempty"` // tensorflow
	RuntimeVersion    string   `json:"runtimeVersion,omitempty"`
	PythonModule      string   `json:"pythonModule,omitempty"`
	PythonVersion     string   `json:"pythonVersion,omitempty"`
	//TrainingDataFileName string   `json:"trainingDataFileName,omitempty"`
	GCPProjectName string `json:"gcpPojectName,omitempty"`

	ModelDir          string `json:"modelDir,omitempty"`
	ModelDownloadDir  string `json:"modelDownloadUrl,omitempty"`
	EstimatedDuration int64  `json:"estimatedDuration,omitempty"`

	TrainingDataFileId string `json:"trainingDataFileId,omitempty"`
}

// Generic Job across all training providers
func NewTrainingJob(jobName string, mlModelConfig *config.MLModelConfig) *TrainingJobDetails {
	job := new(TrainingJobDetails)
	job.MLAlgorithm = mlModelConfig.MLAlgorithm
	job.Name = jobName
	job.ModelName = jobName
	job.MLModelConfig = mlModelConfig
	job.ModelDir = "hedge_export"

	job.ModelDownloadDir = job.MLModelConfigName + "/" + job.ModelDir
	// Generate a new ml_model name by appending timestamp
	job.StatusCode = New
	//ml_model.GCPProjectName = trainingConfig.TrainingProviderDetails.ProjectName //"projects/protean-keyword-292215", read this from auth key json file
	//start Time of Job
	job.StartTime = time.Now().Unix()
	job.MLModelConfigName = mlModelConfig.Name
	return job
}
