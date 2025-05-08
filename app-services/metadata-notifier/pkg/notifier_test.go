package pkg

import (
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"errors"
	mocks2 "github.com/edgexfoundry/go-mod-core-contracts/v3/clients/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
)

var (
	appMock                *utils.HedgeMockUtils
	mockNotificationClient *mocks2.NotificationClient
)

func Init() {
	appMock = utils.NewApplicationServiceMock(map[string]string{})
	mockNotificationClient = &mocks2.NotificationClient{}
	mockNotificationClient.On("SendNotification", mock.Anything, mock.Anything).Return([]common.BaseWithIdResponse{{}}, nil)
	appMock.AppFunctionContext.On("NotificationClient").Return(mockNotificationClient)
}

func TestNotifier_PublishToSupportNotification1(t *testing.T) {
	systemEventWithAddAction := dtos.NewSystemEvent(DeviceType,
		"add",
		"HVAC",
		"Telco",
		map[string]string{
			"devicelabels": "Telco-Tower-A,HVAC,Temperature,Pressure,Humidity", "host": "",
		}, map[string]interface{}{
			"name":        "device_name",
			"profileName": "WindTurbine",
			"id":          "f1c5f0e8-6b64-48ad-91b7-c5981b5ca3b9",
			"description": "Example of Device Virtual",
		})

	systemEventWithNonAddAction := systemEventWithAddAction
	systemEventWithNonAddAction.Action = "remove"

	noCallCheck := func(t *testing.T, client *mocks2.NotificationClient) {
		assert.Equalf(t, 0, len(client.Calls), "SendNotification method shouldn't be called")
	}

	sendNotificationCalledOnceCheck := func(t *testing.T, client *mocks2.NotificationClient) {
		client.AssertNumberOfCalls(t, "SendNotification", 1)
	}

	tests := []struct {
		name                  string
		inputData             interface{}
		want                  bool
		want1                 interface{}
		notificationMockCheck func(t *testing.T, client *mocks2.NotificationClient)
	}{
		{
			"PublishToSupportNotification - system event input action - PASSED", systemEventWithAddAction, false, nil, sendNotificationCalledOnceCheck,
		},
		{
			"PublishToSupportNotification - system event input is nil - FAILED", nil, false, errors.New("No Data Received"), noCallCheck,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init()
			n := &Notifier{
				service: appMock.AppService,
			}
			got, got1 := n.PublishToSupportNotification(appMock.AppFunctionContext, tt.inputData)
			if got != tt.want {
				t.Errorf("PublishToSupportNotification() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("PublishToSupportNotification() got1 = %v, want %v", got1, tt.want1)
			}

			tt.notificationMockCheck(t, mockNotificationClient)
		})
	}
}

func TestNewNotifier(t *testing.T) {
	notifier := NewNotifier(appMock.AppService)
	assert.NotNil(t, notifier)
	assert.Equal(t, appMock.AppService, notifier.service)
}
