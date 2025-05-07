/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package training

import (
	"hedge/edge-ml-service/pkg/dto/job"
)

type JobServiceProvider interface {
	GetTrainingJobStatus(job *job.TrainingJobDetails) error
	SubmitTrainingJob(jobConfig *job.TrainingJobDetails) error
	DownloadModel(localModelDirectory string, fileId string) error
	UploadFile(remoteFile string, localFile string) error
}
