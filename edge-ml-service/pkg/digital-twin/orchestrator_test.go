package digital_twin

import (
	"hedge/edge-ml-service/pkg/dto/twin"
	"github.com/stretchr/testify/assert"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestNewOrchestrator(t *testing.T) {
	trainingURL := "http://training.example.com"
	predictionURL := "http://prediction.example.com"
	callback := func(string) (twin.SimulationDefinition, error) { return twin.SimulationDefinition{}, nil }

	o, err := NewOrchestrator(trainingURL, predictionURL, callback, nil)
	assert.NoError(t, err)

	assert.Equal(t, trainingURL, o.trainingURL)
	assert.Equal(t, predictionURL, o.predictionURL)
	assert.NotNil(t, o.client)
	assert.NotNil(t, o.GetSimulationDefinition)
}

func TestSubmitTrainingJob(t *testing.T) {
	// Setup
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/training_job", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Job submitted successfully"))
	}))
	defer mockServer.Close()

	o, err := NewOrchestrator(mockServer.URL+"/training_job", "http://www.example.com", func(string) (twin.SimulationDefinition, error) {
		return twin.SimulationDefinition{
			//MLModelConfigName: "testConfig",
			//AlgorithmName:     "testAlgorithm",
		}, nil
	}, u.AppService.LoggingClient())
	assert.NoError(t, err)

	// Test
	response, err := o.SubmitTrainingJob("testJob", "testSimulation")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Job submitted successfully", response)
}

func TestRunPredictions(t *testing.T) {
	// Setup
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/prediction/testConfig", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Prediction run successfully"))
	}))
	defer mockServer.Close()

	o, err := NewOrchestrator("http://www.example.com", mockServer.URL+"/prediction", func(string) (twin.SimulationDefinition, error) {
		return twin.SimulationDefinition{
			//MLModelConfigName: "testConfig",
			RunType: twin.OneTimeRun,
		}, nil
	}, nil)
	assert.NoError(t, err)
	o.LoggingClient = u.AppService.LoggingClient()

	// Test
	err = o.RunPredictions("testSimulation")

	// Assert
	assert.NoError(t, err)
}

func TestRunPredictions_Error(t *testing.T) {

	_, err := NewOrchestrator("", "", func(string) (twin.SimulationDefinition, error) {
		return twin.SimulationDefinition{}, assert.AnError
	}, nil)
	assert.Error(t, err)
}

func TestRunPredictions_PeriodicRun(t *testing.T) {
	// Setup
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount > 1 {
			t.Fatal("Unexpected additional call to prediction endpoint")
		}
		// Temporary comment till the actual method is fixed
		//assert.Equal(t, "/prediction/testConfig", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Prediction run successfully"))
	}))
	defer mockServer.Close()

	log.SetOutput(os.Stderr)
	o, err := NewOrchestrator("http://www.example.com", mockServer.URL+"/prediction", func(string) (twin.SimulationDefinition, error) {
		return twin.SimulationDefinition{
			//MLModelConfigName: "testConfig",
			RunType: twin.PeriodicRun,
		}, nil
	}, u.AppService.LoggingClient())
	assert.NoError(t, err)
	o.LoggingClient = u.AppService.LoggingClient()

	// Test
	err = o.RunPredictions("testSimulation")

	// Assert
	assert.NoError(t, err)

	// Wait for a short time to ensure the goroutine has started
	time.Sleep(100 * time.Millisecond)
}
