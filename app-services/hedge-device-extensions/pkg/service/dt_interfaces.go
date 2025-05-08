/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"mime/multipart"
)

type DigitalTwinService interface {
	GetSceneById(sceneId string) (dto.Scene, hedgeErrors.HedgeError)
	CreateDigitalTwin(scene dto.Scene) hedgeErrors.HedgeError
	UpdateDigitalTwin(scene dto.Scene) hedgeErrors.HedgeError
	DeleteDigitalTwin(deviceId string) hedgeErrors.HedgeError
	CheckScene(scene dto.Scene) hedgeErrors.HedgeError

	GetDeviceById(deviceId string, metrics string) (dto.DeviceObject, hedgeErrors.HedgeError)
	GetDTwinByDevice(deviceId string) (dto.Scene, hedgeErrors.HedgeError)

	UploadImage(img *multipart.FileHeader, image dto.Image) hedgeErrors.HedgeError
	GetImage(image dto.Image) (string, string, hedgeErrors.HedgeError)
	DeleteImage(imageId string) hedgeErrors.HedgeError
	SnapshotImage(path string, imageId string, mimeType string) (string, string, hedgeErrors.HedgeError)
	GetImageId(path string) string

	CreateIndex() hedgeErrors.HedgeError
}
