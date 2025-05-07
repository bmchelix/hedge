package broker

import (
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMLBrokerConfig_LoadConfigurations(t *testing.T) {
	u := utils.NewApplicationServiceMock(map[string]string{
		"MetaDataServiceUrl":       "http://metadata-service-url",
		"EventPipelineTriggerURL":  "http://event-pipeline-url",
		"LocalMLModelDir":          "/path/to/models",
		"NonExistingConfigSetting": "some-value",
	})

	mlCfg := NewMLBrokerConfig()
	mlCfg.LoadConfigurations(u.AppService)

	assert.Equal(t, "http://metadata-service-url", mlCfg.MetaDataServiceUrl)
	assert.Equal(t, "http://event-pipeline-url", mlCfg.EventsPublisherURL)
	assert.Equal(t, "/path/to/models", mlCfg.LocalMLModelBaseDir)
}

func TestMLBrokerConfig_NewMLBrokerConfig(t *testing.T) {
	expected := &MLBrokerConfig{}
	result := NewMLBrokerConfig()
	if result == nil {
		t.Errorf("NewMLBrokerConfig() returned nil, expected %v", expected)
		return
	}
}
