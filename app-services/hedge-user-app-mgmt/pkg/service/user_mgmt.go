/*******************************************************************************
* Contributors: BMC Helix, Inc.
*
* (c) Copyright 2020-2025 BMC Helix, Inc.

* SPDX-License-Identifier: Apache-2.0
*******************************************************************************/

package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"hedge/app-services/hedge-user-app-mgmt/pkg/converter"
	"hedge/app-services/hedge-user-app-mgmt/pkg/dto"
	"hedge/app-services/hedge-user-app-mgmt/pkg/repository"
	"hedge/common/client"
	"io"
	"net/http"
	"strings"
	"time"
)

type UserManagementService struct {
	appService               interfaces.ApplicationService
	dbConfig                 map[string]string
	userManagementRepository repository.UserMgmtRepository
	dashboardURL             string
}

type User struct {
	UserId string `json:"userId" validate:"max=200,matchRegex=^[a-zA-Z0-9][a-zA-Z0-9_-]*$"`
}

type Consumer struct {
	Username  string `json:"username"`
	Custom_id string `json:"custom_id"`
}

type LoginInputs struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Dash struct {
	Uid   string `json:"uid"`
	Title string `json:"title"`
	Url   string `json:"url"`
	Type  string `json:"type"`
}

type Res struct {
	Name string
	Uri  string
}

type Roles struct {
	Role []string `json:"role"`
}

type RoleResources struct {
	RoleName      string   `json:"roleName" validate:"max=200,matchRegex=^[a-zA-Z0-9_\\-]+$"`
	ResourcesName []string `json:"resourceName" validate:"dive,min=1,max=100"`
	Permissions   []string `json:"permission" validate:"dive,min=1,max=100"`
}

var UserManagementRepositoryImpl repository.UserMgmtRepository

func NewUserManagementService(appService interfaces.ApplicationService, dbConfig map[string]string) *UserManagementService {
	userManagementService := new(UserManagementService)
	userManagementService.dbConfig = dbConfig
	userManagementService.appService = appService
	if UserManagementRepositoryImpl == nil {
		UserManagementRepositoryImpl = repository.NewUserManagementRepository(appService)
	}
	userManagementService.userManagementRepository = UserManagementRepositoryImpl
	grafanaServerBaseURL, ok := dbConfig["Grafana_Server"]
	if !ok {
		appService.LoggingClient().Errorf("Grafana_Server missing from configuration")
	} else {
		userManagementService.dashboardURL = grafanaServerBaseURL + "/api/search"
	}
	/*	pass, ok := dbConfig["Grafana_Pass"]
		username, ok := dbConfig["Grafana_Admin"]

		var dash []Dash
		var exist = false*/

	return userManagementService
}

//func Login(userName string, password string) (bool, error) {
//	// query the database using the userName and password and return if the user is authenticated or not
//	// return error if the filter (key) is invalid and also return error if no item found
//	return true, nil
//}

//func GetResources(userName string) ([]dto.Resources, error) {
//	return nil, nil
//}

func (s UserManagementService) InitializeHedgeDB() error {

	err := s.userManagementRepository.InitializeHedgeDB()
	if err != nil {
		return err
	}
	// Initialize the admin userid and password. In general, if SSO is in place, this one will not be used
	// We still set this more as recovery one
	username, err := s.appService.GetAppSetting("USERNAME")
	if err != nil {
		s.appService.LoggingClient().Warnf("USERNAME not found in configuration, admin user will be assumed")
		username = "admin"
	} else {
		username = strings.TrimSpace(username)
	}

	pwdEncoded, err := s.appService.GetAppSetting("PLATCRD")
	if err != nil || pwdEncoded == "" {
		s.appService.LoggingClient().Warnf("PLATCRD not found in configuration, so admin password will not be set")
		return nil
	}

	// pass the password from env and use it in here to update the password
	pwd, err := base64.StdEncoding.DecodeString(pwdEncoded)
	if err != nil {
		s.appService.LoggingClient().Warnf("decode error: %v, admin password will not be set", err)
		return nil
	}
	loginInputs := LoginInputs{Username: username, Password: string(pwd[:len(pwd)-1])}
	err = s.CreateCredential(loginInputs)
	// donot return error in case of CreateCredential at the startup since there is a default already in place
	return nil
}

