package router

import (
	digital_twin "hedge/edge-ml-service/pkg/digital-twin"
	"net/http"

	"hedge/common/db/redis"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
)

type Router struct {
	edgexSvc             interfaces.ApplicationService
	dbClient             *redis.DBClient
	digitalTwinService   *digital_twin.DigitalTwinService
	ManagementServiceURL string
	BrokerURL            string
	httpClient           *http.Client
	Orchestrator         digital_twin.OrchestratorInterface
}

var StatusMap = map[digital_twin.Status]int{
	digital_twin.BadRequest:    http.StatusBadRequest,
	digital_twin.InternalError: http.StatusInternalServerError,
	digital_twin.OK:            http.StatusOK,
	digital_twin.NotFound:      http.StatusNotFound,
	digital_twin.AlreadyExist:  http.StatusBadRequest,
}

func NewRouter(service interfaces.ApplicationService, dbClient *redis.DBClient) *Router {

	logger := service.LoggingClient()
	mgmtServiceURL, _ := service.GetAppSetting("MLManagementURL")
	brokerURL, _ := service.GetAppSetting("MLBrokerURL")

	r := Router{
		edgexSvc:             service,
		dbClient:             dbClient,
		ManagementServiceURL: mgmtServiceURL,
		digitalTwinService:   digital_twin.NewDigitalTwinService(logger, dbClient),
		httpClient:           new(http.Client),
		BrokerURL:            brokerURL,
	}

	orchestrator, err := digital_twin.NewOrchestrator(
		mgmtServiceURL+"/training_job",
		brokerURL+"/prediction/mlconfig",
		r.getSimulationDefinition,
		service.LoggingClient(),
	)
	if err != nil {
		r.edgexSvc.LoggingClient().Error(err.Error())
	}

	r.Orchestrator = orchestrator

	//r.edgexSvc.LoggingClient().SetLogLevel("INFO")
	return &r
}

// paths for DigitalTwin & Simulation APIs model definition
const (
	TwinBasePath             = "/api/v3/twin"
	TwinDefinitionPath       = TwinBasePath + "/definition"
	TwinDefinitionPathAll    = TwinDefinitionPath + "/all"
	TwinDefinitionByNamePath = TwinBasePath + "/definition/name/:name"
	TwinDevicesByNamePath    = TwinBasePath + "/definition/name/:name/devices"

	TwinSimulationDefinitionPath       = TwinBasePath + "/simulation"
	TwinSimulationDefinitionByNamePath = TwinBasePath + "/simulation/name/:name"
	TwinSimulationDefinitionsPath      = TwinBasePath + "/simulation/all"

	TwinNotificationPath = TwinBasePath + "/notify"
	TwinTrainingPath     = TwinBasePath + "/training/:name/simulation/:simulationName"
)

// TODO: More APIs need to be added to support search and get name, also get all devices given a twin name

// LoadRestRoutes: Load default routes
func (r *Router) LoadRestRoutes() {
	r.edgexSvc.LoggingClient().Info("adding digital twin routes")
	r.addTwinDefinition()
	r.updateTwinDefinition()
	r.getTwinDefinition()
	r.deleteTwinDefinition()
	r.getAllTwinDefinitions()

	//r.edgexSvc.AddCustomRoute(TwinDefinitionPath, interfaces.Authenticated, r.ListTwinDefinitions, http.MethodGet)
	r.edgexSvc.AddCustomRoute(TwinDevicesByNamePath, interfaces.Authenticated, r.ListTwinDevices, http.MethodGet)

	r.edgexSvc.AddCustomRoute(TwinSimulationDefinitionPath, interfaces.Authenticated, r.AddSimulationDefinition, http.MethodPost)
	r.edgexSvc.AddCustomRoute(TwinSimulationDefinitionByNamePath, interfaces.Authenticated, r.GetSimulationDefinition, http.MethodGet)
	r.edgexSvc.AddCustomRoute(TwinSimulationDefinitionByNamePath, interfaces.Authenticated, r.DeleteSimulationDefinition, http.MethodDelete)
	r.edgexSvc.AddCustomRoute(TwinSimulationDefinitionsPath, interfaces.Authenticated, r.ListSimulationDefinitions, http.MethodGet)

	r.edgexSvc.AddCustomRoute(TwinNotificationPath, false, r.RouteNotify, http.MethodPost)

	//r.edgexSvc.LoggingClient().Infof("Adding path for training: %s %+v %+v", TwinTrainingPath, r.RunTraining, http.MethodPost)
	r.edgexSvc.AddCustomRoute(TwinTrainingPath, interfaces.Authenticated, r.RunTraining, http.MethodPost)

}

