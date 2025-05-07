/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package functions

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"hedge/app-services/export-biz-data/db"
	pg "hedge/app-services/export-biz-data/functions/connection"
	"hedge/common/client"
	bmcmodel "hedge/common/models"
	"hedge/common/service"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/transforms"
)

type ContextData struct {
	queries                    []string
	service                    interfaces.ApplicationService
	deviceExtnExportBizDataUrl string
	conn                       db.SqlInterface
	cache                      map[string]string
}

var (
	conn       pg.DbConnectionInterface
	httpSender service.HTTPSenderInterface
)

func NewContextData(service interfaces.ApplicationService) *ContextData {
	contextData := ContextData{}
	contextData.service = service
	if conn == nil {
		conn = new(pg.DbConnection)
	}
	connect, err := pg.PostgresConnect(service, conn)
	if err != nil {
		service.LoggingClient().Errorf("failed connecting to Postgres: %s", err.Error())
		return nil
	}
	contextData.conn = connect
	contextData.cache = make(map[string]string)
	query, err := service.GetAppSettingStrings("Queries")

	if err != nil && len(query) < 1 {
		service.LoggingClient().
			Errorf("failed to retrieve ApiServer from configuration: %s", err.Error())
	}
	tmp := strings.Join(query, ",")
	contextData.queries = strings.Split(tmp, ";")

	contextDataIngestionAPI, err := service.GetAppSetting("Device_Extn")
	if err != nil {
		service.LoggingClient().Error("unable to get Device_Extn configuration")
		os.Exit(-1)
	}
	contextData.deviceExtnExportBizDataUrl = contextDataIngestionAPI + "/api/v3/metadata/device/biz"

	return &contextData
}

// QueryNewBizData gets new biz data from database and publishes to ContextData Broker
func (m *ContextData) QueryNewBizData(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {
	lc := ctx.LoggingClient()
	lc.Debugf("QueryNewBizData called in export-biz-data pipeline")
	conn := m.conn

	bizData := make([]bmcmodel.ExternalBusinessData, 0)

	// Loop through the array of queries
	for i := range m.queries {
		rows, err := conn.Query(m.queries[i])
		if err != nil {
			lc.Errorf(
				"error retrieving data from Postgres for query: %s, error: %v",
				m.queries[i],
				err,
			)
			continue
		}
		cols, err := rows.Columns()
		if err != nil {
			lc.Errorf("error parsing data from Postgres: %v", err)
			continue
		}

		rowData := make([]interface{}, len(cols))
		processedDevice := make(map[string]struct{}, 0)
		for rows.Next() {
			deviceColIndex := -1

			for i := range cols {
				rowData[i] = &rowData[i]
				if cols[i] == "devicename" || cols[i] == "deviceName" || cols[i] == "device" ||
					cols[i] == "device_name" {
					deviceColIndex = i
				}
			}
			// For every query, only one per device to be considered

			if deviceColIndex == -1 {
				lc.Errorf("Missing column: deviceName in the query: %s", m.queries[i])
				continue
			}

			// Populate the data
			err = rows.Scan(rowData...)
			if err != nil {
				lc.Errorf("error processing data from Postgres")
				continue
			}

			if rowData[deviceColIndex] == nil {
				lc.Errorf("deviceName column has blank value")
				continue
			}

			deviceName := rowData[deviceColIndex].(string)
			if _, ok := processedDevice[deviceName]; ok {
				// Device is already processed for this query, so continue
				continue
			}
			processedDevice[deviceName] = struct{}{}

			// build the lookup key for cache
			key := fmt.Sprintf("q%d-%s", i, deviceName)
			// Check if this device exists in cache
			if val, ok := m.cache[key]; ok {
				if val == base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(rowData))) {
					continue
				} else {
					m.cache[key] = base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(rowData)))
				}
			} else {
				m.cache[key] = base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(rowData)))
			}

			var publishData bmcmodel.ExternalBusinessData

			publishData.DeviceName = deviceName
			vals := make(map[string]interface{}, len(cols))
			for i := range cols {
				if i != deviceColIndex {
					vals[cols[i]] = rowData[i]
				}
			}
			// Get string value from interface
			publishData.BizData = vals
			bizData = append(bizData, publishData)
			lc.Infof("exporting new BizData to hedge-device-extensions: %v", publishData)
		}
	}

	return true, bizData
}

func (m *ContextData) HttpSend(
	ctx interfaces.AppFunctionContext,
	data interface{},
) (bool, interface{}) {

	var bizDataItems []bmcmodel.ExternalBusinessData

	if reflect.TypeOf(data) == reflect.TypeOf([]uint8(nil)) {
		err := json.Unmarshal(data.([]byte), &bizDataItems)
		if err != nil {
			ctx.LoggingClient().Errorf("Error unmarshalling command: %v", err)
			return false, err
		}
	} else {
		bizDataItems, _ = data.([]bmcmodel.ExternalBusinessData)
	}

	for _, bizDataItem := range bizDataItems {
		url := m.deviceExtnExportBizDataUrl + "/" + bizDataItem.DeviceName
		dataBytes, _ := json.Marshal(bizDataItem.BizData)

		var ok bool
		if httpSender != nil {
			ok, _ = httpSender.HTTPPost(ctx, dataBytes)
		} else {
			ok, _ = transforms.NewHTTPSender(url, "application/json", false).HTTPPost(ctx, dataBytes)
		}
		if !ok {
			ctx.LoggingClient().
				Errorf("failed to publish biz data for device: %s, data: %v", bizDataItem.DeviceName, bizDataItem.BizData)
		} else {
			ctx.LoggingClient().Infof("successfully published biz data to device-extension svc for device: %s, data: %v", bizDataItem.DeviceName, bizDataItem.BizData)
		}
	}
	return true, nil
}

// HttpTrigger self triggers the pipeline every 5 minutes
func HttpTrigger(interval string, server string, done chan bool) {
	intvl, _ := strconv.Atoi(interval)
	ticker := &time.Ticker{}
	if intvl < 1 { // for test purpose only
		ticker = time.NewTicker(3 * time.Second)
	} else {
		ticker = time.NewTicker(time.Duration(intvl) * time.Minute)
	}
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tmp, _ := json.Marshal("{}")
			pBody := bytes.NewReader(tmp)
			req, _ := http.NewRequest("POST", server, pBody)
			req.Header.Set("Content-Type", "application/json")
			_, err := client.Client.Do(req)
			if err != nil {
				fmt.Printf("function HttpTrigger failed http call: %s", err)
			}
		case <-done: // for test purpose only (pass nil for "done" argument to ignore this case)
			return
		}
	}
}