// GetUser Gets User
func (s UserManagementService) GetUser(userName string) ([]dto.User, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetUser():: Start")
	users, err := s.userManagementRepository.GetUser(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching User with userName: %v ", userName)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetUser():: End")
	return users, nil
}

// GetUserEmail Gets User by email
func (s UserManagementService) GetUserEmail(userName string) ([]dto.User, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetUserEmail():: Start")
	users, err := s.userManagementRepository.GetUserEmail(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching User with userName: %v ", userName)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetUserEmail():: End")
	return users, nil
}

// GetInactiveUser Gets User that has been set inactive
func (s UserManagementService) GetInactiveUser(userName string) ([]dto.User, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetUser():: Start")
	users, err := s.userManagementRepository.GetInactiveUser(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching User with userName: %v ", userName)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetUser():: End")
	return users, nil
}

// CreateCredential Creates User Credentials
func (s UserManagementService) CreateCredential(credential LoginInputs) error {
	s.appService.LoggingClient().Debugf("user_mgmt.CreateCredential():: Start")
	err := s.userManagementRepository.CreateCredential(credential.Username, credential.Password)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while creating credentials: %s ", err.Error())
		return err
	}

	s.appService.LoggingClient().Debugf("user_mgmt.CreateCredential():: End")
	return nil
}

// DeleteCredential Deletes User Credentials
func (s UserManagementService) DeleteCredential(username string) error {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteCredentials():: Start")
	err := s.userManagementRepository.DeleteCredential(username)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while creating credentials: %s ", err.Error())
		return err
	}

	s.appService.LoggingClient().Debugf("user_mgmt.DeleteCredential():: End")
	return nil
}

// CreateUser Creates User
func (s UserManagementService) CreateUser(newUser dto.User) (*dto.User, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.CreateUser():: Start")
	var result dto.User

	newUser.CreatedOn = time.Now().Format(time.RFC3339)
	newUser.CreatedBy = "admin"

	err := s.userManagementRepository.CreateUser(newUser)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while creating User with userName: %v ", newUser.FullName)
		return nil, err
	}
	users, err := s.userManagementRepository.GetUser(newUser.KongUsername)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching the User with userName: %v ", newUser.FullName)
		return nil, err
	}
	if len(users) > 0 {
		result = users[0]
	}

	s.appService.LoggingClient().Debugf("user_mgmt.CreateUser():: End")
	return &result, nil
}

// UpdateUser Updates User
func (s UserManagementService) UpdateUser(updatedUser *dto.User, userName string) (*dto.User, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateUser():: Start")

	var result *dto.User
	err := s.userManagementRepository.UpdateUser(updatedUser)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while updating user with userName: %v ", userName)
		return nil, err
	}

	users, err := s.userManagementRepository.GetUser(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching the updatedUser from database with userId: %v ", userName)
		return nil, err
	}

	if len(users) > 0 {
		result = &users[0]
	}
	s.appService.LoggingClient().Debugf("User updated successfully with the userId: %v ", userName)
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateUser():: End")
	return result, nil
}

// CreateUserRole Creates UserRole
func (s UserManagementService) CreateUserRole(newUserRole *dto.UserRole, roles Roles) (*dto.UserRole, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.CreateUserRole():: Start")
	for _, role := range roles.Role {
		newUserRole.RoleName = role
		err := s.userManagementRepository.CreateUserRole(newUserRole)
		if err != nil {
			s.appService.LoggingClient().Errorf("Error while creating UserRole with userId: %v ", newUserRole.UserKongUsername)
			return nil, err
		}
	}

	s.appService.LoggingClient().Debugf("user_mgmt.CreateUserRole():: End")
	return newUserRole, nil
}

// DeleteUserRoles Deletes UserRole
func (s UserManagementService) DeleteUserRoles(userId string) error {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteUserRoles():: Start")
	err := s.userManagementRepository.DeleteUserRoles(userId)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while deleting the UserRole for user: %v ", userId)
		return err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteUserRoles():: End")
	return nil
}

