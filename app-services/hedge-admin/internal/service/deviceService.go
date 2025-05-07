/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/app-services/hedge-admin/db"
	"hedge/app-services/hedge-admin/internal/config"
	"hedge/common/client"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/google/uuid"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var (
	consulKVDeviceSvcRootURLFormat = "http://%s/v1/kv/edgex/devices/2.0/"
	consulKVHedgeRoot              = "/Hedge"
	consulKVServiceProtocolRoot    = consulKVHedgeRoot + "/Protocols"

	consulKVCreateDeviceSvcFormat = consulKVDeviceSvcRootURLFormat + "%s" + consulKVServiceProtocolRoot
	//consulKVCreateAppSvcFormat    = "http://%s/v1/kv/edgex/appservices/2.0/%s" + consulKVServiceProtocolRoot

	consulKVQParamRecurse = "?recurse=true"
	consulKVQParamAllKeys = "?keys=true&separator=/"
)

type DeviceService struct {
	service   interfaces.ApplicationService
	dbClient  db.RedisDbClient
	appConfig *config.AppConfig
}

func NewDeviceService(service interfaces.ApplicationService, appConfig *config.AppConfig) *DeviceService {
	deviceService := new(DeviceService)
	deviceService.service = service
	deviceService.dbClient = db.NewDBClient(service)
	deviceService.appConfig = appConfig
	return deviceService
}

func (deviceService *DeviceService) SetHttpClient() {
	if client.Client == nil {
		client.Client = &http.Client{}
	}
}

func (deviceService *DeviceService) RegisterServices(dsProtocols models.DeviceServiceProtocols) hedgeErrors.HedgeError {
	consulToken, hErr := deviceService.fetchConsulToken()
	if hErr != nil {
		return hErr
	}

	errorMessage := "Error registering services"

	for dsName, dsProtocols := range dsProtocols {
		//repeat for every service
		for i, dsProtocol := range dsProtocols {
			//Protocol Name
			kvURL := fmt.Sprintf(consulKVCreateDeviceSvcFormat, deviceService.appConfig.ConsulURL, dsName) + "/" + strconv.Itoa(i) + "/" + models.ConsulKVServiceProtocolName
			kvBody := dsProtocol.ProtocolName
			if kvBody == "" {
				errStr := "Empty Protocol Name"
				deviceService.service.LoggingClient().Errorf(errStr)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("%s: %s", errorMessage, errStr))
			}
			deviceService.service.LoggingClient().Debugf("KV: URL=%s, Payload=%s", kvURL, kvBody)

			request, _ := http.NewRequest(http.MethodPut, kvURL, strings.NewReader(kvBody))
			request.Header.Set("Content-type", "application/json")
			request.Header.Set("X-Consul-Token", consulToken)

			deviceService.SetHttpClient()
			resp, err := client.Client.Do(request)
			if err != nil {
				deviceService.service.LoggingClient().Error(err.Error())
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
			}

			if resp.StatusCode != 200 {
				r, _ := io.ReadAll(resp.Body)
				deviceService.service.LoggingClient().Errorf("Failed HTTP request [%d] %s: URL=%s, Payload=%s; error=%s", resp.StatusCode, resp.Body, kvURL, kvBody, r)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
			} else {
				deviceService.service.LoggingClient().Debugf("Response for HTTP call to create protocol name in consul: %v", resp)
			}

			err = resp.Body.Close()
			if err != nil {
				deviceService.service.LoggingClient().Errorf("Error closing resonse body: %v", err)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
			}

			//Protocol Properties
			kvURL = fmt.Sprintf(consulKVCreateDeviceSvcFormat, deviceService.appConfig.ConsulURL, dsName) + "/" + strconv.Itoa(i) + "/" + models.ConsulKVServiceProtocolProps
			if len(dsProtocol.ProtocolProperties) == 0 {
				errStr := "Empty Protocol Properties"
				deviceService.service.LoggingClient().Errorf(errStr)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("%s: %s", errorMessage, errStr))
			}
			kvBody = strings.Join(dsProtocol.ProtocolProperties, ",")
			deviceService.service.LoggingClient().Debugf("KV: URL=%s, Payload=%s", kvURL, kvBody)

			request, _ = http.NewRequest(http.MethodPut, kvURL, strings.NewReader(kvBody))
			request.Header.Set("Content-type", "application/json")
			request.Header.Set("X-Consul-Token", consulToken)

			deviceService.SetHttpClient()
			resp, err = client.Client.Do(request)
			if err != nil {
				deviceService.service.LoggingClient().Error(err.Error())
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
			}

			if resp.StatusCode != 200 {
				r, _ := io.ReadAll(resp.Body)
				deviceService.service.LoggingClient().Errorf("Failed HTTP request [%d] %s: URL=%s, Payload=%s; error=%s", resp.StatusCode, resp.Body, kvURL, kvBody, r)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
			} else {
				deviceService.service.LoggingClient().Debugf("Response for HTTP call to create protocol properties in consul: %v", resp)
			}

			err = resp.Body.Close()
			if err != nil {
				deviceService.service.LoggingClient().Errorf("Error closing resonse body: %v", err)
				return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
			}
		}
	}

	return nil
}

