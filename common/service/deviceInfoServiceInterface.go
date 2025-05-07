/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import "hedge/common/models"

type DeviceServiceInter interface {
	LoadProfileAndLabels() *DeviceInfoService
	GetDeviceProfiles() []string
	GetDeviceLabels() []string
	GetDevicesByLabels(labels []string) []string
	GetDevicesByLabelsCriteriaOR(labels []string) []string
	GetDeviceToDeviceInfoMap() map[string]models.DeviceInfo
	GetLabels() []string
	GetProfiles() []string
	GetDevicesByProfile(profile string) []string
	GetDevicesByLabel(label string) []string
	GetMetricsByDevices(devices []string) []string
	GetDeviceInfoMap() (deviceToDeviceInfoMap map[string]models.DeviceInfo, metricToDeviceInfoMap map[string][]models.DeviceInfo, err error)
	LoadDeviceInfoFromDB()
	ClearCache()
}
