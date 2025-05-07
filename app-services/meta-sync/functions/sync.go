/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mServ "hedge/app-services/hedge-device-extensions/pkg/service"
	"hedge/app-services/meta-sync/internal/state"
	"hedge/app-services/metadata-notifier/pkg"
	"hedge/common/config"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
	"hedge/common/service"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	commonDTO "github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/requests"
)

type MetaSync struct {
	HAdminUrl          string
	DeviceExtUrl       string
	Service            interfaces.ApplicationService
	MetaService        mServ.MetaDataService
	stateManager       *state.Manager
	HttpSender         service.HTTPSenderInterface
	stateRequestSender *transforms.MQTTSecretSender
	stateDataSender    *transforms.MQTTSecretSender
	node               string
	nodeType           string
}

type Data struct {
	Devices        []*dto.DeviceObject  `json:"devices"`
	Profiles       []*dto.ProfileObject `json:"profiles"`
	DeviceServices []*dto.DeviceService `json:"services"`
}

func NewMetaSync(as interfaces.ApplicationService, metaService mServ.MetaDataService, manager *state.Manager,
	requestSender, dataSender *transforms.MQTTSecretSender, hadminUrl, devexUrl string, persistOnError bool, node, nodeType string) *MetaSync {
	metaSync := MetaSync{}
	metaSync.Service = as
	metaSync.MetaService = metaService
	metaSync.stateManager = manager
	metaSync.HAdminUrl = hadminUrl
	metaSync.DeviceExtUrl = devexUrl
	metaSync.node = node
	metaSync.nodeType = nodeType
	metaSync.stateRequestSender = requestSender
	metaSync.stateDataSender = dataSender
	cookie := service.NewDummyCookie()
	headers := make(map[string]string)
	metaSync.HttpSender = service.NewHTTPSender(hadminUrl, "application/json", persistOnError, headers, &cookie)

	return &metaSync
}

func (s *MetaSync) State(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	lc := ctx.LoggingClient()

	st, ok := data.(state.State)
	if !ok {
		lc.Errorf("failed to convert incoming state data")
		return false, nil
	}

	// check state only for different node type
	if st.NodeType == s.nodeType {
		return false, nil
	}

	err := s.register(ctx, st.Node)
	if err != nil {
		lc.Errorf("failed to register node %s: %s", st.Node.NodeId, err.Error())
		return false, nil
	}

	// check if first sync (known node)
	known := s.stateManager.KnownNode(st.Node)

	// fetch current state
	current, err := s.stateManager.GetState()
	if err != nil {
		lc.Errorf("failed to fetch current state: %s", err.Error())
		return false, nil
	}

	// get incoming state changes
	changes := s.stateManager.Changes(current, &st, !known)
	if changes == nil {
		return false, nil
	}

	// handle delete no need to send over for request
	s.handleDeleteChanges(changes)

	// no changes after delete handled
	if len(changes.Devices) == 0 && len(changes.Profiles) == 0 && len(changes.DeviceServices) == 0 &&
		len(changes.RawDataConfig) == 0 {
		s.Service.LoggingClient().Infof("no changes detected in incoming state from %s", st.Node.NodeId)
		// on first sync in case type is node resend state to validate CORE is synced
		if !known && s.nodeType == "NODE" {
			s.stateManager.StatePublish(current)
		}
		return false, changes
	}

	// set changes needed
	current.Devices = changes.Devices
	current.Profiles = changes.Profiles
	current.DeviceServices = changes.DeviceServices
	current.RawDataConfig = changes.RawDataConfig
	current.Recipient = st.Node.NodeId

	lc.Infof("detected changes in state from %s requesting data...", st.Node.NodeId)

	if s.stateRequestSender != nil {
		s.stateRequestSender.MQTTSend(ctx, current)
	}

	return true, changes
}

