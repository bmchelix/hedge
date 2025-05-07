/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package config

import (
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"os"
	"time"
)

const DBPassKey = "password"

func GetDbConnection(service interfaces.ApplicationService) (*gorm.DB, error) {
	lc := service.LoggingClient()
	host, err1 := service.GetAppSetting("Usr_db_host")
	port, err2 := service.GetAppSetting("Usr_db_port")
	dbname, err3 := service.GetAppSetting("Usr_db_name")
	user, err4 := service.GetAppSetting("Usr_db_user")
	dbCreds, err5 := service.SecretProvider().GetSecret("dbconnection", DBPassKey)
	if err1 != nil {
		lc.Errorf("Usr_db_host Error: %v\n", err1)
		os.Exit(-1)
	}
	if err2 != nil {
		lc.Errorf("Usr_db_port Error: %v\n", err2)
		os.Exit(-1)
	}
	if err3 != nil {
		lc.Errorf("Usr_db_name Error: %v\n", err3)
		os.Exit(-1)
	}
	if err4 != nil {
		lc.Errorf("Usr_db_user Error: %v\n", err4)
		os.Exit(-1)
	}
	if err5 != nil {
		lc.Errorf("Usr_db_password_file Error: %v\n", err5)
		os.Exit(-1)
	}
	lc.Debugf("Usr_db_host as in the application settings: %v", host)

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, dbCreds[DBPassKey], dbname)

	// Connect to database
	for {
		db, err := gorm.Open(postgres.Open(psqlInfo), &gorm.Config{
			NamingStrategy: schema.NamingStrategy{
				TablePrefix:   "hedge.", // schema name
				SingularTable: false,
			},
		})
		if err != nil {
			//keep trying to connect to DB
			lc.Errorf("Failed connecting to DB (%s:%s|%s|%s). Retrying..", host, port, user, dbname)
			time.Sleep(2 * time.Second)
			continue
			//return nil, err
		}

		lc.Debugf("Successfully connected!")
		return db, nil
	}
}
