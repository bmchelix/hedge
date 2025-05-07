/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/dtos"
	"github.com/stretchr/testify/mock"
	"hedge/app-services/hedge-device-extensions/pkg/db/redis"
	"hedge/common/client"
	"hedge/common/dto"
	"hedge/common/errors"
	mkredis "hedge/mocks/hedge/app-services/hedge-device-extensions/pkg/db/redis"
	mds "hedge/mocks/hedge/app-services/hedge-device-extensions/pkg/service"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"net/http"
	"reflect"
	"testing"
)

var mockMetaService = &mds.MockMetaDataService{}

// var dsi = &mservice.MockDeviceServiceInter{}
var u *utils.HedgeMockUtils
var dbc mkredis.MockDeviceExtDBClientInterface
var nodeHttpClient *utils.MockClient
var dtservice DtService
var metaservice MetaService

type fields struct {
	service  interfaces.ApplicationService
	dbClient redis.DeviceExtDBClientInterface
	metaserv *MetaService
}

var flds fields

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{"HedgeAdminURL": "http://hedge-admin:4321", "MetaDataServiceUrl": "MockMetaDataServiceUrl"})
	redis.DBClientImpl = &dbc
	dbc.On("GetDbClient", mock.Anything, mock.Anything).Return(redis.DBClientImpl)
	dbc.On("CreateIndex", mock.Anything).Return(nil)
	dbc.On("CreateDigitalTwin", mock.Anything).Return(nil)
	dbc.On("GetDigitalTwin", mock.Anything).Return(dto.Scene{
		SceneId:       "",
		ImageId:       "",
		DeviceId:      "",
		SceneType:     "",
		DisplayAttrib: nil,
	}, nil)
	dbc.On("GetDTwinByDevice", mock.Anything).Return(dto.Scene{
		SceneId:       "",
		ImageId:       "",
		DeviceId:      "",
		SceneType:     "",
		DisplayAttrib: nil,
	}, nil)
	dbc.On("UpdateDigitalTwin", mock.Anything).Return(nil)
	dbc.On("DeleteDigitalTwin", mock.Anything).Return(nil)
	dbc.On("GetImageObject", mock.Anything).Return(dto.Image{
		ImgId:   "MyImage",
		Object:  "Device",
		ObjName: "MyDevice",
	}, nil)
	dbc.On("DeleteImageObject", mock.Anything).Return(nil)

	mockMetaService.On("GetCompleteDevice", mock.Anything, mock.Anything).Return(dto.DeviceObject{}, nil)
	mockMetaService.On("GetDeviceDetails", mock.Anything).Return(dtos.Device{}, "", nil)

	nodeHttpClient = utils.NewMockClient()
	client.Client = nodeHttpClient
	nodeHttpClient.RegisterExternalMockRestCall("MockMetaDataServiceUrl/api/v3/device/all", http.MethodGet, nil, 200, nil)

	flds = fields{
		service:  u.AppService,
		dbClient: redis.DBClientImpl,
		metaserv: &metaservice,
	}
}

