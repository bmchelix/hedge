package device_services

import (
	"github.com/edgexfoundry/go-mod-bootstrap/v3/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/stretchr/testify/mock"
	"net/http"
)

// MockDeviceService is a mock implementation for the DeviceService interface
type MockDeviceService struct {
	mock.Mock
}

func (m *MockDeviceService) AddDeviceAutoEvent(deviceName string, event models.AutoEvent) error {
	args := m.Called(deviceName, event)
	return args.Error(0)
}

func (m *MockDeviceService) RemoveDeviceAutoEvent(deviceName string, event models.AutoEvent) error {
	args := m.Called(deviceName, event)
	return args.Error(0)
}

func (m *MockDeviceService) AddDevice(device models.Device) (string, error) {
	args := m.Called(device)
	var res string
	if args.Get(0) != nil {
		res = args.Get(0).(string)
	}
	return res, args.Error(1)
}

func (m *MockDeviceService) Devices() []models.Device {
	args := m.Called()
	var res []models.Device
	if args.Get(0) != nil {
		res = args.Get(0).([]models.Device)
	}
	return res
}

func (m *MockDeviceService) GetDeviceByName(name string) (models.Device, error) {
	args := m.Called(name)
	var res models.Device
	if args.Get(0) != nil {
		res = args.Get(0).(models.Device)
	}
	return res, args.Error(1)
}

func (m *MockDeviceService) DriverConfigs() map[string]string {
	args := m.Called()
	var res map[string]string
	if args.Get(0) != nil {
		res = args.Get(0).(map[string]string)
	}
	return res
}

func (m *MockDeviceService) SetDeviceOpState(name string, state models.OperatingState) error {
	args := m.Called(name, state)
	return args.Error(0)
}

func (m *MockDeviceService) RemoveDeviceByName(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockDeviceService) UpdateDevice(device models.Device) error {
	args := m.Called(device)
	return args.Error(0)
}

func (m *MockDeviceService) UpdateDeviceOperatingState(deviceName string, state string) error {
	args := m.Called(deviceName, state)
	return args.Error(0)
}

func (m *MockDeviceService) AddDeviceProfile(profile models.DeviceProfile) (string, error) {
	args := m.Called(profile)
	var res string
	if args.Get(0) != nil {
		res = args.Get(0).(string)
	}
	return res, args.Error(1)
}

func (m *MockDeviceService) DeviceProfiles() []models.DeviceProfile {
	args := m.Called()
	var res []models.DeviceProfile
	if args.Get(0) != nil {
		res = args.Get(0).([]models.DeviceProfile)
	}
	return res
}

func (m *MockDeviceService) GetProfileByName(name string) (models.DeviceProfile, error) {
	args := m.Called(name)
	var res models.DeviceProfile
	if args.Get(0) != nil {
		res = args.Get(0).(models.DeviceProfile)
	}
	return res, args.Error(1)
}

func (m *MockDeviceService) RemoveDeviceProfileByName(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockDeviceService) UpdateDeviceProfile(profile models.DeviceProfile) error {
	args := m.Called(profile)
	return args.Error(0)
}

func (m *MockDeviceService) DeviceCommand(deviceName string, commandName string) (models.DeviceCommand, bool) {
	args := m.Called(deviceName, commandName)
	var res models.DeviceCommand
	if args.Get(0) != nil {
		res = args.Get(0).(models.DeviceCommand)
	}
	return res, args.Get(1).(bool)
}

func (m *MockDeviceService) DeviceResource(deviceName string, deviceResource string) (models.DeviceResource, bool) {
	args := m.Called(deviceName, deviceResource)
	var res models.DeviceResource
	if args.Get(0) != nil {
		res = args.Get(0).(models.DeviceResource)
	}
	return res, args.Get(1).(bool)
}

func (m *MockDeviceService) AddProvisionWatcher(watcher models.ProvisionWatcher) (string, error) {
	args := m.Called(watcher)
	var res string
	if args.Get(0) != nil {
		res = args.Get(0).(string)
	}
	return res, args.Error(1)
}

func (m *MockDeviceService) ProvisionWatchers() []models.ProvisionWatcher {
	args := m.Called()
	var res []models.ProvisionWatcher
	if args.Get(0) != nil {
		res = args.Get(0).([]models.ProvisionWatcher)
	}
	return res
}

func (m *MockDeviceService) GetProvisionWatcherByName(name string) (models.ProvisionWatcher, error) {
	args := m.Called(name)
	var res models.ProvisionWatcher
	if args.Get(0) != nil {
		res = args.Get(0).(models.ProvisionWatcher)
	}
	return res, args.Error(1)
}

func (m *MockDeviceService) RemoveProvisionWatcher(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockDeviceService) UpdateProvisionWatcher(watcher models.ProvisionWatcher) error {
	args := m.Called(watcher)
	return args.Error(0)
}

func (m *MockDeviceService) Name() string {
	args := m.Called()
	var res string
	if args.Get(0) != nil {
		res = args.Get(0).(string)
	}
	return res
}

func (m *MockDeviceService) Version() string {
	args := m.Called()
	var res string
	if args.Get(0) != nil {
		res = args.Get(0).(string)
	}
	return res
}

func (m *MockDeviceService) AsyncReadings() bool {
	args := m.Called()
	var res bool
	if args.Get(0) != nil {
		res = args.Get(0).(bool)
	}
	return res
}

func (m *MockDeviceService) DeviceDiscovery() bool {
	args := m.Called()
	var res bool
	if args.Get(0) != nil {
		res = args.Get(0).(bool)
	}
	return res
}

func (m *MockDeviceService) AddRoute(route string, handler func(http.ResponseWriter, *http.Request), methods ...string) error {
	args := m.Called(route, handler, methods)
	return args.Error(0)
}

func (m *MockDeviceService) Stop(force bool) {
	m.Called(force)
	return
}

func (m *MockDeviceService) LoadCustomConfig(customConfig interfaces.UpdatableConfig, sectionName string) error {
	args := m.Called(customConfig, sectionName)
	return args.Error(0)
}

func (m *MockDeviceService) ListenForCustomConfigChanges(configToWatch interface{}, sectionName string, changedCallback func(interface{})) error {
	args := m.Called(configToWatch, sectionName, changedCallback)
	return args.Error(0)
}

func (m *MockDeviceService) GetLoggingClient() logger.LoggingClient {
	args := m.Called()
	var res logger.LoggingClient
	if args.Get(0) != nil {
		res = args.Get(0).(logger.LoggingClient)
	}
	return res
}

func (m *MockDeviceService) GetSecretProvider() interfaces.SecretProvider {
	args := m.Called()
	var res interfaces.SecretProvider
	if args.Get(0) != nil {
		res = args.Get(0).(interfaces.SecretProvider)
	}
	return res
}

func (m *MockDeviceService) GetMetricsManager() interfaces.MetricsManager {
	args := m.Called()
	var res interfaces.MetricsManager
	if args.Get(0) != nil {
		res = args.Get(0).(interfaces.MetricsManager)
	}
	return res
}
