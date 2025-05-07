package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hedge/app-services/meta-sync/internal/db"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	commonDtos "github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/requests"
	model "github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	mServ "hedge/app-services/hedge-device-extensions/pkg/service"
	"hedge/app-services/metadata-notifier/pkg"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"hedge/common/models"
)

const (
	ServiceNameSeparator = "__"

	UPDATE = "UPDATE"
	CREATE = "CREATE"
	DELETE = "DELETE"
)

type State struct {
	CoreStateVersion string           `json:"core_state_version"`
	Node             models.Node      `json:"node"`
	NodeType         string           `json:"node_type"`
	Recipient        string           `json:"recipient,omitempty"`
	Devices          map[string]*Info `json:"devices_info"`
	Profiles         map[string]*Info `json:"profiles_info"`
	DeviceServices   map[string]*Info `json:"services_info"`
	RawDataConfig    map[string]*Info `json:"raw_data_config_info"`
	Data             *Data            `json:"data,omitempty"`
	Timestamp        int64            `json:"timestamp"`
}

type Info struct {
	Sum    string `json:"sum"`
	NodeID string `json:"node_id,omitempty"`
	Action string `json:"action,omitempty"`
}

type Data struct {
	Devices        []*dto.DeviceObject      `json:"devices,omitempty"`
	Profiles       []*dto.ProfileObject     `json:"profiles,omitempty"`
	DeviceServices []*dtos.DeviceService    `json:"services,omitempty"`
	RawConfig      []*dto.NodeRawDataConfig `json:"raw_config,omitempty"`
}

type Change struct {
	Devices        map[string]*Info
	Profiles       map[string]*Info
	DeviceServices map[string]*Info
	RawDataConfig  map[string]*Info
}

type CompareContext struct {
	firstSync bool
	FromNode  string
}

type Manager struct {
	service       interfaces.ApplicationService
	metaService   mServ.MetaDataService
	dbClient      redis.DBClient
	node          models.Node
	nodeType      string
	stateInterval int
	eventInterval int
	eventCh       chan *pkg.MetaEvent
	notifyCh      chan *State
}

