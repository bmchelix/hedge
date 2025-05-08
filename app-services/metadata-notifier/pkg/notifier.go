package pkg

import (
	"encoding/json"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos/requests"
	model "github.com/edgexfoundry/go-mod-core-contracts/v3/models"
)

const (
	DeviceType  = "device"
	ProfileType = "deviceprofile"
	NodeType    = "node"
	SceneType   = "scene"
	ImageType   = "image"

	AddAction    = "add"
	DeleteAction = "delete"
	UpdateAction = "update"
)

type Notifier struct {
	service interfaces.ApplicationService
}

type MetaEvent struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func NewNotifier(service interfaces.ApplicationService) *Notifier {
	notifier := new(Notifier)
	notifier.service = service
	return notifier
}

func (n *Notifier) PublishToSupportNotification(ctx interfaces.AppFunctionContext, data interface{}) (bool, interface{}) {
	if data == nil {
		// We didn't receive a result
		return false, errors.New("No Data Received")
	}
	systemEvent := data.(dtos.SystemEvent)
	ctx.LoggingClient().Infof("received event from %s type: %s action: %s", systemEvent.Owner, systemEvent.Type, systemEvent.Action)

	var name string
	switch systemEvent.Type {
	case DeviceType, ProfileType:
		name = systemEvent.Details.(map[string]interface{})["name"].(string)
	case NodeType:
		name = systemEvent.Details.(string)
	}

	content, err := json.Marshal(MetaEvent{Name: name, Type: systemEvent.Type})
	if err != nil {
		return false, nil
	}
	ctx.LoggingClient().Debugf("about to publish system-event with content: %v", content)
	notification := dtos.Notification{
		Category: "change-notify",
		Sender:   "metadata-notifier",
		Content:  string(content),
		Severity: model.Normal,
		Status:   model.New,
	}
	req := requests.NewAddNotificationRequest(notification)
	responses, err := ctx.NotificationClient().SendNotification(n.service.AppContext(), []requests.AddNotificationRequest{req})

	if err != nil {
		ctx.LoggingClient().Errorf("error publishing notification for new device: %v", err)
	}
	if responses != nil && len(responses) > 0 {
		ctx.LoggingClient().Infof("response publishing support-notification: %v", responses)
	}

	return false, nil
}
