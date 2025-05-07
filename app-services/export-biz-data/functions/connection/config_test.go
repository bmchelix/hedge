/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package connection

import (
	dbmocks "hedge/mocks/hedge/app-services/export-biz-data/db"
	"hedge/mocks/hedge/app-services/export-biz-data/functions/connection"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	appMock            *utils.HedgeMockUtils
	mockedSqlInterface *dbmocks.MockSqlDB
)

func init() {
	appMock = utils.NewApplicationServiceMock(map[string]string{"HedgeAdminURL": "http://hedge-admin:4321"})
	mockedSqlInterface = &dbmocks.MockSqlDB{}

}

func TestConnectionString_ValidConfig_ResponseNoError(t *testing.T) {
	// given
	conf := Config{
		Host:     "host",
		Password: "0ivmaEHeZkCkxeWagJfPnogwSwj9nTrK2uzDrJoBdOWz",
		User:     "test",
		Port:     5432,
		DBName:   "test",
		SSLMode:  "disable",
	}

	// when
	connectionString, err := conf.ConnectionString()

	// then
	t.Run("Test config", func(t *testing.T) {
		t.Run("Error Should Be Nil", func(t *testing.T) {
			assert.Nil(t, err)
		})
		t.Run("Response As Expected", func(t *testing.T) {
			assert.Equal(t, connectionString, "postgresql://test:0ivmaEHeZkCkxeWagJfPnogwSwj9nTrK2uzDrJoBdOWz@host:5432/test?sslmode=disable")
		})
	})
}

func TestConnectionString_InvalidConfigPasswordNotBase64_ResponseError(t *testing.T) {
	// given
	conf := Config{
		Host:     "host",
		Password: "password-test",
		User:     "test",
		Port:     5432,
		DBName:   "test",
		SSLMode:  "disable",
	}

	// when
	connectionString, err := conf.ConnectionString()

	// then
	t.Run("Test config", func(t *testing.T) {
		t.Run("Error Should Be Nil", func(t *testing.T) {
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "Password")
		})
		t.Run("Response As Expected", func(t *testing.T) {
			assert.Equal(t, connectionString, "")
		})
	})
}

func TestConnectionString_InvalidConfigPasswordLength_ResponseError(t *testing.T) {
	// given
	conf := Config{
		Host:     "host",
		Password: "cGFzc3dvcmQtdGVzdA==",
		User:     "test",
		Port:     5432,
		DBName:   "test",
		SSLMode:  "disable",
	}

	// when
	connectionString, err := conf.ConnectionString()

	// then
	t.Run("Test config", func(t *testing.T) {
		t.Run("Error Should Be Nil", func(t *testing.T) {
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "Password")
		})
		t.Run("Response As Expected", func(t *testing.T) {
			assert.Equal(t, connectionString, "")
		})
	})
}

func TestPostgresConnect_Success(t *testing.T) {
	dbConnection := &connection.MockDbConnectionInterface{}
	dbConnection.On("CreatePostgresConnection", appMock.AppService).Return(mockedSqlInterface, nil)
	dbConnection.On("TestConnection", mockedSqlInterface).Return(nil)

	dbWrapper, err := PostgresConnect(appMock.AppService, dbConnection)

	assert.NoError(t, err)
	assert.NotNil(t, dbWrapper)
	dbConnection.AssertExpectations(t)
}

func TestPostgresConnect_Failure_CreateConnection(t *testing.T) {
	expectedError := errors.New("connection error")
	dbConnection := &connection.MockDbConnectionInterface{}
	dbConnection.On("CreatePostgresConnection", appMock.AppService).Return(nil, expectedError)

	dbWrapper, err := PostgresConnect(appMock.AppService, dbConnection)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Nil(t, dbWrapper)
	dbConnection.AssertExpectations(t)
}

func TestPostgresConnect_Failure_TestConnection(t *testing.T) {
	expectedError := errors.New("ping error")
	dbConnection := &connection.MockDbConnectionInterface{}
	dbConnection.On("CreatePostgresConnection", appMock.AppService).Return(mockedSqlInterface, nil)
	dbConnection.On("TestConnection", mockedSqlInterface).Return(expectedError)

	dbWrapper, err := PostgresConnect(appMock.AppService, dbConnection)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Nil(t, dbWrapper)
	dbConnection.AssertExpectations(t)
}

func TestDbConnection_TestConnection_Success(t *testing.T) {
	dbConnection := DbConnection{}
	mockedSqlInterface.On("Ping").Return(nil)

	err := dbConnection.TestConnection(mockedSqlInterface)

	assert.NoError(t, err)
	mockedSqlInterface.AssertExpectations(t)
}

func TestDbConnection_CreatePostgresConnection_Failure_EmptyPgInfo(t *testing.T) {
	dbConnection := DbConnection{}

	dbWrapper, err := dbConnection.CreatePostgresConnection(appMock.AppService)

	assert.Error(t, err)
	assert.Nil(t, dbWrapper)
}
