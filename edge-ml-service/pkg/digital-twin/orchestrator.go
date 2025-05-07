package digital_twin

import (
	"hedge/edge-ml-service/pkg/dto/twin"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hedge/edge-ml-service/pkg/dto/job"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
)

type OrchestratorInterface interface {
	SubmitTrainingJob(jobName, simulationDefinitionName string) (string, error)
	RunPredictions(simulationDefinitionName string) error
}

type Orchestrator struct {
	trainingURL             string
	predictionURL           string
	client                  *http.Client
	LoggingClient           logger.LoggingClient
	GetSimulationDefinition func(string) (twin.SimulationDefinition, error)
}

func NewOrchestrator(
	trainingURL string,
	predictionURL string,
	callback func(string) (twin.SimulationDefinition, error),
	loggingClient logger.LoggingClient,
) (*Orchestrator, error) {

	if !strings.Contains(predictionURL, "http") || !strings.Contains(trainingURL, "http") {
		return nil, errors.New("predictionURL and trainingURL must be valid")
	}
	return &Orchestrator{
		trainingURL:             trainingURL,
		predictionURL:           predictionURL,
		client:                  http.DefaultClient,
		GetSimulationDefinition: callback,
		LoggingClient:           loggingClient,
	}, nil
}

func (o *Orchestrator) SubmitTrainingJob(jobName, simulationDefinitionName string) (string, error) {

	//Get simulation definition data
	simulationDefinition, err := o.GetSimulationDefinition(simulationDefinitionName)
	if err != nil {
		return "", err
	}
	//TODO: Get mlConfigName and algoName from simulationDefinition struct
	configName := "simulationDefinition.MLModelConfigName" // Same as mlModelConfigName -- contains Feature Definitions etc.
	algorithmName := "simulationDefinition.AlgorithmName"  // like AutoEncoder

	o.LoggingClient.Infof("MLModelConfig: %s, Algorithm: %s", configName, algorithmName)
	o.LoggingClient.Infof("simulation definition: %+v", simulationDefinition)
	payload := job.JobSubmissionDetails{
		Name:                     jobName,
		Description:              "Training job " + "for " + jobName,
		MLAlgorithm:              algorithmName,
		MLModelConfigName:        configName,
		NotifyOnCompletion:       true,
		SimulationDefinitionName: simulationDefinition.Name,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	trainingURL := o.trainingURL + "?notify=true" + "&simulationDefinitionName=" + simulationDefinitionName
	// Submit job
	reader := bytes.NewReader(data)
	resp, err := o.client.Post(trainingURL, "Content-Type: application/json", reader)
	if err != nil {
		return "", err
	}

	// Read and return response
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func (o *Orchestrator) RunPredictions(simulationDefinitionName string) error {

	simulationDefinition, err := o.GetSimulationDefinition(simulationDefinitionName)
	//Handle error
	if err != nil {
		o.LoggingClient.Errorf("Error getting simulation definition from DB: %s", err.Error())
		return err
	}

	// TODO: Make sure that multiple predictions aren't kicked off in parallel.
	go func(predictionURL string, simulationDefinition twin.SimulationDefinition, logClient logger.LoggingClient) {
		for {

			//TODO: Get MLModelConfigName from simulationDefinition
			response, err := http.Get(predictionURL + "/" + "simulationDefinition.MLModelConfigName")
			if err != nil {
				logClient.Errorf("Error sending prediction request to Broker: %s", err.Error())
				time.Sleep(60 * time.Minute)
				continue
			}

			body, err := io.ReadAll(response.Body)
			if err != nil {
				fmt.Printf("error: %s\n", err)
				logClient.Errorf("Error reading response from Broker: %s", err.Error())
			} else {
				logClient.Infof("reading response from Broker: %s", string(body))
			}

			if simulationDefinition.RunType == twin.PeriodicRun {
				//TODO: Make sleep time variable
				time.Sleep(60 * time.Minute)
			} else {
				break
			}
			response.Body.Close()
		}
	}(o.predictionURL, simulationDefinition, o.LoggingClient)

	return nil

}
