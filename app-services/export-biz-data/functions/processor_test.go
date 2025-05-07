/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	database "hedge/app-services/export-biz-data/db"
	"hedge/app-services/export-biz-data/functions/connection"
	cl "hedge/common/client"
	"hedge/common/models"
	dbmocks "hedge/mocks/hedge/app-services/export-biz-data/db"
	connmocks "hedge/mocks/hedge/app-services/export-biz-data/functions/connection"
	"hedge/mocks/hedge/common/service"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"strings"
	"time"

	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
)

var (
	u                *utils.HedgeMockUtils
	lc               logger.LoggingClient
	mockedHttpClient *utils.MockClient
	mockedHttpSender service.MockHTTPSenderInterface
	data             interface{}
	ctx              interfaces.AppFunctionContext
	bizData          []models.ExternalBusinessData
)

type fields struct {
	queries                    []string
	service                    interfaces.ApplicationService
	deviceExtnExportBizDataUrl string
	conn                       database.SqlInterface
	cache                      map[string]string
}

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{"Pg_db_password_file": "connection/test.txt", mock.Anything: mock.Anything})
	lc = logger.NewMockClient()
	ctx = pkg.NewAppFuncContextForTest("Test", lc)
	mockedHttpClient = utils.NewMockClient()
	cl.Client = mockedHttpClient
	mockedHttpSender = service.MockHTTPSenderInterface{}
	mockedHttpSender.On("HTTPPost", mock.Anything, mock.Anything).Return(true, nil)
	bizData = []models.ExternalBusinessData{
		{
			DeviceName: "deviceName",
			BizData: map[string]interface{}{
				"someOtherColumn": "someOtherColumn",
			},
		},
	}
}

