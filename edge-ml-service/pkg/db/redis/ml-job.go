/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	hedgeErrors "hedge/common/errors"
	"hedge/edge-ml-service/pkg/dto/job"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"

	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	"github.com/gomodule/redigo/redis"
)

func (dbClient *DBClient) AddMLTrainingJob(trainingJob job.TrainingJobDetails) (string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()
	lc := dbClient.client.Logger

	jobName, err := addMLTrainingJob(conn, lc, trainingJob)
	return jobName, err
}

func (dbClient *DBClient) UpdateMLTrainingJob(trainingJob job.TrainingJobDetails) (string, hedgeErrors.HedgeError) {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	lc := dbClient.client.Logger

	err := deleteMLTrainingJob(conn, lc, trainingJob)
	if err != nil {
		return "", err
	}

	jobName, err := addMLTrainingJob(conn, lc, trainingJob)

	return jobName, err
}

func addMLTrainingJob(conn redis.Conn, lc logger.LoggingClient, trainingJob job.TrainingJobDetails) (string, hedgeErrors.HedgeError) {
	errorMessage := fmt.Sprintf("Failed to add ML training job: %s", trainingJob.Name)

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.MLTrainingJob+":name", trainingJob.Name))
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	} else if exists {
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConflict, fmt.Sprintf("Training %s already exists", trainingJob.Name))
	}
	trainingJob.Status = trainingJob.StatusCode.String()

	m, err := marshalObject(trainingJob)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", trainingJob.Name, m)
	_ = conn.Send("SADD", db2.MLTrainingJob, trainingJob.Name)
	_ = conn.Send("HSET", db2.MLTrainingJob+":name", trainingJob.Name, trainingJob.Name)
	_ = conn.Send("SADD", db2.MLTrainingJob+":config:"+trainingJob.MLModelConfigName, trainingJob.Name)

	_, err = conn.Do("EXEC")
	if err != nil {
		lc.Errorf("%s: error saving anomaly training job in DB: %v", errorMessage, err)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	return trainingJob.Name, nil
}

func (dbClient *DBClient) GetMLTrainingJob(id string) (job.TrainingJobDetails, hedgeErrors.HedgeError) {
	lc := newLoggingClient()

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	var trainingJob job.TrainingJobDetails
	err := redis2.GetObjectById(conn, id, unmarshalObject, &trainingJob)
	if err != nil {
		lc.Errorf("Error While Get ML Training Job: %s", err.Error())
		if errors.Is(err, db2.ErrNotFound) {
			return trainingJob, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("ML training job %s not found", id))
		}

		return trainingJob, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Failed to get ML training job %s", id))
	}

	return trainingJob, nil
}

// GetMLTrainingJobsByConfig Optional filter for jobStatus, if this is empty, jobStatus filter is not applied
func (dbClient *DBClient) GetMLTrainingJobsByConfig(trainingConfigName string, jobStatus string) ([]job.TrainingJobDetails, hedgeErrors.HedgeError) {
	lc := dbClient.client.Logger

	errorMessage := fmt.Sprintf("Failed to get ML training jobs by config: %s", trainingConfigName)

	jobKey := db2.MLTrainingJob
	if trainingConfigName != "" {
		jobKey += ":config:" + trainingConfigName
	}
	filteredJobs := make([]job.TrainingJobDetails, 0)
	trainingJobs, err := dbClient.GetMLTrainingJobs(jobKey)
	if err != nil {
		lc.Errorf("%s: %v", errorMessage, err)
		return filteredJobs, err
	}
	if jobStatus == "" {
		return trainingJobs, nil
	}

	for _, trainingJob := range trainingJobs {
		if trainingJob.Status == jobStatus {
			filteredJobs = append(filteredJobs, trainingJob)
		}
	}
	return filteredJobs, nil
}

func (dbClient *DBClient) MarkOldTrainingDataDeprecated(trainingConfigName string, currentJobName string) hedgeErrors.HedgeError {
	jobs, err := dbClient.GetMLTrainingJobsByConfig(trainingConfigName, "TrainingDataCollected")
	if err != nil || jobs == nil {
		return err
	}
	for _, jb := range jobs {
		if jb.Status == "TrainingDataCollected" && jb.Name != currentJobName {
			jb.StatusCode = job.Deprecated
			jb.Status = job.Deprecated.String()
			jb.Msg = "Training data file overwritten by new version"
			dbClient.UpdateMLTrainingJob(jb)
		}
	}
	return nil
}

func (dbClient *DBClient) GetMLTrainingJobs(key string) ([]job.TrainingJobDetails, hedgeErrors.HedgeError) {
	lc := dbClient.client.Logger

	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	errorMessage := fmt.Sprintf("Failed to get ML training jobs for key %s", key)

	objects, err := redis2.GetObjectsByValue(conn, key)
	if err != nil {
		lc.Errorf("Error While Get Anomaly Training Jobs from DB: %v", err)
		return []job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, errorMessage)
	}

	d := make([]job.TrainingJobDetails, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &d[i])
		if err != nil {
			lc.Errorf("%s: Error unmarshal anomaly training job: %v", errorMessage, err)
			return []job.TrainingJobDetails{}, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
		}
	}

	return d, nil
}

func (dbClient *DBClient) DeleteMLTrainingJobs(jobName string) hedgeErrors.HedgeError {
	conn := dbClient.client.Pool.Get()
	defer conn.Close()

	lc := dbClient.client.Logger

	trainingJob, err := dbClient.GetMLTrainingJob(jobName)

	if err != nil {
		lc.Errorf("Error deleting ML training job %s: %v", jobName, err)
		return err
	}

	err = deleteMLTrainingJob(conn, lc, trainingJob)
	if err != nil {
		lc.Errorf("Error deleting ML training job %s: %v", jobName, err)
		return err
	}

	return nil
}

func deleteMLTrainingJob(conn redis.Conn, lc logger.LoggingClient, trainingJob job.TrainingJobDetails) hedgeErrors.HedgeError {
	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", trainingJob.Name)
	_ = conn.Send("SREM", db2.MLTrainingJob, trainingJob.Name)
	_ = conn.Send("HDEL", db2.MLTrainingJob+":name", trainingJob.Name)
	_ = conn.Send("SREM", db2.MLTrainingJob+":config:"+trainingJob.MLModelConfigName, trainingJob.Name)

	_, err := conn.Do("EXEC")
	if err != nil {
		lc.Errorf("Error While Deleting Anomaly Training Job in DB: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeDBError, fmt.Sprintf("Failed to delete ML training job %s", trainingJob.Name))
	}

	return nil
}
