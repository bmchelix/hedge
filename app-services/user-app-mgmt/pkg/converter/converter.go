/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package converter

import (
	"hedge/app-services/user-app-mgmt/pkg/dto"
)

func ConvertResourceEntityToMenuJSON(menuEntities *[]dto.Resources) dto.Resource {
	var response dto.Resource
	for _, menuEntity := range *menuEntities {
		if len(menuEntity.ParentResource) < 1 {
			//for parent menuItems
			response.Name = menuEntity.Name
			response.DisplayName = menuEntity.DisplayName
			//response.ResourceRefName = menuEntity.ResourceRefName
			response.LinkDetails.LinkUri = menuEntity.Uri
			response.LinkDetails.LinkType = menuEntity.LinkType
			response.SubResources = getSubMenuResourceResp(menuEntity.Name, menuEntities)
			break
		} else {
			//for subMenuItems
			//if len(menuEntity.ParentResource) > 0 {
			response.Name = menuEntity.Name
			response.DisplayName = menuEntity.DisplayName
			//response.ResourceRefName = menuEntity.ResourceRefName
			response.LinkDetails.LinkUri = menuEntity.Uri
			response.LinkDetails.LinkType = menuEntity.LinkType
			//}
		}

	}
	return response
}

func getSubMenuResourceResp(parentMenuItemName string, menus *[]dto.Resources) []dto.Resource {
	//r.appService.LoggingClient().Debugf("router.getSubMenuItems() :: Start")
	var subMenuItems []dto.Resource
	var subMenuItem dto.Resource
	for _, menu := range *menus {
		if menu.ParentResource == parentMenuItemName {
			subMenuItem.Name = menu.Name
			subMenuItem.DisplayName = menu.DisplayName
			//subMenuItem.ResourceRefName = menu.ResourceRefName
			subMenuItem.LinkDetails.LinkUri = menu.Uri
			subMenuItem.LinkDetails.LinkType = menu.LinkType
			subMenuItems = append(subMenuItems, subMenuItem)
		}
	}
	//r.appService.LoggingClient().Debugf("router.getSubMenuItems() :: End")
	return subMenuItems
}

func ConvertResourceEntityToMenuItemRes(resource dto.Resources, resources []dto.Resources, responseList []dto.Resource) []dto.Resource {
	var response dto.Resource
	response.Name = resource.Name
	response.DisplayName = resource.DisplayName
	// Need to handle at time of insertion, temp for now
	if resource.AllowedPermissions == "" {
		response.AllowedPermissions = "RW"
	} else {
		response.AllowedPermissions = resource.AllowedPermissions
	}
	//response.ResourceRefName = resource.ResourceRefName
	response.LinkDetails.LinkUri = resource.Uri
	response.LinkDetails.LinkType = resource.LinkType
	response.LinkDetails.LinkId = resource.UiId
	response.Id = resource.Id
	response.SubResources = getSubMenuItemFromResources(resource.Name, resources)
	responseList = append(responseList, response)
	return responseList
}

func ConvertResourceEntitiesToResources(entityResources []dto.Resources) []dto.Resource {
	resources := make([]dto.Resource, 0)
	for _, entityRes := range entityResources {
		if entityRes.Uri == "" || entityRes.LinkType == "" {
			continue
		}
		var resource dto.Resource
		resource.Name = entityRes.Name
		//resource.Tooltip = entityRes.T
		resource.DisplayName = entityRes.DisplayName
		// Need to handle at time of insertion, temp for now
		if entityRes.AllowedPermissions == "" {
			resource.AllowedPermissions = "RW"
		} else {
			resource.AllowedPermissions = entityRes.AllowedPermissions
		}
		//response.ResourceRefName = resource.ResourceRefName
		resource.LinkDetails.LinkUri = entityRes.Uri
		resource.LinkDetails.LinkType = entityRes.LinkType
		resource.LinkDetails.LinkId = entityRes.UiId
		resources = append(resources, resource)
	}
	return resources
}

func getSubMenuItemFromResources(parentMenuItemName string, resources []dto.Resources) []dto.Resource {
	//r.appService.LoggingClient().Debugf("router.getSubMenuItems() :: Start")
	var subMenuItems []dto.Resource
	var subMenuItem dto.Resource
	for _, resource := range resources {
		if resource.ParentResource == parentMenuItemName {
			subMenuItem.Name = resource.Name
			subMenuItem.DisplayName = resource.DisplayName
			if resource.AllowedPermissions == "" {
				subMenuItem.AllowedPermissions = "RW"
			} else {
				subMenuItem.AllowedPermissions = resource.AllowedPermissions
			}
			//subMenuItem.ResourceRefName = resource.ResourceRefName
			subMenuItem.LinkDetails.LinkUri = resource.Uri
			subMenuItem.LinkDetails.LinkType = resource.LinkType
			subMenuItem.LinkDetails.LinkId = resource.UiId
			subMenuItems = append(subMenuItems, subMenuItem)
		}
	}
	//r.appService.LoggingClient().Debugf("router.getSubMenuItems() :: End")
	return subMenuItems
}

func ConvertMenuJSONToResourceEntity(menuInput dto.Resource) []dto.Resources {
	var menuOutput dto.Resources
	var menuOutputList []dto.Resources
	//parent Resource
	menuOutput.Name = menuInput.Name
	menuOutput.DisplayName = menuInput.DisplayName
	menuOutput.Uri = menuInput.LinkDetails.LinkUri
	menuOutput.LinkType = menuInput.LinkDetails.LinkType
	menuOutput.UiId = menuInput.LinkDetails.LinkId
	menuOutput.AllowedPermissions = menuInput.AllowedPermissions
	//menuOutput.ResourceRefName = menuInput.ResourceRefName
	menuOutputList = append(menuOutputList, menuOutput)
	if len(menuInput.SubResources) > 0 {
		//child Resource
		for _, subMenuItem := range menuInput.SubResources {
			menuOutput.Name = subMenuItem.Name
			menuOutput.DisplayName = subMenuItem.DisplayName
			menuOutput.Uri = subMenuItem.LinkDetails.LinkUri
			menuOutput.LinkType = subMenuItem.LinkDetails.LinkType
			menuOutput.UiId = subMenuItem.LinkDetails.LinkId
			//menuOutput.ResourceRefName = subMenuItem.ResourceRefName
			menuOutput.ParentResource = menuInput.Name
			menuOutputList = append(menuOutputList, menuOutput)
		}
	}
	return menuOutputList
}
