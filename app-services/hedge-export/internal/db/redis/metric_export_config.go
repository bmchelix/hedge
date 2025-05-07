/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package redis

import (
	"encoding/json"
	"fmt"

	"hedge/app-services/hedge-export/internal/dto"
	"hedge/common/db"
	db2 "hedge/common/db"
	redis2 "hedge/common/db/redis"
	"github.com/gomodule/redigo/redis"
)

func (dbClient *DBClient) AddMetricExportData(metricConfig dto.MetricExportConfig, configName string) (string, error) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	return addMetricExportData(conn, metricConfig, configName)
}

func (dbClient *DBClient) UpdateMetricExportData(metricConfig dto.MetricExportConfig, configName string) (string, error) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	err := deleteMetricExportData(conn, configName)
	if err != nil {
		return "", err
	}

	return addMetricExportData(conn, metricConfig, configName)
}

func addMetricExportData(conn redis.Conn, metricConfig dto.MetricExportConfig, configName string) (string, error) {

	exists, err := redis.Bool(conn.Do("HEXISTS", db2.MetricExportConfig+":name", configName))
	if err != nil {
		return "", err
	} else if exists {
		return "", db.ErrNotUnique
	}

	m, err := marshalObject(metricConfig)
	if err != nil {
		return "", err
	}

	_ = conn.Send("MULTI")
	_ = conn.Send("SET", db2.MetricExportConfig+":"+configName, m)
	_ = conn.Send("ZADD", db2.MetricExportConfig, 0, configName)
	//_ = conn.Send("HSET", db2.MetricExportConfig+":name", configName, configName)

	_, err = conn.Do("EXEC")
	if err != nil {
		fmt.Println("Error While Saving MetricExportData in DB:", err.Error())
	}
	return metricConfig.Metric, err
}

func (dbClient *DBClient) GetMetricExportDataById(metricName string) (dto.MetricExportConfig, error) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	var metricConfig dto.MetricExportConfig
	err := redis2.GetObjectById(conn, metricName, unmarshalObject, &metricConfig)
	if err != nil {
		fmt.Println("Error While Get MetricExportData from DB:", err.Error())
	}
	return metricConfig, err
}

func (dbClient *DBClient) GetAllMetricExportData() ([]dto.MetricExportConfig, error) {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	objects, err := redis2.GetObjectsByRange(conn, db2.MetricExportConfig, 0, -1)
	if err != nil {
		fmt.Println("Error While Get All MetricExportData from DB:", err.Error())
		return []dto.MetricExportConfig{}, err
	}

	d := make([]dto.MetricExportConfig, len(objects))
	for i, object := range objects {
		err = unmarshalObject(object, &d[i])
		if err != nil {
			return []dto.MetricExportConfig{}, err
		}
	}

	return d, nil
}

func (dbClient *DBClient) DeleteMetricExportData(metricConfig string) error {
	conn := dbClient.Pool.Get()
	defer conn.Close()

	return deleteMetricExportData(conn, metricConfig)
}

func deleteMetricExportData(conn redis.Conn, metricConfig string) error {

	_ = conn.Send("MULTI")
	_ = conn.Send("DEL", metricConfig)
	_ = conn.Send("ZREM", db2.MetricExportConfig, metricConfig)
	_ = conn.Send("HDEL", db2.MetricExportConfig+":name", metricConfig)

	_, err := conn.Do("EXEC")
	if err != nil {
		fmt.Println("Error While Deleting MetricExportData in DB:", err.Error())
	}
	return err
}

func (dbClient *DBClient) SetIntervalExportData(ivl string) error {
	conn := dbClient.Pool.Get()
	defer conn.Close()
	err := conn.Send("SET", db2.MetricExportConfig+":"+"ival", ivl)
	if err != nil {
		return err
	}
	return nil
}

func (dbClient *DBClient) GetIntervalExportData() (interface{}, error) {
	conn := dbClient.Pool.Get()
	defer conn.Close()
	var interval interface{}
	object, err := redis.Bytes(conn.Do("GET", db2.MetricExportConfig+":"+"ival"))
	if err != nil {
		return nil, err
	}
	json.Unmarshal(object, &interval)
	return interval, nil
}
