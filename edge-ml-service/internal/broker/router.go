package broker

import (
	"net/http"

	"hedge/common/client"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/labstack/echo/v4"
)

var (
	Client         client.HTTPClient
	initialEnvVars map[string]string
)

type Router struct {
	service             interfaces.ApplicationService
	appConfig           *MLBrokerConfig
	notificationChannel chan<- string
}

type RouterResponse struct {
	Status string
}

func NewRouter(svc interfaces.ApplicationService, appConfig *MLBrokerConfig, notificationChannel chan<- string) *Router {
	if Client == nil {
		Client = &http.Client{}
	}
	return &Router{
		service:             svc,
		appConfig:           appConfig,
		notificationChannel: notificationChannel,
	}
}

func (r *Router) Reinitialize(c echo.Context) error {

	lc := r.service.LoggingClient()
	lc.Infof("Reinitializing ML Broker")

	// redeploy models only when the broker pipeline feeds the right data ie when the new model deployment is successful
	// Status of deployment should be reported now via MQTT, to be fixed??
	// Dangerous way to deploy, however since RestartSelf never returns to the next time, can't find a better way for now

	c.Response().WriteHeader(http.StatusAccepted)
	c.Response().Write([]byte("Success"))
	c.Response().Header().Set("Content-Type", "application/json")

	go func() {
		err := RestartSelf()
		if err != nil {
			lc.Errorf("RestartSelf failed with error: %v", err)
		}
	}()

	return nil
}

func (r *Router) ServeNotifications(c echo.Context) error {

	mlModelConfigName := c.Param("mlModelConfigName")
	if mlModelConfigName == "" {
		r.service.LoggingClient().Error("Need to specify ml config name")
		return c.JSON(http.StatusBadRequest, "Need to specify ml config name")
	}

	go func(channel chan<- string, mlModelConfigName string) {
		r.service.LoggingClient().Infof("notifying prediction pipeline for ml model config: %s", mlModelConfigName)
		channel <- mlModelConfigName
	}(r.notificationChannel, mlModelConfigName)

	return c.JSON(http.StatusOK, RouterResponse{Status: "Running prediction for " + mlModelConfigName})
}

func (r *Router) AddRoutes() {
	r.service.AddCustomRoute("/api/v3/ml_broker/prediction/mlconfig/:mlModelConfigName", interfaces.Authenticated, r.ServeNotifications, http.MethodGet)
	r.service.AddCustomRoute("/api/v3/ml_broker/reinitialize", interfaces.Authenticated, r.Reinitialize, http.MethodPost)
}

//func (r *Router) doReinitializeInferenceService(configName string) error {
//
//	service := r.service
//	URL := r.configNameToReinitializingURL[configName]
//	r.service.LoggingClient().Debugf("URL for inference service reinitializing: %s", URL)
//
//	req, err := http.NewRequest(http.MethodPost, URL, nil)
//	if err != nil {
//		service.LoggingClient().Errorf("error creating post payload to re-initialize new models: %v", err)
//		return err
//	}
//
//	req.Header.Set("Content-Type", "application/json")
//	req.Header.Set("Accept", "application/json")
//
//	service.LoggingClient().Debugf("About to reload model by calling: %s", URL)
//	response, err := Client.Do(req)
//	if response != nil && response.Body != nil {
//		defer response.Body.Close()
//	}
//	if err != nil {
//		// In case of dial error we come here ie when the other service is not running
//		return fmt.Errorf("error while re-initializing the inferencing, error: %v", err)
//	}
//
//	if response.StatusCode < 200 || response.StatusCode >= 300 {
//		service.LoggingClient().Errorf("error response to reload model", response.StatusCode)
//		// We still want the pipeline to continue and publish event to TimeSeries db
//		return fmt.Errorf("error while re-initializing the inferencing, http Status from inferencing: %d", response.StatusCode)
//	} else {
//		service.LoggingClient().Debugf("\t response code: %d", response.StatusCode)
//		return nil
//	}
//}