// @Summary      Create a new Digital Definition template. This template is a wrapper around a Simulation ML model
// @Description  The template definition that is created is used to create ML model that gets used for training
// @Tags         DigitalTwin - Simulations
// @Param        Body            body     twin.DigitalTwinDefinition true "Digital Twin Definition details."
// @Success      201             "DigitalTwin successfully created."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/twin/definition [post]
func (r *Router) addTwinDefinition() {
	_ = r.edgexSvc.AddCustomRoute(TwinDefinitionPath, interfaces.Authenticated, r.AddTwinDefinition, http.MethodPost)
}

// @Summary      Updates the existing Digital Definition template. This template is a wrapper around a Simulation ML model
// @Description  The template definition that is created is used to create ML model that gets used for training
// @Tags         DigitalTwin - Simulations
// @Param        Body            body     twin.DigitalTwinDefinition true "Digital Twin Definition details."
// @Success      201             "DigitalTwin successfully created."
// @Failure			400			{object}	error	"{"message":"Error message"}"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/twin/definition [put]
func (r *Router) updateTwinDefinition() {
	_ = r.edgexSvc.AddCustomRoute(TwinDefinitionPath, interfaces.Authenticated, r.UpdateTwinDefinition, http.MethodPut)
}

// @Summary      Gets the Digital Definition template. This template is a wrapper around a Simulation ML model
// @Description  The template definition that is fetched is used to create ML model that gets used for training
// @Tags         DigitalTwin - Simulations
// @Param        digitalTwinDefinitionName  path     string  true   "digitalTwinDefinition Name"  // Name of the digitalTwinDefinition to retrieve
// @Success      200            {object} twin.DigitalTwinDefinition "Digital Twin Definition details"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/twin/definition/name/{digitalTwinDefinitionName} [get]
func (r *Router) getTwinDefinition() {
	r.edgexSvc.AddCustomRoute(TwinDefinitionByNamePath, interfaces.Authenticated, r.GetTwinDefinition, http.MethodGet)
}

// @Summary      Delete the Digital Definition template. This template is a wrapper around a Simulation ML model
// @Description  The template definition that is fetched is used to create ML model that gets used for training
// @Tags         DigitalTwin - Simulations
// @Param        digitalTwinDefinitionName  path     string  true   "digitalTwinDefinition Name"  // Name of the digitalTwinDefinition to retrieve
// @Success      200            {object} twin.DigitalTwinDefinition "Digital Twin Definition deleted successfully"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/twin/definition/name/{digitalTwinDefinitionName} [delete]
func (r *Router) deleteTwinDefinition() {
	r.edgexSvc.AddCustomRoute(TwinDefinitionByNamePath, interfaces.Authenticated, r.DeleteTwinDefinition, http.MethodDelete)
}

// @Summary      Get all Digital Twin Definitions.
// @Description  Should get the summary view which is yet to be implemented
// @Tags         DigitalTwin - Simulations
// @Success      200            {object} []twin.DigitalTwinDefinition "Digital Twin Definition deleted successfully"
// @Failure			404			{object}	error	"{"message":"Error message"}"
// @Failure			500			{object}	error	"{"message":"Error message"}"
// @Router       /api/v3/twin/definition/all [get]
func (r *Router) getAllTwinDefinitions() {
	r.edgexSvc.AddCustomRoute(TwinDefinitionPathAll, interfaces.Authenticated, r.ListTwinDefinitions, http.MethodGet)
}
