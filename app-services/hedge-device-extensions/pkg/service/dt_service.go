/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/qmuntal/gltf"
	"golang.org/x/image/draw"
	"hedge/app-services/hedge-device-extensions/pkg/db/redis"
	"hedge/common/dto"
	hedgeErrors "hedge/common/errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"mime/multipart"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

const (
	PNG          = "image/png"
	JPG          = "image/jpeg"
	GLB          = "model/gltf-binary"
	IMG_WIDTH    = 300      // width in pixels to resize for snapshot
	IMG_MAX_SIZE = 50000000 // 50MB max size image to upload
)

type DtService struct {
	service  interfaces.ApplicationService
	dbClient redis.DeviceExtDBClientInterface
	metaserv *MetaService
	imgDir   string
}

func NewDtService(service interfaces.ApplicationService, dbClient redis.DeviceExtDBClientInterface, metaserv *MetaService) DtService {
	dtserv := DtService{}
	dtserv.service = service
	dtserv.dbClient = dbClient
	dtserv.metaserv = metaserv
	var er error
	dtserv.imgDir, er = service.GetAppSetting("ImageDir")
	if er != nil {
		dtserv.service.LoggingClient().Errorf("Error getting ImageDir from config: %v", er)
		os.Exit(1)
	}
	if err := contentDir(dtserv.imgDir); err != nil {
		service.LoggingClient().Errorf("Directory creation failed with error: " + err.Error())
		os.Exit(1)
	}
	return dtserv
}

func (d DtService) GetSceneById(sceneId string) (dto.Scene, hedgeErrors.HedgeError) {
	dt, err := d.dbClient.GetDigitalTwin(sceneId)
	if err != nil {
		d.service.LoggingClient().Errorf("Error getting digital-twin from redis: %v", err)
		return dt, err
	}
	return dt, nil
}

func (d DtService) CreateDigitalTwin(scene dto.Scene) hedgeErrors.HedgeError {
	err := checkNames(scene)
	if err != nil {
		d.service.LoggingClient().Errorf("Error with params: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("Error %s", err.Error()))
	}
	er := d.dbClient.CreateDigitalTwin(scene)
	if er != nil {
		d.service.LoggingClient().Errorf("Error creating digital-twin in redis: %v", err)
		return er
	}
	return nil
}

func (d DtService) UpdateDigitalTwin(scene dto.Scene) hedgeErrors.HedgeError {
	err := checkNames(scene)
	if err != nil {
		d.service.LoggingClient().Errorf("Error with params: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, fmt.Sprintf("Error %s", err.Error()))
	}
	// if update is of image, delete old image
	oldScene, err := d.GetSceneById(scene.SceneId)
	if oldScene.ImageId != scene.ImageId {
		d.DeleteImage(oldScene.ImageId)
	}
	er := d.dbClient.UpdateDigitalTwin(scene)
	if er != nil {
		d.service.LoggingClient().Errorf("Error getting digital-twin from redis: %v", err)
		return er
	}
	return nil
}

func (d DtService) DeleteDigitalTwin(sceneId string) hedgeErrors.HedgeError {
	err := d.dbClient.DeleteDigitalTwin(sceneId)
	if err != nil {
		d.service.LoggingClient().Errorf("Error getting digital-twin from redis: %v", err)
		return err
	}
	return nil
}

func (d DtService) CheckScene(scene dto.Scene) hedgeErrors.HedgeError {
	if scene.SceneId == "" {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("sceneId is required"))
	}
	if scene.ImageId == "" {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("imageId is required"))
	}
	if scene.DeviceId == "" {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("deviceId is required"))
	}
	if scene.SceneType == "" {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("sceneType is required"))
	}
	return nil
}

func (d DtService) GetDeviceById(deviceId string, metrics string) (dto.DeviceObject, hedgeErrors.HedgeError) {
	device, err := d.metaserv.GetCompleteDevice(deviceId, metrics, d.service)
	if err != nil {
		d.service.LoggingClient().Errorf("Error getting digital-twin from redis: %v", err)
		return device, err
	}
	var chDevs []dto.ChildDevice

	if len(device.Associations) > 0 {
		for _, v := range device.Associations {
			cd, err := d.metaserv.GetCompleteDevice(v.NodeName, "", d.service)
			if err != nil {
				d.service.LoggingClient().Errorf("Error getting digital-twin from redis: %v", err)
				return device, err
			}
			chldDev := dto.ChildDevice{
				DeviceId:    cd.Device.Name,
				Description: cd.Device.Description,
				Status:      cd.Device.OperatingState,
			}
			chDevs = append(chDevs, chldDev)
		}
		if device.Device.Properties == nil {
			device.Device.Properties = make(map[string]interface{})
		}
		device.Device.Properties["ChildDevices"] = chDevs
	}
	return device, nil
}

