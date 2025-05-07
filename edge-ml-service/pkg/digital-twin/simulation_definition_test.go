package digital_twin

import (
	"hedge/edge-ml-service/pkg/dto/twin"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"hedge/mocks/hedge/edge-ml-service/pkg/db/redis"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	u                 *utils.HedgeMockUtils
	mockedTwinDBLayer *redis.MockTwinDB
)

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{})
	mockedTwinDBLayer = &redis.MockTwinDB{}
}

func TestGetSimulationDefinitionFromDB(t *testing.T) {
	// Setup test data
	definitionName := "test-definition"
	simulationDefinition := &twin.SimulationDefinition{
		Name: "test-definition",
	}
	data, _ := json.Marshal(simulationDefinition)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/getdefinition", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/getDefinition/:name")
	c.SetParamNames("name")
	c.SetParamValues(definitionName)

	var err error

	t.Run("Test the GetSimulationDefinitionFromDB function with a valid simulation definition name", func(t *testing.T) {
		// Test the GetSimulationDefinitionFromDB function with a valid simulation definition name
		mockedTwinDBLayer = &redis.MockTwinDB{}
		mockedTwinDBLayer.On("DBGetSimulationDefinition", mock.Anything).Return(string(data), nil)
		// Create a DigitalTwinService instance with the mocks
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}

		result, err := digitalTwinService.GetSimulationDefinitionFromDB(c)
		assert.Nil(t, err)
		assert.Equal(t, definitionName, result.Name)
		assert.Equal(t, simulationDefinition.Name, result.SimulationDefinition.Name)
	})

	t.Run("Test the Getsimulationdefinitionfromdb function with an error during DB retrieval", func(t *testing.T) {
		mockedTwinDBLayer = &redis.MockTwinDB{}
		mockedTwinDBLayer.On("DBGetSimulationDefinition", mock.Anything).Return("", errors.New("error connecting to DB"))
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}
		_, err = digitalTwinService.GetSimulationDefinitionFromDB(c)
		assert.NotNil(t, err)
	})

	t.Run("Test with invalid simulation definition in DB", func(t *testing.T) {
		mockedTwinDBLayer = &redis.MockTwinDB{}
		mockedTwinDBLayer.On("DBGetSimulationDefinition", mock.Anything).Return("test123", nil)
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}

		_, err = digitalTwinService.GetSimulationDefinitionFromDB(c)
		assert.NotNil(t, err)
	})

}

func TestGetAllSimulationDefinitions(t *testing.T) {

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/getalldefinitions", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	data := []string{"definitionA", "definitionB"}

	t.Run("Get Simulation definitions without errors", func(t *testing.T) {
		mockedTwinDBLayer.On("DBGetSimulationDefinitions", mock.Anything).Return(data, nil)
		// Create a DigitalTwinService instance with the mocks
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}

		result, err := digitalTwinService.GetAllSimulationDefinitions(c)
		assert.NoError(t, err)
		assert.Equal(t, result.Keys[0], "definitionA")
		assert.Equal(t, result.Keys[1], "definitionB")
	})

	t.Run("Get Simulation definitions with error", func(t *testing.T) {
		mockedTwinDBLayer = &redis.MockTwinDB{}
		mockedTwinDBLayer.On("DBGetSimulationDefinitions", mock.Anything).Return([]string{}, errors.New("Could not retrieve definitions from DB"))
		// Create a DigitalTwinService instance with the mocks
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}
		_, err := digitalTwinService.GetAllSimulationDefinitions(c)
		assert.Error(t, err)
	})
}

func TestDeleteSimulationDefinition(t *testing.T) {

	definitionName := "definitionA"
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/deletedefinition", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/deletedefinition/:name")
	c.SetParamNames("name")
	c.SetParamValues(definitionName)

	t.Run("Get Simulation definitions without errors", func(t *testing.T) {
		mockedTwinDBLayer.On("DBDeleteSimulationDefinition", mock.Anything).Return(nil)
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}

		result, err := digitalTwinService.DeleteSimulationDefinition(c)
		assert.NoError(t, err)
		assert.Equal(t, result.Name, definitionName)
	})

	t.Run("Get Simulation definitions with error", func(t *testing.T) {
		mockedTwinDBLayer = &redis.MockTwinDB{}
		mockedTwinDBLayer.On("DBDeleteSimulationDefinition", mock.Anything).Return(errors.New("Could not delete definition in DB"))
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}
		_, err := digitalTwinService.DeleteSimulationDefinition(c)
		assert.Error(t, err)
	})
}

func TestAddSimulationDefinition(t *testing.T) {

	// Setup test data
	definitionName := "test-definition"
	simulationDefinition := &twin.SimulationDefinition{
		Name: definitionName,
	}
	data, _ := json.Marshal(simulationDefinition)

	t.Run("Add simulation definition error getting existing definitions", func(t *testing.T) {
		mockedTwinDBLayer.On("DBGetSimulationDefinitions", mock.Anything).Return([]string{}, errors.New("error checking DB for existing definitions"))
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}
		_, err := digitalTwinService.AddSimulationDefinition(simulationDefinition)
		assert.Error(t, err)
	})

	t.Run("Add simulation definition error obtained invalid definition", func(t *testing.T) {
		mockedTwinDBLayer = &redis.MockTwinDB{}
		mockedTwinDBLayer.On("DBGetSimulationDefinition", mock.Anything).Return("invalid definition", nil)
		mockedTwinDBLayer.On("DBGetSimulationDefinitions", mock.Anything).Return([]string{definitionName}, nil)
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}
		_, err := digitalTwinService.AddSimulationDefinition(simulationDefinition)
		assert.Error(t, err)
	})

	t.Run("Add simulation definition return preexisting definition", func(t *testing.T) {
		mockedTwinDBLayer = &redis.MockTwinDB{}
		mockedTwinDBLayer.On("DBGetSimulationDefinition", mock.Anything).Return(string(data), nil)
		mockedTwinDBLayer.On("DBGetSimulationDefinitions", mock.Anything).Return([]string{definitionName}, nil)
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}
		_, err := digitalTwinService.AddSimulationDefinition(simulationDefinition)
		assert.NoError(t, err)
	})

	t.Run("Add simulation definition add new definition", func(t *testing.T) {

		definitionName = "definitionA"
		mockedTwinDBLayer = &redis.MockTwinDB{}
		mockedTwinDBLayer.On("DBGetSimulationDefinition", mock.Anything).Return(string(data), nil)
		mockedTwinDBLayer.On("DBGetSimulationDefinitions", mock.Anything).Return([]string{definitionName}, nil)
		mockedTwinDBLayer.On("DBAddSimulationDefinition", mock.Anything, mock.Anything).Return(nil)
		digitalTwinService := &DigitalTwinService{
			LoggingClient: u.AppService.LoggingClient(),
			DBLayer:       mockedTwinDBLayer,
		}
		_, err := digitalTwinService.AddSimulationDefinition(simulationDefinition)
		assert.NoError(t, err)
	})

}