// DeleteUser Deletes User
func (s UserManagementService) DeleteUser(userName string) error {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteUser():: Start")
	err := s.userManagementRepository.DeleteUser(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while deleting the user : %v ", err)
		return err
	}

	err = s.DeleteCredential(userName)
	if err != nil {
		return err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteUser():: End")
	return nil
}

func populateDefaultFieldsResources(resource *dto.Resources) (*dto.Resources, error) {
	resource.CreatedOn = time.Now().Format(time.RFC3339)
	resource.CreatedBy = "admin"
	resource.ModifiedOn = time.Now().Format(time.RFC3339)
	resource.ModifiedBy = "admin"
	return resource, nil
}

func populateDefaultFieldsRole(role *dto.Role) (*dto.Role, error) {
	role.CreatedOn = time.Now().Format(time.RFC3339)
	role.CreatedBy = "admin"
	role.ModifiedOn = time.Now().Format(time.RFC3339)
	role.ModifiedBy = "admin"
	for i := range role.Resources {
		role.Resources[i].CreatedOn = time.Now().Format(time.RFC3339)
		role.Resources[i].CreatedBy = "admin"
		role.Resources[i].ModifiedOn = time.Now().Format(time.RFC3339)
		role.Resources[i].ModifiedBy = "admin"
		fmt.Println(role.Resources[i])
	}
	return role, nil
}

// GetResource Get Resources
func (s UserManagementService) GetResource(resourceName string) (dto.Resources, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetResource():: Start")
	resource, err := s.userManagementRepository.GetResource(resourceName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching Resource with resourceName: %v ", resourceName)
		return resource, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetResource():: End")
	return resource, nil
}

// CreateResource Creates Resources
func (s UserManagementService) CreateResource(newResource dto.Resources) (*dto.Resources, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.CreateResource():: Start")
	var result dto.Resources
	newResource.CreatedOn = time.Now().Format(time.RFC3339)
	newResource.CreatedBy = "admin"
	newResource.ModifiedOn = time.Now().Format(time.RFC3339)
	newResource.ModifiedBy = "admin"

	err := s.userManagementRepository.CreateResource(newResource)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while creating Resource : %v ", err)
		return nil, err
	}

	//resources, err := s.userManagementRepository.GetResource(newResource.Name)
	//if err != nil {
	//	s.appService.LoggingClient().Errorf("Error while fetching the Resources: %v ", newResource.Name)
	//	return nil, err
	//}
	//if len(resource) > 0 {
	//	result = resources[0]
	//}

	s.appService.LoggingClient().Debugf("user_mgmt.CreateResource():: End")
	return &result, nil
}

// DeleteResource Deletes Resource
func (s UserManagementService) DeleteResource(resourceName string) error {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteResource():: Start")
	err := s.userManagementRepository.DeleteResource(resourceName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while deleting the user : %v ", err)
		return err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteResource():: End")
	return nil
}

// CreateRole Creates Role
func (s UserManagementService) CreateRole(newRole *dto.Role) (*dto.Role, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.CreateRole():: Start")
	newRole.CreatedOn = time.Now().Format(time.RFC3339)
	newRole.CreatedBy = "admin"
	if len(newRole.Resources) > 0 {
		newRole.Resources[0].CreatedOn = time.Now().Format(time.RFC3339)
		newRole.Resources[0].CreatedBy = "admin"
	}
	role, err := s.userManagementRepository.CreateRole(newRole)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while creating Role with RoleName: %v ", newRole.Name)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.CreateRole():: End")
	return role, nil
}

// GetRole Gets Role
func (s UserManagementService) GetRole(roleName string) ([]*dto.Role, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetRole():: Start")
	roles, err := s.userManagementRepository.GetRoles(roleName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching User with userName: %v ", roleName)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetRole():: End")
	return roles, nil
}

// UpdateRole Updates Role
func (s UserManagementService) UpdateRole(updatedRole *dto.Role) (*dto.Role, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateRole():: Start")
	updatedRole, err := populateDefaultFieldsRole(updatedRole)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while populatingDefault fields for  Role with RoleName: %v ", updatedRole.Name)
		return nil, err
	}
	role, err := s.userManagementRepository.UpdateRole(updatedRole)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while updating Role with RoleName: %v ", updatedRole.Name)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateRole():: End")
	return role, nil
}