func (deviceService *DeviceService) GetServiceProtocolsFromConsul(devServiceName string) (models.DeviceProtocols, hedgeErrors.HedgeError) {
	ret := models.DeviceProtocols{}
	consulToken, hErr := deviceService.fetchConsulToken()
	if hErr != nil {
		return ret, hErr
	}

	errorMessage := fmt.Sprintf("Error getting service protocol from Consul for devServiceName: %s", devServiceName)

	kvURL := fmt.Sprintf(consulKVCreateDeviceSvcFormat, deviceService.appConfig.ConsulURL, devServiceName) + consulKVQParamRecurse
	deviceService.service.LoggingClient().Debugf("KV: URL=%s", kvURL)

	request, _ := http.NewRequest(http.MethodGet, kvURL, nil)
	request.Header.Set("Content-type", "application/json")
	request.Header.Set("X-Consul-Token", consulToken)

	deviceService.SetHttpClient()
	resp, err := client.Client.Do(request)
	if err != nil {
		deviceService.service.LoggingClient().Error(err.Error())
		return ret, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		deviceService.service.LoggingClient().Infof("Empty response for HTTP call to fetch service protocols in consul")
	} else if resp.StatusCode != 200 {
		r, _ := io.ReadAll(resp.Body)
		deviceService.service.LoggingClient().Errorf("Failed HTTP request [%d] %s: URL=%s; error=%s", resp.StatusCode, resp.Body, kvURL, r)
		return ret, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	} else {
		deviceService.service.LoggingClient().Debugf("Response for HTTP call to fetch service protocols in consul: %v", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		deviceService.service.LoggingClient().Error(err.Error())
		return ret, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	if len(body) == 0 {
		body = []byte(`[]`)
	}

	var consulKV []models.ConsulKV
	err = json.Unmarshal(body, &consulKV)
	if err != nil {
		deviceService.service.LoggingClient().Error(err.Error())
		return ret, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	ret, err = models.ConsulKVtoDSProtocols(consulKV)
	if err != nil {
		deviceService.service.LoggingClient().Error(err.Error())
		return ret, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return ret, nil
}

func (deviceService *DeviceService) InitializeDeviceServices(inServiceName string) (models.DeviceServiceProtocols, hedgeErrors.HedgeError) {
	dsps := models.DeviceServiceProtocols{}
	var consulSvcs []string

	errorMessage := fmt.Sprintf("Error initializing device services %s", inServiceName)

	if len(inServiceName) == 0 || inServiceName == "all" {
		// Get all device services
		var err hedgeErrors.HedgeError
		consulSvcs, err = deviceService.getAllServices()
		if err != nil {
			deviceService.service.LoggingClient().Error(err.Error())
			return nil, err
		}
	} else {
		consulSvcs = append(consulSvcs, inServiceName)
	}

	// For every device service, fetch protocol info from consul and create entry in redis
	for _, conSvc := range consulSvcs {
		var consulDevSvcName string
		ss := strings.Split(conSvc, "/")
		if len(ss) == 1 {
			consulDevSvcName = ss[0]
		} else if len(ss) > 2 {
			consulDevSvcName = ss[len(ss)-2]
		} else {
			errStr := "Invalid service name: " + conSvc
			deviceService.service.LoggingClient().Error(errStr)
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError,
				fmt.Sprintf("%s: %s", errorMessage, errStr))
		}

		devProtocols, err := deviceService.GetServiceProtocolsFromConsul(consulDevSvcName)
		if err != nil {
			deviceService.service.LoggingClient().Error(err.Error())
			return nil, err
		}

		if len(devProtocols) == 0 || devProtocols == nil {
			// service protocol details not found in consul
			deviceService.service.LoggingClient().Errorf("Protocol details not found for %s in consul. Skipping to next service..", consulDevSvcName)
			continue //move to the next service
		}

		dspstmp := models.DeviceServiceProtocols{}
		dspstmp[consulDevSvcName] = devProtocols
		// Create redis entries for the protocol info
		dspsKeys, _err := deviceService.dbClient.SaveProtocols(dspstmp)
		if _err != nil {
			deviceService.service.LoggingClient().Errorf("Failed creating redis entries for %s service protocols. Error: %s", consulDevSvcName, _err.Error())
			continue //move to the next service
		}

		dsps[consulDevSvcName] = devProtocols
		deviceService.service.LoggingClient().Debugf("Successfully created redis entries at %s", dspsKeys)
	}

	return dsps, nil
}

func (deviceService *DeviceService) getAllServices() ([]string, hedgeErrors.HedgeError) {
	consulToken, hErr := deviceService.fetchConsulToken()
	if hErr != nil {
		return nil, hErr
	}

	errorMessage := "Error getting all services"

	kvURL := fmt.Sprintf(consulKVDeviceSvcRootURLFormat, deviceService.appConfig.ConsulURL) + consulKVQParamAllKeys
	deviceService.service.LoggingClient().Debugf("KV: URL=%s", kvURL)

	request, _ := http.NewRequest(http.MethodGet, kvURL, nil)
	request.Header.Set("Content-type", "application/json")
	request.Header.Set("X-Consul-Token", consulToken)

	deviceService.SetHttpClient()
	resp, err := client.Client.Do(request)
	if err != nil {
		deviceService.service.LoggingClient().Error(err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		deviceService.service.LoggingClient().Infof("Empty response for HTTP call to fetch All device services in consul")
	} else if resp.StatusCode != 200 {
		r, _ := io.ReadAll(resp.Body)
		deviceService.service.LoggingClient().Errorf("Failed HTTP request [%d] %s: URL=%s; error=%s", resp.StatusCode, resp.Body, kvURL, r)
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	} else {
		deviceService.service.LoggingClient().Debugf("Response for HTTP call to fetch All device services in consul: %v", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		deviceService.service.LoggingClient().Error(err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	if len(body) == 0 {
		body = []byte(`[]`)
	}

	var services []string
	err = json.Unmarshal(body, &services)
	if err != nil {
		deviceService.service.LoggingClient().Error(err.Error())
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}

	return services, nil
}

func (deviceService *DeviceService) fetchConsulToken() (string, hedgeErrors.HedgeError) {
	consulToken := ""
	consulTokenFile, found := os.LookupEnv("HEDGE_CONSUL_TOKEN_FILE")
	if !found {
		deviceService.service.LoggingClient().Errorf("Environment variable HEDGE_CONSUL_TOKEN_FILE not found. Error out..")
		return consulToken, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeConfig, "Invalid consul token found")
	}
	token, err := os.ReadFile(consulTokenFile)
	if err != nil {
		deviceService.service.LoggingClient().Errorf("Failed reading the token file. ERROR: %v", err)
		return consulToken, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, "Failed reading the consul token")
	}
	consulToken = strings.TrimSpace(string(token))
	// security check - validate if file contains a valid uuid token. Prevents from path manipulation attack.
	_, err = uuid.Parse(consulToken)
	if err != nil {
		deviceService.service.LoggingClient().Errorf("invalid token found at %s. ERROR: %v", consulTokenFile, err)
		deviceService.service.LoggingClient().Debugf("Token found = %s", consulToken)
		return "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, "Invalid consul token found")
	}
	return consulToken, nil
}