func NewManager(service interfaces.ApplicationService, meta mServ.MetaDataService, client redis.DBClient, node models.Node, nodeType string, stateInterval, eventInterval int, subscribeNotifications bool) (*Manager, error) {
	m := &Manager{
		service:       service,
		metaService:   meta,
		dbClient:      client,
		node:          node,
		nodeType:      nodeType,
		stateInterval: stateInterval,
		eventInterval: eventInterval,
		eventCh:       make(chan *pkg.MetaEvent),
		notifyCh:      make(chan *State),
	}

	if subscribeNotifications {
		if err := m.subscribeNotifications(); err != nil {
			return nil, err
		}
	}
	if err := m.registerSyncRoute(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Changes(current, incoming *State, firstSync bool) *Change {
	switch strings.Contains(current.NodeType, "CORE") {
	case true:
		currentCoreStateVersion := m.dbClient.GetStateCoreVersion()
		// allow node to update only if on latest core version
		if incoming.CoreStateVersion != currentCoreStateVersion {
			// if node is not synchronized with core, trigger state notify for node
			m.notifyCh <- current
			return nil
		}
	default:
		// in case we are NODE type set incoming core version
		err := m.dbClient.SetStateCoreVersion(incoming.CoreStateVersion)
		if err != nil {
			m.service.LoggingClient().Errorf("error setting core state version: %s", err.Error())
			return nil
		}
		current.CoreStateVersion = incoming.CoreStateVersion
	}

	ctx := CompareContext{firstSync: firstSync, FromNode: incoming.Node.NodeId}

	// in case node is not CORE node, filter devices/configs by node
	incomingDevices := m.infoByNode(incoming.Devices)
	incomingConfigs := m.infoByNode(incoming.RawDataConfig)
	incomingServices := m.infoByNode(incoming.DeviceServices)

	deviceChanged := m.compareStateResource(current.Devices, incomingDevices, ctx)
	profileChanged := m.compareStateResource(current.Profiles, incoming.Profiles, ctx)
	deviceServiceChanged := m.compareStateResource(current.DeviceServices, incomingServices, ctx)
	configChanged := m.compareStateResource(current.RawDataConfig, incomingConfigs, ctx)

	var change Change
	change.Devices = deviceChanged
	change.Profiles = profileChanged
	change.DeviceServices = deviceServiceChanged
	change.RawDataConfig = configChanged

	return &change
}

func (m *Manager) Listen() chan *State {
	go func() {
		var mu sync.Mutex
		// events counter
		var events []*pkg.MetaEvent

		computeState := func() {
			// lock to ensure exclusive access
			mu.Lock()
			defer mu.Unlock()

			// no events to process
			if len(events) == 0 {
				return
			}

			s, err := m.calculateState()
			if err != nil {
				m.service.LoggingClient().Errorf("failed to calculate state: %v", err)
				events = make([]*pkg.MetaEvent, 0)
				return
			}
			m.service.LoggingClient().Infof("recaculated state for %d events", len(events))

			if s != nil {
				m.notifyCh <- s
				// somehow the events list always contain empty events, so ok to override with new event
				_ = m.notifyExternal(events)
			}
			events = make([]*pkg.MetaEvent, 0)
		}

		// fixed interval trigger
		fixedInterval := time.NewTicker(time.Duration(m.stateInterval) * 60 * time.Second)
		defer fixedInterval.Stop()

		// events processing interval trigger
		eventsInterval := time.NewTicker(time.Duration(m.eventInterval) * time.Second)
		defer eventsInterval.Stop()

		for {
			select {
			// listen for fixed interval trigger
			case <-fixedInterval.C:
				mu.Lock()
				events = append(events, nil)
				mu.Unlock()
				computeState()
			// listen for event interval trigger
			case <-eventsInterval.C:
				computeState()
			// listen events and count for process
			case e := <-m.eventCh:
				mu.Lock()
				events = append(events, e)
				eventsInterval = time.NewTicker(time.Duration(m.eventInterval) * time.Second)
				mu.Unlock()
			}
		}
	}()

	// init and send state on listen startup
	m.eventCh <- &pkg.MetaEvent{}

	return m.notifyCh
}

func (m *Manager) GetState() (*State, hedgeErrors.HedgeError) {
	errorMessage := "Error getting state"

	s, err := m.dbClient.GetState()
	if err != nil {
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	// Unmarshal JSON bytes into a State struct
	var st State
	if err := json.Unmarshal(s, &st); err != nil {
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, errorMessage)
	}

	return &st, nil
}

func (m *Manager) GetStateCoreVersion() string {
	return m.dbClient.GetStateCoreVersion()
}

func (m *Manager) SetStateCoreVersion(version string) hedgeErrors.HedgeError {
	return m.dbClient.SetStateCoreVersion(version)
}

func (m *Manager) KnownNode(node models.Node) bool {
	known := m.dbClient.KnownNode(node.NodeId)
	if len(known) == 0 {
		_ = m.dbClient.SetKnownNode(node.NodeId, node.HostName)
		return false
	}
	return true
}

func (m *Manager) PipelineDone(events []pkg.MetaEvent) {
	for _, event := range events {
		m.eventCh <- &event
	}
	//m.eventCh <- &pkg.MetaEvent{}
}

func (m *Manager) StatePublish(state *State) {
	m.notifyCh <- state
}

func (m *Manager) calculateState() (*State, hedgeErrors.HedgeError) {
	devices, err := m.devicesInfo()
	if err != nil {
		return nil, err
	}

	profiles, err := m.profilesInfo()
	if err != nil {
		return nil, err
	}

	services, err := m.servicesInfo()
	if err != nil {
		return nil, err
	}

	devicesStateInfo := make(map[string]*Info)
	for _, device := range devices {
		cd, err := m.metaService.GetCompleteDevice(device.Name, "", m.service)
		if err != nil {
			return nil, err
		}

		// set to zero values because we dont want to compare IDs/Dates between nodes
		cd.Device.Id = ""
		cd.Device.Modified = 0
		cd.Device.Created = 0

		// will be different in core or node we don't want to compare that
		cd.Node.IsRemoteHost = false
		cd.Node.RuleEndPoint = ""
		cd.Node.WorkFlowEndPoint = ""

		encoded, _ := json.Marshal(cd)
		devicesStateInfo[cd.Device.Name] = m.stateInfo(cd.Node.NodeId, encoded)

	}

	profilesStateInfo := make(map[string]*Info)
	for _, profile := range profiles {
		cp, err := m.metaService.GetCompleteProfile(profile.Name)
		if err != nil {
			return nil, err
		}

		// set to zero values because we dont want to compare IDs/Dates between nodes
		cp.Profile.Id = ""
		cp.Profile.Modified = 0
		cp.Profile.Created = 0

		encoded, _ := json.Marshal(cp)
		profilesStateInfo[cp.Profile.Name] = m.stateInfo("" /* node not needed */, encoded)
	}

	serviceStateInfo := make(map[string]*Info)
	for _, service := range services {
		// in case service is local one add node id prefix
		if !strings.Contains(service.Name, ServiceNameSeparator) {
			service.Name = m.node.NodeId + ServiceNameSeparator + service.Name
		}

		nodeID := strings.Split(service.Name, ServiceNameSeparator)[0]

		// set to zero values because we dont want to compare IDs/Dates between nodes
		service.Id = ""
		service.Modified = 0
		service.Created = 0

		encoded, _ := json.Marshal(service)
		serviceStateInfo[service.Name] = m.stateInfo(nodeID, encoded)
	}

	// fetch raw data nodes config
	configs, err := m.metaService.GetNodeRawDataConfigs([]string{"all"})
	if err != nil {
		return nil, err
	}

	configsStateInfo := make(map[string]*Info)
	for nodeId, config := range configs {
		// if send raw data is false then we don't want to compare timestamps between servers
		// when set to false start time will be the creation time/last update time when was true and end would be current time
		if !config.SendRawData {
			config.StartTime = 0
			config.EndTime = 0
		}

		encoded, _ := json.Marshal(&config)
		configsStateInfo[config.Node.NodeID] = m.stateInfo(nodeId, encoded)
	}

	s := State{
		CoreStateVersion: m.dbClient.GetStateCoreVersion(),
		Node:             m.node,
		NodeType:         m.nodeType,
		Devices:          devicesStateInfo,
		Profiles:         profilesStateInfo,
		DeviceServices:   serviceStateInfo,
		RawDataConfig:    configsStateInfo,
		Timestamp:        time.Now().Unix(),
	}

	// set core current state version if type is "CORE"
	if strings.Contains(m.nodeType, "CORE") {
		version := uuid.New().String()
		// set calculated version for state
		err = m.dbClient.SetStateCoreVersion(version)
		if err != nil {
			return nil, err
		}
		s.CoreStateVersion = version
	}

	// set state
	st, e := json.Marshal(s)
	if e != nil {
		return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Failed to calculate state")
	}
	err = m.dbClient.SetState(st)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (m *Manager) compareStateResource(current, incoming map[string]*Info, ctx CompareContext) map[string]*Info {
	changed := make(map[string]*Info)
	if incoming == nil {
		return changed
	}

	// set delete if sum is missing and not first sync
	for key, currentInfo := range current {
		if ctx.firstSync {
			break
		}
		if _, ok := incoming[key]; !ok {
			isCore := strings.Contains(m.nodeType, "CORE")
			// check resource as node owner
			asNodeOwner := currentInfo.NodeID != ""
			// check current incoming node owns the resource
			correctNodeOwner := currentInfo.NodeID == ctx.FromNode

			// before delete in node with core type, check resource belong to node trying to delete
			if isCore && asNodeOwner && !correctNodeOwner {
				continue
			}
			currentInfo.Action = DELETE
			changed[key] = currentInfo
		}
	}

	for name, incomingInfo := range incoming {
		switch currentInfo, ok := current[name]; ok {
		case true:
			// set update if sums not match
			if incomingInfo.Sum != currentInfo.Sum {
				currentInfo.Action = UPDATE
				changed[name] = currentInfo
			}
		default:
			// set create if sum not found
			incomingInfo.Action = CREATE
			changed[name] = incomingInfo
		}
	}
	return changed
}

func (m *Manager) devicesInfo() ([]dtos.Device, hedgeErrors.HedgeError) {
	var offset = 0
	var limit = 1024

	var devices []dtos.Device
	for {
		d, err := m.service.DeviceClient().AllDevices(context.Background(), []string{}, offset, limit)
		if err != nil {
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error getting devices info")
		}

		devices = append(devices, d.Devices...)

		if len(d.Devices) < limit {
			break
		}

		offset += limit
	}

	return devices, nil
}

func (m *Manager) profilesInfo() ([]dtos.DeviceProfile, hedgeErrors.HedgeError) {
	var offset = 0
	var limit = 1024

	var profiles []dtos.DeviceProfile
	for {
		p, err := m.service.DeviceProfileClient().AllDeviceProfiles(context.Background(), []string{}, offset, limit)
		if err != nil {
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error getting profiles info")
		}

		profiles = append(profiles, p.Profiles...)

		if len(p.Profiles) < limit {
			break
		}

		offset += limit
	}

	return profiles, nil
}

func (m *Manager) servicesInfo() ([]dtos.DeviceService, hedgeErrors.HedgeError) {
	var offset = 0
	var limit = 1024

	var services []dtos.DeviceService
	for {
		s, err := m.service.DeviceServiceClient().AllDeviceServices(context.Background(), []string{}, offset, limit)
		if err != nil {
			return nil, hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Error getting services info")
		}

		services = append(services, s.Services...)

		if len(s.Services) < limit {
			break
		}

		offset += limit
	}

	return services, nil
}

func (m *Manager) stateInfo(nodeId string, data []byte) *Info {
	hash := sha256.Sum256(data)

	return &Info{
		NodeID: nodeId,
		Sum:    hex.EncodeToString(hash[:]),
	}
}

func (m *Manager) infoByNode(incoming map[string]*Info) map[string]*Info {
	if strings.Contains(m.nodeType, "CORE") {
		return incoming
	}

	filtered := make(map[string]*Info)
	for name, info := range incoming {
		if info.NodeID == m.node.NodeId {
			filtered[name] = info
		}
	}

	return filtered
}

func (m *Manager) registerSyncRoute() error {
	// set syncing route for manual trigger or notifications
	return m.service.AddCustomRoute("/api/v3/state/refresh", interfaces.Unauthenticated, func(c echo.Context) error {
		var event pkg.MetaEvent
		err := json.NewDecoder(c.Request().Body).Decode(&event)
		m.service.LoggingClient().Debugf("metaEvent: %v", event)
		if err != nil {
			m.service.LoggingClient().Errorf("failed to parse meta event error: %s", err.Error())
		}

		m.eventCh <- &event
		return nil
	}, http.MethodPost)
}

func (m *Manager) subscribeNotifications() hedgeErrors.HedgeError {
	// notification call back
	subscription := []requests.AddSubscriptionRequest{{
		BaseRequest: commonDtos.NewBaseRequest(),
		Subscription: dtos.Subscription{
			Name:        "meta-sync-notify",
			Description: "metadata change subscription",
			Receiver:    "meta-sync",
			Channels: []dtos.Address{{
				Type: "REST",
				Host: "meta-sync",
				Port: 48108,
				RESTAddress: dtos.RESTAddress{
					Path:       "/api/v3/state/refresh",
					HTTPMethod: http.MethodPost,
				},
			}},
			Categories: []string{
				"change-notify",
			},
			AdminState: "UNLOCKED",
		},
	}}

	// subscribe to notifications
	rs, err := m.service.SubscriptionClient().Add(context.Background(), subscription)
	if err != nil {
		m.service.LoggingClient().Errorf("failed subscription setup for notifications: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Failed to subscribe to notifications")
	}

	// check subscription status
	for _, r := range rs {
		// check subscription created or already exist
		if r.StatusCode != 200 && r.StatusCode != 201 && r.StatusCode != 409 {
			m.service.LoggingClient().Errorf("failed subscription setup for notifications: %v", r)
			return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, "Failed to subscribe to notifications")
		}
	}
	return nil
}

// returns error for test purpose
func (m *Manager) notifyExternal(events []*pkg.MetaEvent) error {
	seen := make(map[string]bool)
	for _, event := range events {
		if event == nil || event.Type == "" {
			continue
		}

		if _, found := seen[event.Name]; found {
			continue
		}
		seen[event.Name] = true

		m.service.LoggingClient().Debugf("notification event to be published to support-notification: %v", event)
		e, err := json.Marshal(event)
		if err != nil {
			m.service.LoggingClient().Errorf("error marshalling to MetaEvent for event:%v", e)
			return err
		}

		notification := dtos.Notification{
			Category: "sync-finished",
			Sender:   "meta-sync",
			Content:  string(e),
			Severity: model.Normal,
			Status:   model.New,
		}

		req := requests.NewAddNotificationRequest(notification)
		responses, err := m.service.NotificationClient().SendNotification(m.service.AppContext(), []requests.AddNotificationRequest{req})
		if err != nil {
			m.service.LoggingClient().Errorf("error publishing notification for sync finished: %v", err)
		} else if len(responses) > 0 {
			m.service.LoggingClient().Infof("response publishing support-notification: %v", responses)
		}
	}

	return nil
}