func TestNewContextData(t *testing.T) {
	type args struct {
		service interfaces.ApplicationService
	}
	var (
		appServ1 *utils.HedgeMockUtils
	)
	appServ1 = utils.NewApplicationServiceMock(nil)

	arg := args{service: u.AppService}
	arg1 := args{service: appServ1.AppService}

	contextData := ContextData{}
	contextData.service = u.AppService
	contextData.queries = []string{""}

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		"", 0, "", "1234", "")
	db, _ := sql.Open("postgres", psqlInfo)
	dbWrapper := &database.SqlDBWrapper{Db: db}
	mockedConn := &connmocks.MockDbConnectionInterface{}
	mockedConn.On("CreatePostgresConnection", mock.Anything).Return(dbWrapper, nil)
	mockedConn.On("TestConnection", mock.Anything).Return(nil)
	conn = mockedConn

	tests := []struct {
		name string
		args args
		want *ContextData
	}{
		{"NewContextData - Passed", arg, &contextData},
		{"NewContextData - Failed1", arg1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got := NewContextData(tt.args.service)
			if tt.want != nil {
				got = &contextData
			} else {
				got = nil
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewContextData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContextData_QueryNewBizData(t *testing.T) {
	mockDB := new(dbmocks.MockSqlDB)
	mockDB1 := new(dbmocks.MockSqlDB)
	mockDB2 := new(dbmocks.MockSqlDB)
	mockDB3 := new(dbmocks.MockSqlDB)

	mockRows := new(dbmocks.MockRows)
	mockRows1 := new(dbmocks.MockRows)
	mockRows2 := new(dbmocks.MockRows)

	mockRows.On("Columns").Return([]string{"deviceName", "someOtherColumn"}, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Scan", mock.Anything, mock.Anything).Return(nil)
	mockDB1.On("Query", mock.Anything, mock.Anything).Return(mockRows, nil)

	type args struct {
		ctx  interfaces.AppFunctionContext
		data interface{}
	}
	fld1 := fields{
		queries: []string{"test query 1"},
		service: u.AppService,
		conn:    mockDB,
	}
	fld2 := fields{
		queries: []string{"test query 1", "test query 2"},
		service: u.AppService,
		conn:    mockDB2,
		cache:   make(map[string]string),
	}
	fld3 := fields{
		queries: []string{"test query 1", "test query 2"},
		service: u.AppService,
		conn:    mockDB1,
		cache:   make(map[string]string),
	}
	fld4 := fields{
		queries: []string{"test query 1"},
		service: u.AppService,
		conn:    mockDB3,
		cache:   map[string]string{"q0-deviceName": "something"},
	}
	arg := args{
		ctx:  ctx,
		data: data,
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
		want1  interface{}
	}{
		{"QueryNewBizData - Passed1", fld3, arg, true, bizData},
		{"QueryNewBizData - Passed2", fld1, arg, true, []models.ExternalBusinessData{}},
		{"QueryNewBizData - Passed3", fld2, arg, true, []models.ExternalBusinessData{}},
		{"QueryNewBizData - Passed4", fld4, arg, true, bizData},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if strings.Contains(tt.name, "Passed2") {
				mockDB.On("Query", mock.Anything, mock.Anything).Return(mockRows, errors.New("mocked error"))
			}
			if strings.Contains(tt.name, "Passed3") {
				mockRows1.On("Columns").Return([]string{"deviceName", "someOtherColumn"}, errors.New("mocked error")).Once()
				mockRows1.On("Columns").Return([]string{"deviceName", "someOtherColumn"}, nil)
				mockRows1.On("Next").Return(true).Once()
				mockRows1.On("Next").Return(false)
				mockRows1.On("Scan", mock.Anything, mock.Anything).Return(errors.New("mocked error"))
				mockDB2.On("Query", mock.Anything, mock.Anything).Return(mockRows1, nil)
			}
			if strings.Contains(tt.name, "Passed4") {
				mockRows2.On("Columns").Return([]string{"deviceName", "someOtherColumn"}, nil)
				mockRows2.On("Next").Return(true).Once()
				mockRows2.On("Next").Return(false)
				mockRows2.On("Scan", mock.Anything, mock.Anything).Return(nil)
				mockDB3.On("Query", mock.Anything, mock.Anything).Return(mockRows2, nil)
			}

			m := &ContextData{
				queries:                    tt.fields.queries,
				service:                    tt.fields.service,
				deviceExtnExportBizDataUrl: tt.fields.deviceExtnExportBizDataUrl,
				conn:                       tt.fields.conn,
				cache:                      tt.fields.cache,
			}
			got, got1 := m.QueryNewBizData(tt.args.ctx, tt.args.data)
			if got != tt.want {
				t.Errorf("QueryNewBizData() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("QueryNewBizData() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestPostgresConnect(t *testing.T) {
	mockConn := &connmocks.MockDbConnectionInterface{}
	mockDbWrapper := &dbmocks.MockSqlDB{}
	mockConn.On("CreatePostgresConnection", mock.Anything).Return(mockDbWrapper, nil)
	mockConn.On("TestConnection", mock.Anything).Return(nil)

	result, err := connection.PostgresConnect(u.AppService, mockConn)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	mockConn.AssertExpectations(t)
}

func TestHttpTrigger_CallSucceeded(t *testing.T) {
	mockHTTPClient := &service.MockHTTPClient{}
	cl.Client = mockHTTPClient

	server := "http://example.com"
	interval := "0"
	mockHTTPClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{StatusCode: http.StatusOK}, nil)

	// Create a channel to signal the completion of the test
	done := make(chan bool)

	go HttpTrigger(interval, server, done)
	time.Sleep(5 * time.Second)

	mockHTTPClient.AssertCalled(t, "Do", mock.AnythingOfType("*http.Request"))
	done <- true
}

func TestHttpTrigger_CallFailed(t *testing.T) {
	mockHTTPClient1 := &service.MockHTTPClient{}
	cl.Client = mockHTTPClient1

	server := "http://example.com"
	interval := "0"
	mockHTTPClient1.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{StatusCode: http.StatusBadRequest}, errors.New("mocked error"))

	// Create a channel to signal the completion of the test
	done := make(chan bool)

	go HttpTrigger(interval, server, done)
	time.Sleep(5 * time.Second)

	mockHTTPClient1.AssertCalled(t, "Do", mock.AnythingOfType("*http.Request"))
	done <- true
}

func TestContextData_HttpSend(t *testing.T) {
	flds := fields{
		deviceExtnExportBizDataUrl: "deviceExtnExportBizDataUrl",
	}
	type args struct {
		ctx  interfaces.AppFunctionContext
		data interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
		want1  interface{}
	}{
		{"HttpSend - Passed", flds, args{ctx, bizData}, true, nil},
		{"HttpSend - Failed1", flds, args{ctx, bizData}, true, nil},
		{"HttpSend - Failed2", flds, args{ctx, []byte(nil)}, false, json.SyntaxError{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if strings.Contains(tt.name, "Passed") {
				httpSender = &mockedHttpSender
			}
			m := &ContextData{
				queries:                    tt.fields.queries,
				service:                    tt.fields.service,
				deviceExtnExportBizDataUrl: tt.fields.deviceExtnExportBizDataUrl,
				conn:                       tt.fields.conn,
				cache:                      tt.fields.cache,
			}
			got, got1 := m.HttpSend(tt.args.ctx, tt.args.data)
			assert.Equalf(t, tt.want, got, "HttpSend(%v, %v)", tt.args.ctx, tt.args.data)
			if strings.Contains(tt.name, "Failed2") {
				var syntaxErr *json.SyntaxError
				gotError := got1.(*json.SyntaxError)
				assert.ErrorAs(t, gotError, &syntaxErr)
				assert.Equal(t, "unexpected end of JSON input", gotError.Error())
			} else {
				assert.Equalf(t, tt.want1, got1, "HttpSend(%v, %v)", tt.args.ctx, tt.args.data)
			}
			httpSender = nil
		})
	}
}
