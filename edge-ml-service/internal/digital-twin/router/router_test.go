package router

import (
	digital_twin "hedge/edge-ml-service/pkg/digital-twin"
	digital_twin2 "hedge/mocks/hedge/edge-ml-service/pkg/digital-twin"

	//"hedge/edge-ml-service/internal/digital_twin"
	"hedge/edge-ml-service/pkg/dto/twin"
	//mocks "hedge/mocks/bmc.com/edge-i"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"encoding/json"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hedge/edge-ml-service/pkg/dto/job"
	//mocks "hedge/mocks/hedge/app-services/digital-twin/service"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

var (
	u                  *utils.HedgeMockUtils
	mockedOrchestrator *digital_twin2.MockOrchestratorInterface
)

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{})
	mockedOrchestrator = &digital_twin2.MockOrchestratorInterface{}
}

func getSimulationDefinition(simulationDefinitionName string) (twin.SimulationDefinition, error) {
	return twin.SimulationDefinition{}, nil
}

func TestRouteNotify_BindError(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader("invalid json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	router := &Router{}
	router.edgexSvc = u.AppService

	// Test
	err := router.RouteNotify(c)

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, c.Response().Status, http.StatusInternalServerError)
}

func TestRouteNotify_UnmarshalError(t *testing.T) {
	// Setup
	e := echo.New()

	// Use invalid JSON content to simulate unmarshal error
	invalidJSON := `{"Name": "invalidContent",`
	req := httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(invalidJSON))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	router := &Router{
		edgexSvc: u.AppService,
	}

	// Test
	err := router.RouteNotify(c)

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, http.StatusInternalServerError, c.Response().Status)
	//assert.Contains(t, rec.Body.String(), "unexpected")
}

func TestRouteNotify_JobNotCompleted(t *testing.T) {
	// Setup

	e := echo.New()

	jobDetails := job.TrainingJobDetails{Status: job.JobStatusMap[job.Failed]}
	content, _ := json.Marshal(jobDetails)
	notification := dtos.Notification{Content: string(content)}
	jsonBody, _ := json.Marshal(notification)

	req := httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(string(jsonBody)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	router := &Router{edgexSvc: u.AppService, Orchestrator: mockedOrchestrator}

	// Test
	err := router.RouteNotify(c)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, http.StatusExpectationFailed, rec.Code)
}

func TestRouteNotify_JobCompleted(t *testing.T) {
	// Setup

	e := echo.New()

	jobDetails := job.TrainingJobDetails{Status: job.JobStatusMap[job.TrainingCompleted]}
	content, _ := json.Marshal(jobDetails)
	notification := dtos.Notification{Content: string(content)}
	jsonBody, _ := json.Marshal(notification)

	req := httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(string(jsonBody)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	mockOrchestrator, err := digital_twin.NewOrchestrator(
		"http://www.example.com/training",
		"http://www.example.com/prediction",
		nil,
		u.AppService.LoggingClient(),
	)
	assert.NoError(t, err)
	mockOrchestrator.GetSimulationDefinition = getSimulationDefinition
	router := &Router{edgexSvc: u.AppService, Orchestrator: mockOrchestrator}

	// Test
	err = router.RouteNotify(c)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRoute_RunTraining(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/training/trainingJob/simulation/test", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(req, rec)
	c.SetPath("/training/:name/simulation/:simulationName")
	c.SetParamNames("name", "simulationName")
	c.SetParamValues("training", "test")
	t.Log(c.Request().RequestURI)

	router := &Router{edgexSvc: u.AppService, Orchestrator: mockedOrchestrator}
	mockedOrchestrator.On("SubmitTrainingJob", mock.Anything, mock.Anything).Return("", nil)

	err := router.RunTraining(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, c.Response().Status)

}