// DeleteRole Deletes Role
func (s UserManagementService) DeleteRole(roleName string) (string, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteRole():: Start")
	_, err := s.userManagementRepository.DeleteRole(roleName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while deleting the Role with the roleName: %v ", roleName)
		return "", err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteRole():: End")
	return "OK", err
}

// GetResourcesByUser Gets Resources by User
func (s UserManagementService) GetResourcesByUser(userName string) ([]dto.Resources, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetResourcesByUser():: Start")
	resources, err := s.userManagementRepository.GetResourcesByUser(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while getting resources for user: %v ", userName)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetResourcesByUser():: End")
	return resources, err
}

// GetResourcesUrlByUser Gets Resources by User - For Kong auth plugin
func (s UserManagementService) GetResourcesUrlByUser(userName string) ([]string, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetResourcesUrlByUser():: Start")
	userRoleUrls, err := s.userManagementRepository.GetResourcesUrlByUser(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while getting Urls for user: %v ", userName)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetResourcesUrlByUser():: End")
	return userRoleUrls, err
}

// GetMenuResource Get Menu
func (s UserManagementService) GetMenuResource(menuName string, fetchResourcesOnly bool) ([]dto.Resource, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetMenu():: Start")
	var err error
	var entityResources []dto.Resources
	entityResources, err = s.userManagementRepository.GetMenuResource(menuName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching Menu : %v ", err)
		return nil, err
	}
	//Convert MenuEntity To Resource
	var responseList []dto.Resource
	if fetchResourcesOnly {
		responseList = converter.ConvertResourceEntitiesToResources(entityResources)
	} else {
		if len(entityResources) == 1 {
			responseList = converter.ConvertResourceEntityToMenuItemRes(entityResources[0], entityResources, responseList)
		} else {
			for _, resource := range entityResources {
				if len(resource.ParentResource) < 1 {
					//for parent menuItems
					responseList = converter.ConvertResourceEntityToMenuItemRes(resource, entityResources, responseList)
				}
			}
		}
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetMenu():: End")
	return responseList, nil
}

// CreateMenuResources Create Menu
func (s UserManagementService) CreateMenuResources(newMenuItemRes dto.Resource) error {
	s.appService.LoggingClient().Debugf("user_mgmt.CreateMenuResources():: Start")
	newMenus := converter.ConvertMenuJSONToResourceEntity(newMenuItemRes)
	err := s.userManagementRepository.CreateMenuResources(&newMenus)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while creating Menu : %v ", err)
		return err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.CreateMenuResources():: End")
	return nil
}

// UpdateMenuResources Update Menu
func (s UserManagementService) UpdateMenuResources(menuInput dto.Resource) (*dto.Resource, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateMenus():: Start")
	updatedMenus := converter.ConvertMenuJSONToResourceEntity(menuInput)
	menuResponse, err := s.userManagementRepository.UpdateMenuResources(&updatedMenus)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while updating Resource: %v ", err)
		return nil, err
	}
	updatedMenusItemResp := converter.ConvertResourceEntityToMenuJSON(menuResponse)
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateMenus():: End")
	return &updatedMenusItemResp, nil
}

// DeleteMenuResource Deletes Menu
func (s UserManagementService) DeleteMenuResource(menuName string) error {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteMenuResource():: Start")
	err := s.userManagementRepository.DeleteMenuResource(menuName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while deleting the menu : %v ", err)
		return err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteMenuResource():: End")
	return nil
}

/*func (s UserManagementService) GetMenusByUser(userName string) ([]dto.Menu, error) {
	menus, err := s.userManagementRepository.GetMenusByUser(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while deleting the Role with the roleName: %v ", userName)
		return nil, err
	}
	return menus, err
}*/

//Update Menu
/*func (s UserManagementService) UpdateMenu(updatedMenu *dto.Menu) (*dto.Menu, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateMenu():: Start")
	updatedMenu, err := populateDefaultFieldsMenu(updatedMenu)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while populatingDefault fields for  Menu with MenuName: %v ", updatedMenu.Name)
		return nil, err
	}
	menu, err := s.userManagementRepository.UpdateMenu(updatedMenu)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while updating Menu with MenuName: %v ", updatedMenu.Name)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateMenu():: End")
	return menu, nil
}*/

// GetUserPreference Gets User Preferences
func (s UserManagementService) GetUserPreference(userName string) ([]dto.UserPreference, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.GetUserPreference():: Start")
	userPreferences, err := s.userManagementRepository.GetUserPreference(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching UserPreference with userName: %v ", userName)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.GetUserPreference():: End")
	return userPreferences, nil
}

// CreateUserPreference Creates User Preferences
func (s UserManagementService) CreateUserPreference(newUserPreference dto.UserPreference) (*dto.UserPreference, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.CreateUserPreference():: Start")
	var result dto.UserPreference

	newUserPreference.CreatedOn = time.Now().Format(time.RFC3339)
	newUserPreference.CreatedBy = "admin"

	err := s.userManagementRepository.CreateUserPreference(newUserPreference)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while creating UserPreference with userName: %v ", newUserPreference.KongUsername)
		return nil, err
	}
	userPreferences, err := s.userManagementRepository.GetUserPreference(newUserPreference.KongUsername)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching the UserPreference with userName: %v ", newUserPreference.KongUsername)
		return nil, err
	}
	if len(userPreferences) > 0 {
		result = userPreferences[0]
	}

	s.appService.LoggingClient().Debugf("user_mgmt.CreateUserPreference():: End")
	return &result, nil
}

