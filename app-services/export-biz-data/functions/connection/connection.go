/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package connection

import (
	database "hedge/app-services/export-biz-data/db"
	"database/sql"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	_ "github.com/lib/pq"
	"strconv"
)

const DBPassKey = "password"

type DbConnectionInterface interface {
	TestConnection(dbWrapper database.SqlInterface) error
	CreatePostgresConnection(service interfaces.ApplicationService) (database.SqlInterface, error)
}

type DbConnection struct{}

func PostgresConnect(service interfaces.ApplicationService, c DbConnectionInterface) (database.SqlInterface, error) {
	dbWrapper, err := c.CreatePostgresConnection(service)
	lc := service.LoggingClient()
	if err != nil {
		lc.Errorf("failed connecting to Postgres: %s", err.Error())
		return nil, err
	}
	err = c.TestConnection(dbWrapper)
	if err != nil {
		lc.Errorf("failed connecting/pinging to postgres: %s", err.Error())
		return nil, err
	}
	return dbWrapper, nil
}

func (c *DbConnection) TestConnection(dbWrapper database.SqlInterface) error {
	return dbWrapper.Ping()
}

func (c *DbConnection) CreatePostgresConnection(service interfaces.ApplicationService) (database.SqlInterface, error) {
	lc := service.LoggingClient()

	var conf Config
	var err error
	conf.Host, err = service.GetAppSetting("Pg_db_host")
	if err != nil {
		lc.Errorf("Usr_db_host Error: %v\n", err)
		return nil, err
	}
	conf.DBName, err = service.GetAppSetting("Pg_db_name")
	if err != nil {
		lc.Errorf("Usr_db_name Error: %v\n", err)
		return nil, err
	}
	conf.User, err = service.GetAppSetting("Pg_db_user")
	if err != nil {
		lc.Errorf("Usr_db_user Error: %v\n", err)
		return nil, err
	}
	dbCreds, err := service.SecretProvider().GetSecret("dbconnection", DBPassKey)
	if err != nil {
		lc.Errorf("Usr_db_password_file Error: %v\n", err)
		return nil, err
	}
	portStr, err := service.GetAppSetting("Pg_db_port")
	if err != nil {
		lc.Errorf("Usr_db_port Error: %v\n", err)
		return nil, err
	}

	conf.Password = dbCreds[DBPassKey]
	conf.Port, _ = strconv.Atoi(portStr)
	conf.SSLMode = "disable"

	lc.Warnf("Usr_db_host as in the application settings: %v", conf.Host)

	// Note that password should not have / character, otherwise the connect api fails
	psqlInfo, err := conf.ConnectionString()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		lc.Errorf("failed connecting to Postgres: %s", err.Error())
		return nil, err
	}

	return &database.SqlDBWrapper{Db: db}, err
}