func (s *MetaSync) StateRequest(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	lc := ctx.LoggingClient()

	incoming, ok := data.(state.State)
	if !ok {
		lc.Errorf("failed to convert state data")
		return false, nil
	}

	// continue only if recipient match current node
	if incoming.Recipient != s.node {
		return false, nil
	}

	node, err := config.GetNode(s.Service, "current")
	if err != nil {
		return false, nil
	}

	// reset state data and set target recipient and sender
	incoming.Recipient = incoming.Node.NodeId
	incoming.Node = node
	incoming.Data = &state.Data{}

	// set on state requested devices
	for name := range incoming.Devices {
		device, err := s.MetaService.GetCompleteDevice(name, "", s.Service)
		if err != nil {
			s.Service.LoggingClient().Errorf("failed to get device %s", name)
			continue
		}
		incoming.Data.Devices = append(incoming.Data.Devices, &device)
	}

	// set on state requested profiles
	for name := range incoming.Profiles {
		profile, err := s.MetaService.GetCompleteProfile(name)
		if err != nil {
			s.Service.LoggingClient().Errorf("failed to get profile %s", name)
			continue
		}
		incoming.Data.Profiles = append(incoming.Data.Profiles, &profile)
	}

	// set on state requested services
	for name := range incoming.DeviceServices {
		// remove service name node id prefix before lookup if not in core
		if s.nodeType == "NODE" {
			name = strings.TrimPrefix(name, s.node+state.ServiceNameSeparator)
		}
		srv, err := s.Service.DeviceServiceClient().DeviceServiceByName(context.Background(), name)
		if err != nil {
			s.Service.LoggingClient().Errorf("failed to get service %s", name)
			continue
		}
		incoming.Data.DeviceServices = append(incoming.Data.DeviceServices, &srv.Service)
	}

	// set on state requested node configs
	for nodeId := range incoming.RawDataConfig {
		conf, err := s.MetaService.GetNodeRawDataConfigs([]string{nodeId})
		if err != nil {
			s.Service.LoggingClient().Errorf("failed to get raw data config for %s", nodeId)
			continue
		}
		incoming.Data.RawConfig = append(incoming.Data.RawConfig, conf[nodeId])
	}

	if s.stateRequestSender != nil {
		s.stateDataSender.MQTTSend(ctx, incoming)
	}

	return true, incoming
}

func (s *MetaSync) StateData(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	lc := ctx.LoggingClient()

	incoming, ok := data.(state.State)
	if !ok {
		lc.Errorf("Failed to convert state data")
		return false, nil
	}

	// continue only if recipient match current node
	if incoming.Recipient != s.node || incoming.Data == nil {
		return false, nil
	}

	events := make([]pkg.MetaEvent, 0)

	// handle service changes
	for _, ds := range incoming.Data.DeviceServices {
		// map will always include node id prefix in state map
		key := s.node + state.ServiceNameSeparator + ds.Name
		if strings.Contains(s.nodeType, "CORE") {
			// in case in core and not local service set node id to incoming node
			name := incoming.Node.NodeId + state.ServiceNameSeparator + ds.Name
			key = name
			ds.Name = name
		}

		if sum, ok := incoming.DeviceServices[key]; ok {
			switch sum.Action {
			case state.UPDATE:
				update := []requests.UpdateDeviceServiceRequest{{
					BaseRequest: commonDTO.NewBaseRequest(),
					Service: dtos.UpdateDeviceService{
						Id:          &ds.Id,
						Name:        &ds.Name,
						Description: &ds.Description,
						BaseAddress: &ds.BaseAddress,
						Labels:      ds.Labels,
						AdminState:  &ds.AdminState,
					},
				}}
				_, err := s.Service.DeviceServiceClient().Update(context.Background(), update)
				if err != nil {
					lc.Errorf("failed to update service %s error: %s", ds.Name, err.Error())
				}
			case state.CREATE:
				create := []requests.AddDeviceServiceRequest{{BaseRequest: commonDTO.NewBaseRequest(), Service: *ds}}
				_, err := s.Service.DeviceServiceClient().Add(context.Background(), create)
				if err != nil {
					lc.Errorf("failed to create service %s error: %s", ds.Name, err.Error())
				}
			}
		}
	}

	// handle profile changes
	for _, p := range incoming.Data.Profiles {
		if sum, ok := incoming.Profiles[p.Profile.Name]; ok {
			switch sum.Action {
			case state.UPDATE:
				_, err := s.MetaService.UpdateCompleteProfile(p.Profile.Name, *p)
				if err != nil {
					lc.Errorf("failed to update profile %s error: %s", p.Profile.Name, err.Error())
				}
			case state.CREATE:
				_, err := s.MetaService.CreateCompleteProfile(p.Profile.Name, *p)
				if err != nil {
					lc.Errorf("failed to create profile %s error %s", p.Profile.Name, err.Error())
				}
			}
		}
	}

	// handle raw config changes
	for _, conf := range incoming.Data.RawConfig {
		if sum, ok := incoming.RawDataConfig[conf.Node.NodeID]; ok {
			switch sum.Action {
			case state.UPDATE, state.CREATE:
				err := s.MetaService.UpsertNodeRawDataConfigs([]dto.NodeRawDataConfig{*conf})
				if err != nil {
					lc.Errorf("failed to sync raw config for %s error: %s", conf.Node.NodeID, err.Error())
				} else {
					//Only for RawData do we add an event for now, for other scenarios we already get the events via metadata-notifier
					event := pkg.MetaEvent{
						Type: pkg.NodeType,
						Name: conf.Node.NodeID,
					}
					lc.Debugf("added rawDataConfig event")
					events = append(events, event)
				}
			}
		}
	}

	// handle device changes
	for _, d := range incoming.Data.Devices {
		if sum, ok := incoming.Devices[d.Device.Name]; ok {
			switch sum.Action {
			case state.UPDATE:
				_, err := s.MetaService.UpdateCompleteDevice(d.Device.Name, *d)
				if err != nil {
					lc.Errorf("failed to update device %s error %s", d.Device.Name, err.Error())
				}
			case state.CREATE:
				_, err := s.MetaService.CreateCompleteDevice(d.Device.Name, *d)
				if err != nil {
					lc.Errorf("failed to create device %s error %s", d.Device.Name, err.Error())
				}
			}
		}
	}

	// signal update complete
	if s.stateManager != nil { //for test purpose
		s.stateManager.PipelineDone(events)
	}

	return true, nil
}