// UpdateUserPreference Updates User Preferences
func (s UserManagementService) UpdateUserPreference(updatedUserPreference *dto.UserPreference, userName string) (*dto.UserPreference, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateUserPreference():: Start")

	var result *dto.UserPreference
	updatedUserPreference.KongUsername = userName
	err := s.userManagementRepository.UpdateUserPreference(updatedUserPreference)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while updating userPreference with userName: %v ", userName)
		return nil, err
	}

	userPreferences, err := s.userManagementRepository.GetUserPreference(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while fetching the updatedUserPreference from database with userName: %v ", userName)
		return nil, err
	}

	if len(userPreferences) > 0 {
		result = &userPreferences[0]
	}
	s.appService.LoggingClient().Debugf("UserPreference updated successfully with the userName: %v ", userName)
	s.appService.LoggingClient().Debugf("user_mgmt.UpdateUserPreference():: End")
	return result, nil
}

// DeleteUserPreference Deletes User Preference
func (s UserManagementService) DeleteUserPreference(userName string) error {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteUserPreference():: Start")
	err := s.userManagementRepository.DeleteUserPreference(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while deleting the userPreference : %v ", err)
		return err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteUserPreference():: End")
	return nil
}

// GetUserContextLandingPage Get User Context
func (s UserManagementService) GetUserContextLandingPage(userName string) (dto.LinkDetails, error) {
	var linkDetails dto.LinkDetails
	var userPreference dto.UserPreference
	userPreferences, err := s.GetUserPreference(userName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while getting the userPreference : %v ", err)
		return linkDetails, err
	}

	if len(userPreferences) > 0 {
		userPreference = userPreferences[0]
		resourceName := userPreference.ResourceName
		resource, err := s.GetResource(resourceName)
		if err != nil {
			s.appService.LoggingClient().Errorf("Error while getting resources : %v ", err)
			return linkDetails, err
		}
		linkDetails.LinkType = resource.LinkType
		linkDetails.LinkUri = resource.Uri
		linkDetails.LinkId = resource.UiId
	}
	return linkDetails, err
}

// CreateRoleResourcePermission Create Role-Resource-Permission
func (s UserManagementService) CreateRoleResourcePermission(roleResource *dto.RoleResourcePermission) (*dto.RoleResourcePermission, error) {
	s.appService.LoggingClient().Debugf("user_mgmt.CreateRoleResourcePermission():: Start")
	result, err := s.userManagementRepository.CreateRoleResourcePermission(roleResource)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while creating Role-Resource: %v ", roleResource.RoleName)
		return nil, err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.CreateRoleResourcePermission():: End")
	return result, nil
}

// UpdateRoleResourcePermission Update User Preferences
//func (s UserManagementService) UpdateRoleResourcePermission(roleResource *dto.RoleResourcePermission) (*dto.RoleResourcePermission, error) {
//	s.appService.LoggingClient().Debugf("user_mgmt.UpdateRoleResourcePermission():: Start")
//	result, err := s.userManagementRepository.UpdateRoleResourcePermission(roleResource)
//	if err != nil {
//		s.appService.LoggingClient().Errorf("Error while updating Role-Resource: %v ", roleResource.RoleName+"-"+roleResource.ResourcesName)
//		return nil, err
//	}
//	s.appService.LoggingClient().Debugf("user_mgmt.UpdateRoleResourcePermission():: End")
//	return result, nil
//}

// DeleteRoleResourcePermission Delete Role Resource Permission
func (s *UserManagementService) DeleteRoleResourcePermission(roleName string) error {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteRoleResourcePermission():: Start")
	err := s.userManagementRepository.DeleteRoleResourcePermission(roleName)
	if err != nil {
		s.appService.LoggingClient().Errorf("Error while deleting the RoleResourcePermission : %v ", err)
		return err
	}
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteRoleResourcePermission():: End")
	return nil
}

// SaveNewGrafanaDashboard Request all dashboards from server
func (s *UserManagementService) SaveNewGrafanaDashboard(setting map[string]string) error {
	var resource dto.Resources

	pass, ok := setting["Grafana_Pass"]
	if !ok {
		s.appService.LoggingClient().Errorf("Grafana_Pass missing from configuration")
		return fmt.Errorf("Grafana_Pass missing from configuration")
	}
	username, ok := setting["Grafana_Admin"]
	if !ok {
		s.appService.LoggingClient().Errorf("Grafana_Admin missing from configuration")
		return fmt.Errorf("Grafana_Admin missing from configuration")
	}

	var dash []Dash
	req, _ := http.NewRequest("GET", s.dashboardURL, nil)
	req.SetBasicAuth(username, pass)
	response, err := client.Client.Do(req)
	if err != nil {
		return fmt.Errorf("got error %s", err.Error())
	}
	defer response.Body.Close()
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	json.Unmarshal(resBody, &dash)
	resources, err := s.userManagementRepository.GetAllDashboardResources()
	if err != nil {
		s.appService.LoggingClient().Errorf("Error getting all grafana resources: %v", err)
		return err
	}

	//Remove from resources the deleted dashboards
	var exist = false
	for _, v := range resources {
		for _, j := range dash {
			if v["uri"] == j.Url {
				exist = true
				break
			}
		}
		if !exist {
			err := s.DeleteResource(v["name"])
			if err != nil {
				s.appService.LoggingClient().Errorf("Error deleting dashboard resource: %v", err)
				continue
			}
		}
		exist = false
	}
	//Add new dashboards
	exist = false
	for _, v := range dash {
		if v.Type == "dash-db" {
			resName := strings.Replace(v.Title, " ", "_", -1)
			for _, j := range resources {
				if resName == j["name"] {
					exist = true
					break
				}
			}
			if !exist {
				resource.Uri = v.Url
				resource.Active = true
				resource.Name = resName
				resource.UiId = resName
				resource.LinkType = "frame"
				resource.DisplayName = v.Title
				resource.ParentResource = "analytics"
				resource.AllowedPermissions = "R"
				_, err := s.CreateResource(resource)
				if err != nil {
					s.appService.LoggingClient().Errorf("Error creating dashboard resource: %v", err)
					continue
				} else {
					roleResourcePermission := dto.RoleResourcePermission{
						ResourcesName: resName,
						RoleName:      "DashboardUser",
						Permission:    "RW",
					}
					_, err := s.CreateRoleResourcePermission(&roleResourcePermission)
					if err != nil {
						s.appService.LoggingClient().Errorf("Error adding dashboard permission: %v", err)
						continue
					}
				}
			}
			exist = false
		}
	}
	return nil
}
