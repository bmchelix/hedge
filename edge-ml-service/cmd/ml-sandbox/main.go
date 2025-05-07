/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package main

import (
	"hedge/common/client"
	"hedge/edge-ml-service/pkg/docker"
	"hedge/edge-ml-service/pkg/dto/job"
	"hedge/edge-ml-service/pkg/helpers"
	"encoding/json"
	"fmt"
	"os"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

type Trainer struct {
	registry helpers.ImageRegistryConfigInterface
	manager  docker.ContainerManager
}

func main() {
	svc, ok := pkg.NewAppServiceWithTargetType(client.HedgeMLSandboxServiceKey, &job.HedgeTrainJob{})
	if !ok {
		fmt.Errorf("failed to start service: %s\n", client.HedgeMLSandboxServiceKey)
		os.Exit(-1)
	}

	jobDir, err := svc.GetAppSetting("JobDir")
	if err != nil {
		svc.LoggingClient().Errorf("topic name is required in app setting, got error: error:%v", err)
		os.Exit(-1)
	}

	trainer := Trainer{
		registry: helpers.NewImageRegistryConfig(svc),
	}
	trainer.registry.LoadRegistryConfig()

	creds := trainer.registry.GetRegistryCredentials()
	trainer.manager, err = docker.NewDockerContainerManager(svc, jobDir, creds.RegistryURL, creds.UserName, creds.Password, false)
	if err != nil {
		svc.LoggingClient().Errorf("failed on docker container manager creation, got error: error:%v", err)
		os.Exit(-1)
	}

	err = svc.SetDefaultFunctionsPipeline(trainer.Train)
	if err != nil {
		svc.LoggingClient().Errorf("failed on pipline creation returned error: %s", err.Error())
		os.Exit(-1)
	}

	if err := svc.Run(); err != nil {
		svc.LoggingClient().Errorf("service run failed, returned error: %s", err.Error())
		os.Exit(-1)
	}
	return
}

func (t *Trainer) Train(ctx interfaces.AppFunctionContext, jobI interface{}) (bool, interface{}) {

	trainingJob, ok := jobI.(job.HedgeTrainJob)
	if !ok {
		ctx.LoggingClient().Errorf("failed to parse training job request, returned error: %s", jobI)
		return false, nil
	}
	hedgeTrgJobStatus := job.HedgeJobStatus{
		JobName: trainingJob.JobName,
		Status:  "UNKNOWN",
		Message: "",
	}

	id, err := t.manager.CreateTrainingContainer(trainingJob.ImagePath, trainingJob.JobName, trainingJob.DataFile)
	/*	hedgeTrgJobStatus.Status = "SUCCESSFUL"
		hedgeTrgJobStatus.Message = "Successfully completed the training job"
		SetResponseStatus(ctx, hedgeTrgJobStatus)
	return true, nil */
	if err != nil {
		errMsg := fmt.Sprintf("failed to create training container for job '%s' with image '%s', returned error: %s", trainingJob.JobName, trainingJob.ImagePath, err)
		ctx.LoggingClient().Errorf(errMsg)
		hedgeTrgJobStatus.Status = "FAILED"
		hedgeTrgJobStatus.Message = errMsg
		SetResponseStatus(ctx, hedgeTrgJobStatus)
		return false, nil
	}

	// ensure we remove the container if it got created
	defer func(containerID string) {
		if containerID != "" {
			//ctx.LoggingClient().Infof("about to remove the container: %s", containerID)
			err := t.manager.RemoveContainerById(containerID)
			if err != nil {
				ctx.LoggingClient().Errorf("failed to remove '%s' container after successful training error: %v", containerID, err)
			}
		}
	}(id)

	err = t.manager.RunTrainingContainer(trainingJob.JobName, id)
	if err != nil {
		errMsg := fmt.Sprintf("failed to start training container for job '%s', returned error: %s", trainingJob.JobName, err)
		ctx.LoggingClient().Errorf(errMsg)
		hedgeTrgJobStatus.Status = "FAILED"
		hedgeTrgJobStatus.Message = errMsg
		SetResponseStatus(ctx, hedgeTrgJobStatus)
		return false, nil
	}

	hedgeTrgJobStatus.Status = "SUCCESSFUL"
	hedgeTrgJobStatus.Message = "Successfully completed the training job"
	SetResponseStatus(ctx, hedgeTrgJobStatus)
	return true, nil
}

func SetResponseStatus(ctx interfaces.AppFunctionContext, hedgeJobStatus job.HedgeJobStatus) {
	jobStatusBytes, _ := json.Marshal(hedgeJobStatus)
	ctx.SetResponseData(jobStatusBytes)
}