func (s *MetaSync) handleDeleteChanges(incoming *state.Change) {
	// handle device changes
	for name, info := range incoming.Devices {
		if info.Action == state.DELETE {
			err := s.MetaService.DeleteCompleteDevice(name)
			if err != nil {
				s.Service.LoggingClient().Errorf("failed to create device %s error: %s", name, err.Error())
			}
			delete(incoming.Devices, name)
		}
	}

	// handle profile changes
	for name, info := range incoming.Profiles {
		if info.Action == state.DELETE {
			err := s.MetaService.DeleteCompleteProfile(name)
			if err != nil {
				s.Service.LoggingClient().Errorf("failed to delete profile %s error: %s", name, err.Error())
			}
			delete(incoming.Profiles, name)
		}
	}

	// handle service changes
	for name, info := range incoming.DeviceServices {
		trimmed := name
		// in case in node remove node id prefix from service name before creating
		if s.nodeType == "NODE" {
			trimmed = strings.TrimPrefix(name, s.node+state.ServiceNameSeparator)
		}
		if info.Action == state.DELETE {
			_, err := s.Service.DeviceServiceClient().DeleteByName(context.Background(), trimmed)
			if err != nil {
				s.Service.LoggingClient().Errorf("failed to delete service %s error: %s", name, err.Error())
			}
			delete(incoming.DeviceServices, name)
		}
	}

	// no delete allowed for raw data config
	for name, info := range incoming.RawDataConfig {
		if info.Action == state.DELETE {
			delete(incoming.RawDataConfig, name)
		}
	}
}

func (s *MetaSync) register(ctx interfaces.AppFunctionContext, node models.Node) hedgeErrors.HedgeError {
	if strings.Contains(s.nodeType, "CORE") {
		// get all registered nodes
		nodes, err := s.MetaService.GetAllNodes()
		if err != nil {
			return err
		}

		// check if exist and have changes in properties
		register := true
		for _, n := range nodes {
			if node.NodeId == n.NodeId {
				switch n.HostName == node.HostName {
				case true:
					register = false
				case false:
					err := s.deleteNode(ctx, n.NodeId)
					if err != nil {
						return err
					}
				}
			}
		}

		node.IsRemoteHost = true
		node.RuleEndPoint = ""
		node.WorkFlowEndPoint = ""

		if register {
			err := s.registerNode(ctx, nil, node)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *MetaSync) registerNode(ctx interfaces.AppFunctionContext, syncObjAsBytes []byte, node models.Node) hedgeErrors.HedgeError {
	lc := ctx.LoggingClient()

	errorMessage := fmt.Sprintf("Error registering node %s", node.NodeId)

	jsonByte, err := json.Marshal(node)
	if err != nil {
		lc.Errorf("failed marshal data for registering node id %s error: %s", node.NodeId, err.Error())
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}
	var URL = s.HAdminUrl + "/api/v3/node_mgmt/node?addToDefaultGroup=true"
	s.HttpSender.SetURL(URL)

	exportData := service.NewExportData(jsonByte, syncObjAsBytes)
	ok, responseData := s.HttpSender.HTTPPost(ctx, exportData)
	if !ok {
		errorMessage := fmt.Sprintf("Error registering node %s", node.NodeId)
		service.LogErrorResponseFromService(lc, URL, errorMessage, responseData)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	lc.Infof("new incoming properties for node %s registered", node.NodeId)
	return nil
}

func (s *MetaSync) deleteNode(ctx interfaces.AppFunctionContext, nodeID string) hedgeErrors.HedgeError {
	lc := ctx.LoggingClient()

	var URL = s.HAdminUrl + "/api/v3/node_mgmt/node/" + nodeID
	s.HttpSender.SetURL(URL)

	ok, responseData := s.HttpSender.HTTPDelete(ctx, nil)
	if !ok {
		errorMessage := fmt.Sprintf("Error deleting old node %s", nodeID)
		service.LogErrorResponseFromService(lc, URL, errorMessage, responseData)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, errorMessage)
	}
	lc.Infof("deleted old setup for node id: %s (removed/wipedout)", nodeID)
	return nil
}