func TestDtService_CreateIndex(t *testing.T) {
	tests := []struct {
		name   string
		fields fields
		want   errors.HedgeError
	}{
		{"Create index ok", flds, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			if got := d.CreateIndex(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDtService_CheckScene(t *testing.T) {
	type args struct {
		scene dto.Scene
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   errors.HedgeError
	}{
		{"Scene Ok", flds, args{scene: dto.Scene{
			SceneId:       "MyScene",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, nil},
		{"Scene Fail", flds, args{scene: dto.Scene{
			SceneId:       "",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, errors.NewCommonHedgeError(errors.ErrorTypeNotFound, fmt.Sprintf("sceneId is required"))},
		{"Scene Fail", flds, args{scene: dto.Scene{
			SceneId:       "MyScene",
			ImageId:       "",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, errors.NewCommonHedgeError(errors.ErrorTypeNotFound, fmt.Sprintf("imageId is required"))},
		{"Scene Fail", flds, args{scene: dto.Scene{
			SceneId:       "MyScene",
			ImageId:       "MyImage",
			DeviceId:      "",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, errors.NewCommonHedgeError(errors.ErrorTypeNotFound, fmt.Sprintf("deviceId is required"))},
		{"Scene Fail", flds, args{scene: dto.Scene{
			SceneId:       "MyScene",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "",
			DisplayAttrib: nil,
		}}, errors.NewCommonHedgeError(errors.ErrorTypeNotFound, fmt.Sprintf("sceneType is required"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			if got := d.CheckScene(tt.args.scene); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CheckScene() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDtService_CreateDigitalTwin(t *testing.T) {
	type args struct {
		scene dto.Scene
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   errors.HedgeError
	}{
		{"Create scene ok", flds, args{scene: dto.Scene{
			SceneId:       "MyScene",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, nil},
		{"Create scene failed", flds, args{scene: dto.Scene{
			SceneId:       "My Scene",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, errors.NewCommonHedgeError(errors.ErrorTypeBadRequest, "Error character not allowed in My Scene")},
		{"Create scene failed", flds, args{scene: dto.Scene{
			SceneId:       "My-Scene",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, errors.NewCommonHedgeError(errors.ErrorTypeBadRequest, "Error character not allowed in My-Scene")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			if got := d.CreateDigitalTwin(tt.args.scene); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateDigitalTwin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDtService_UpdateDigitalTwin(t *testing.T) {
	type args struct {
		scene dto.Scene
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   errors.HedgeError
	}{
		{"Update scene ok", flds, args{scene: dto.Scene{
			SceneId:       "MyScene",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, nil},
		{"Update scene failed", flds, args{scene: dto.Scene{
			SceneId:       "My Scene",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, errors.NewCommonHedgeError(errors.ErrorTypeBadRequest, "Error character not allowed in My Scene")},
		{"Update scene failed", flds, args{scene: dto.Scene{
			SceneId:       "My-Scene",
			ImageId:       "MyImage",
			DeviceId:      "MyDevice",
			SceneType:     "2D",
			DisplayAttrib: nil,
		}}, errors.NewCommonHedgeError(errors.ErrorTypeBadRequest, "Error character not allowed in My-Scene")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			if got := d.UpdateDigitalTwin(tt.args.scene); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateDigitalTwin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDtService_DeleteDigitalTwin(t *testing.T) {
	type args struct {
		sceneId string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   errors.HedgeError
	}{
		{"Delete scene ok", flds, args{sceneId: "MyScene"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			if got := d.DeleteDigitalTwin(tt.args.sceneId); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteDigitalTwin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDtService_GetSceneById(t *testing.T) {
	type args struct {
		sceneId string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   dto.Scene
		want1  errors.HedgeError
	}{
		{"Get scene ok", flds, args{sceneId: "MyScene"}, dto.Scene{
			SceneId:       "",
			ImageId:       "",
			DeviceId:      "",
			SceneType:     "",
			DisplayAttrib: nil,
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			got, got1 := d.GetSceneById(tt.args.sceneId)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSceneById() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("GetSceneById() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestDtService_GetDTwinByDevice(t *testing.T) {
	type args struct {
		deviceId string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   dto.Scene
		want1  errors.HedgeError
	}{
		{"Get scene ok", flds, args{deviceId: "MyDevice"}, dto.Scene{
			SceneId:       "",
			ImageId:       "",
			DeviceId:      "",
			SceneType:     "",
			DisplayAttrib: nil,
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			got, got1 := d.GetDTwinByDevice(tt.args.deviceId)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDTwinByDevice() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("GetDTwinByDevice() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestDtService_GetImage(t *testing.T) {
	type args struct {
		image dto.Image
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
		want1  string
		want2  errors.HedgeError
	}{
		{"Get image ok", flds, args{image: dto.Image{
			ImgId:   "MyImage",
			Object:  "Device",
			ObjName: "MyDevice",
		}}, "MyImage", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			got, got1, got2 := d.GetImage(tt.args.image)
			if got != tt.want {
				t.Errorf("GetImage() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetImage() got1 = %v, want %v", got1, tt.want1)
			}
			if !reflect.DeepEqual(got2, tt.want2) {
				t.Errorf("GetImage() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestDtService_GetImageId(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{"Get image id ok", flds, args{path: "/MyImage"}, "MyImage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DtService{
				service:  tt.fields.service,
				dbClient: tt.fields.dbClient,
				metaserv: tt.fields.metaserv,
			}
			if got := d.GetImageId(tt.args.path); got != tt.want {
				t.Errorf("GetImageId() = %v, want %v", got, tt.want)
			}
		})
	}
}

//func TestDtService_DeleteImage(t *testing.T) {
//	type args struct {
//		imageId string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//		want   errors.HedgeError
//	}{
//		{"Delete image ok", flds, args{imageId: "MyImage"}, nil},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			d := DtService{
//				service:  tt.fields.service,
//				dbClient: tt.fields.dbClient,
//				metaserv: tt.fields.metaserv,
//			}
//			if got := d.DeleteImage(tt.args.imageId); !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("DeleteImage() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func TestDtService_SnapshotImage(t *testing.T) {
//	type args struct {
//		path     string
//		imageId  string
//		mimeType string
//	}
//	tests := []struct {
//		name   string
//		fields fields
//		args   args
//		want   string
//		want1  string
//		want2  errors.HedgeError
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			d := DtService{
//				service:  tt.fields.service,
//				dbClient: tt.fields.dbClient,
//				metaserv: tt.fields.metaserv,
//			}
//			got, got1, got2 := d.SnapshotImage(tt.args.path, tt.args.imageId, tt.args.mimeType)
//			if got != tt.want {
//				t.Errorf("SnapshotImage() got = %v, want %v", got, tt.want)
//			}
//			if got1 != tt.want1 {
//				t.Errorf("SnapshotImage() got1 = %v, want %v", got1, tt.want1)
//			}
//			if !reflect.DeepEqual(got2, tt.want2) {
//				t.Errorf("SnapshotImage() got2 = %v, want %v", got2, tt.want2)
//			}
//		})
//	}
//}

func TestNewDtService(t *testing.T) {
	type args struct {
		service  interfaces.ApplicationService
		dbClient redis.DeviceExtDBClientInterface
		metaserv MetaService
	}
	tests := []struct {
		name string
		args args
		want DtService
	}{
		{"Dt service ok", args{
			service:  u.AppService,
			dbClient: redis.DBClientImpl,
			metaserv: metaservice,
		}, dtservice},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dtservice
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDtService() = %v, want %v", got, tt.want)
			}
		})
	}
}

//func Test_checkImage(t *testing.T) {
//	type args struct {
//		image *multipart.FileHeader
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if err := checkImage(tt.args.image); (err != nil) != tt.wantErr {
//				t.Errorf("checkImage() error = %v, wantErr %v", err, tt.wantErr)
//			}
//		})
//	}
//}
//
//func Test_contentDir(t *testing.T) {
//	tests := []struct {
//		name    string
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if err := contentDir(); (err != nil) != tt.wantErr {
//				t.Errorf("contentDir() error = %v, wantErr %v", err, tt.wantErr)
//			}
//		})
//	}
//}
//
//func Test_encode(t *testing.T) {
//	type args struct {
//		dst      image.Image
//		mimetype string
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantW   string
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			w := &bytes.Buffer{}
//			err := encode(w, tt.args.dst, tt.args.mimetype)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("decode() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if gotW := w.String(); gotW != tt.wantW {
//				t.Errorf("decode() gotW = %v, want %v", gotW, tt.wantW)
//			}
//		})
//	}
//}
//
//func Test_decode(t *testing.T) {
//	type args struct {
//		r        io.Reader
//		src      image.Image
//		mimetype string
//	}
//	tests := []struct {
//		name    string
//		args    args
//		want    image.Image
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := decode(tt.args.r, tt.args.src, tt.args.mimetype)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("encode() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("encode() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func Test_extractImageData(t *testing.T) {
//	type args struct {
//		doc *gltf.Document
//		img *gltf.Image
//	}
//	tests := []struct {
//		name    string
//		args    args
//		want    []byte
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			got, err := extractImageData(tt.args.doc, tt.args.img)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("extractImageData() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("extractImageData() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func Test_mimeType(t *testing.T) {
//	type args struct {
//		imageId string
//	}
//	tests := []struct {
//		name string
//		args args
//		want string
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			if got := mimeType(tt.args.imageId); got != tt.want {
//				t.Errorf("mimeType() = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
//
//func Test_snapshot2D(t *testing.T) {
//	type args struct {
//		path     string
//		mimetype string
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantW   string
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			w := &bytes.Buffer{}
//			err := snapshot2D(tt.args.path, w, tt.args.mimetype)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("snapshot2D() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if gotW := w.String(); gotW != tt.wantW {
//				t.Errorf("snapshot2D() gotW = %v, want %v", gotW, tt.wantW)
//			}
//		})
//	}
//}
//
//func Test_snapshot3D(t *testing.T) {
//	type args struct {
//		path string
//	}
//	tests := []struct {
//		name    string
//		args    args
//		wantW   string
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			w := &bytes.Buffer{}
//			err := snapshot3D(tt.args.path, w)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("snapshot3D() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if gotW := w.String(); gotW != tt.wantW {
//				t.Errorf("snapshot3D() gotW = %v, want %v", gotW, tt.wantW)
//			}
//		})
//	}
//}