func (d DtService) GetDTwinByDevice(deviceId string) (dto.Scene, hedgeErrors.HedgeError) {
	dt, err := d.dbClient.GetDTwinByDevice(deviceId)
	if err != nil {
		d.service.LoggingClient().Errorf("Error getting digital-twin from redis: %v", err)
		return dt, err
	}
	return dt, nil
}

func (d DtService) UploadImage(img *multipart.FileHeader, image dto.Image) hedgeErrors.HedgeError {
	err := checkNames(image)
	if err != nil {
		d.service.LoggingClient().Errorf("error with params: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, err.Error())
	}
	err = checkImage(img)
	if err != nil {
		d.service.LoggingClient().Errorf("file requirement failed: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeBadRequest, err.Error())
	}
	// Source
	src, err := img.Open()
	if err != nil {
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}
	defer src.Close()

	// Destination
	path := filepath.Join(d.imgDir, image.ImgId)
	dst, err := os.OpenFile(path, os.O_EXCL|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		d.service.LoggingClient().Errorf("error creating image file: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}
	defer dst.Close()

	// Copy
	if _, err = io.Copy(dst, src); err != nil {
		d.service.LoggingClient().Errorf("error saving image to filesystem: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}

	err = d.dbClient.SaveImageObject(image)
	if err != nil {
		d.service.LoggingClient().Errorf("error saving image path to redis: %v", err)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, err.Error())
	}
	return nil
}

func (d DtService) GetImage(image dto.Image) (string, string, hedgeErrors.HedgeError) {
	var img dto.Image
	var err hedgeErrors.HedgeError
	er := checkNames(image)
	if er != nil {
		d.service.LoggingClient().Errorf("Error with params: %v", er)
		return "", "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeNotFound, fmt.Sprintf("Error %s", er.Error()))
	}

	if image.ImgId == "all" {
		img, err = d.dbClient.GetAllImages(image)
	} else {
		img, err = d.dbClient.GetImageObject(image)
	}

	if err != nil {
		d.service.LoggingClient().Errorf("Error getting digital-twin from redis: %v", err)
		return "", "", err
	}
	imgPath := filepath.Join(d.imgDir, img.ImgId)
	imgType := mimeType(img.ImgId)
	return imgPath, imgType, nil
}

func (d DtService) DeleteImage(imageId string) hedgeErrors.HedgeError {
	path := filepath.Join(d.imgDir, imageId)
	err := d.dbClient.DeleteImageObject(imageId)
	if err != nil {
		d.service.LoggingClient().Errorf("Error deleting image from redis: %v", err)
		return err
	}
	er := os.Remove(path)
	if er != nil {
		//TODO: rollback if err above
		d.service.LoggingClient().Errorf("Error deleting image from filesystem: %v", er)
		return hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error %s", er.Error()))
	}
	return nil
}

func (d DtService) GetImageId(path string) string {
	arr := strings.SplitAfter(path, "/")
	return arr[len(arr)-1]
}

func (d DtService) SnapshotImage(path string, imageId string, mimeType string) (string, string, hedgeErrors.HedgeError) {
	var name string
	glb := false
	if mimeType == GLB {
		imageId = imageId + ".jpg"
		mimeType = JPG
		glb = true
	}

	// Destination
	dst, err := os.Create(d.imgDir + "/tmp/" + imageId)
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating image file: %v", err)
		return "", "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error %s", err.Error()))
	}
	defer dst.Close()

	// Resize
	if glb {
		err = snapshot3D(path, dst)
		name = dst.Name() + ".jpg"
	} else {
		err = snapshot2D(path, dst, mimeType)
		name = dst.Name()
	}
	if err != nil {
		d.service.LoggingClient().Errorf("Error saving image path to redis: %v", err)
		return "", "", hedgeErrors.NewCommonHedgeError(hedgeErrors.ErrorTypeServerError, fmt.Sprintf("Error %s", err.Error()))
	}
	return name, mimeType, nil
}

func (d DtService) CreateIndex() hedgeErrors.HedgeError {
	err := d.dbClient.CreateIndex()
	if err != nil {
		d.service.LoggingClient().Errorf("Error creating digital-twin index in redis: %v", err)
		return err
	}
	return nil
}

func checkImage(image *multipart.FileHeader) error {
	if image.Size > IMG_MAX_SIZE {
		return errors.New("file is too big")
	}

	switch exp := filepath.Ext(image.Filename); strings.ToUpper(exp) {
	case ".PNG":
		return nil
	case ".JPG":
		return nil
	case ".JPEG":
		return nil
	case ".GLB":
		return nil
	default:
		return errors.New("file format not valid")
	}
}

func mimeType(imageId string) string {
	switch exp := filepath.Ext(imageId); strings.ToUpper(exp) {
	case ".PNG":
		return PNG
	case "JPEG":
		return JPG
	case ".JPEG":
		return JPG
	case ".GLB":
		return GLB
	default:
		return ""
	}
}

func checkNames(strct interface{}) error {
	var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9_.]+`)
	var char string
	count := 0
	v := reflect.ValueOf(strct)
	types := v.Type()
	if types.Name() == "Scene" {
		count = 3
	} else {
		count = v.NumField()
	}
	for i := 0; i < count; i++ {
		if types.Field(i).Name != "displayAttrib" {
			char = nonAlphanumericRegex.FindString(v.Field(i).Interface().(string))
			if char != "" {
				return errors.New("character not allowed in " + v.Field(i).Interface().(string))
			}
		}
	}
	return nil
}

func snapshot2D(path string, w io.Writer, mimetype string) error {
	var src image.Image
	var err error
	// Source
	r, err := os.Open(path)
	if err != nil {
		return err
	}
	defer r.Close()

	src, err = decode(r, src, mimetype)
	if err != nil {
		return err
	}

	ratio := (float64)(src.Bounds().Max.Y) / (float64)(src.Bounds().Max.X)
	height := int(math.Round(float64(IMG_WIDTH) * ratio))
	dst := image.NewRGBA(image.Rect(0, 0, IMG_WIDTH, height))
	draw.NearestNeighbor.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)
	encode(w, dst, mimetype)
	return nil
}

func snapshot3D(path string, w io.Writer) error {
	// Load the GLB file
	doc, err := gltf.Open(path)
	if err != nil {
		return err
	}
	// Extract the first texture (if it exists)
	if len(doc.Images) > 0 {
		img := doc.Images[0]

		// Decode the image data
		imgData, err := extractImageData(doc, img)
		if err != nil {
			return err
		}

		// Save the image as JPG
		_, err = w.Write(imgData)
		if err != nil {
			return err
		}

		// Reduce size of imgData
		newF := w.(*os.File).Name()
		defer os.RemoveAll(newF)
		f, err := os.Create(newF + ".jpg")
		snapshot2D(newF, f, JPG)
	} else {
		errors.New("no images found in the GLB file")
	}
	return nil
}

func extractImageData(doc *gltf.Document, img *gltf.Image) ([]byte, error) {
	var data []byte
	var err error

	// Determine the source of the image data (either embedded base64 or a buffer view)
	if img.URI != "" {
		// If the image is embedded in the URI, decode the base64 data
		if img.URI[:5] == "data:" {
			commaIdx := bytes.IndexByte([]byte(img.URI), ',')
			if commaIdx == -1 {
				return nil, err
			}
			data, err = base64.StdEncoding.DecodeString(img.URI[commaIdx+1:])
			if err != nil {
				return nil, err
			}
		} else {
			// If the URI points to an external file (not common in GLB files)
			return nil, err
		}
	} else if img.BufferView != nil {
		// If the image data is stored in a buffer view
		bv := doc.BufferViews[*img.BufferView]
		buffer := doc.Buffers[bv.Buffer]
		data = buffer.Data[bv.ByteOffset : bv.ByteOffset+bv.ByteLength]
	}
	return data, nil
}

func decode(r io.Reader, src image.Image, mimetype string) (image.Image, error) {
	var err error
	switch mimetype {
	case "image/jpeg":
		src, err = jpeg.Decode(r)
	case "image/png":
		src, err = png.Decode(r)
	}
	return src, err
}

func encode(w io.Writer, dst image.Image, mimetype string) error {
	var err error
	switch mimetype {
	case "image/jpeg":
		err = jpeg.Encode(w, dst, nil)
	case "image/png":
		err = png.Encode(w, dst)
	}
	return err
}

func contentDir(dir string) error {
	err := os.MkdirAll(dir+"/tmp", 0755)
	if err == nil {
		return nil
	}
	if os.IsExist(err) {
		// check that the existing path is a directory
		info, err := os.Stat(dir + "/tmp")
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("path exists but is not a directory")
		}
		return nil
	}
	return err
}
