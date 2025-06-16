/*******************************************************************************
* Contributors: BMC Helix, Inc.
*
* (c) Copyright 2020-2025 BMC Helix, Inc.

* SPDX-License-Identifier: Apache-2.0
*******************************************************************************/

package repository

import (
	"errors"
	"fmt"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
	"hedge/app-services/hedge-user-app-mgmt/pkg/config"
	"hedge/app-services/hedge-user-app-mgmt/pkg/dto"
	"os"
	"os/exec"
	"strings"
	"time"
)

var dbName = "hedge"
var dbInitFile = "res/user-app-mgmt.sql"
var dbDemoMenusFile = "res/demo-menus.sql"
var credentialsPathVariable = "HTPASSWD_FILE_PATH"

type UserManagementRepository struct {
	appService   interfaces.ApplicationService
	dbConnection *gorm.DB
}

func NewUserManagementRepository(appService interfaces.ApplicationService) *UserManagementRepository {
	userManagementRepository := new(UserManagementRepository)
	userManagementRepository.appService = appService
	userManagementRepository.dbConnection = getDbConnection(appService)
	return userManagementRepository
}

func getDbConnection(service interfaces.ApplicationService) *gorm.DB {
	db, err := config.GetDbConnection(service)
	lc := service.LoggingClient()
	if err != nil {
		lc.Errorf("Database connection Error, exiting the service: %v\n", err)
		os.Exit(-1)
	}
	lc.Debugf("Successfully connected!")
	return db
}

// InitializeHedgeDB Run init sql script
func (r *UserManagementRepository) InitializeHedgeDB() error {
	r.appService.LoggingClient().Debugf("RepositoryService.InitializeHedgeDB():: Start")
	db := r.dbConnection
	if db == nil {
		return errors.New("Could not connect to hedge database")
	}
	data, err := os.ReadFile(dbInitFile)
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while reading the DB init file (%s): %v", dbInitFile, err)
		return err
	}

	username, err := r.appService.GetAppSetting("USERNAME")
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while getting app setting: %v", err)
		return err
	}
	userEmail, err := r.appService.GetAppSetting("USER_EMAIL")
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while getting app setting: %v", err)
		return err
	}

	sqldata := strings.Replace(string(data), "USERNAME", username, -1)
	sqldata = strings.Replace(sqldata, "USER_EMAIL", userEmail, -1)
	consulDC, err := r.appService.GetAppSetting("Consul_DC_Name")
	if err != nil {
		r.appService.LoggingClient().Warnf("Error while getting app setting: %v", err)
	}
	consulSvcBaseUrl, err := r.appService.GetAppSetting("CONSUL_SERVICES_URL")
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while getting app setting CONSUL_SERVICES_URL: %v", err)
		sqldata = strings.Replace(sqldata, "CONSUL_SERVICES_URL", "/ui", -1)
		sqldata = strings.Replace(sqldata, "CONSUL_KEYVALUE_URL", "/ui/"+strings.ToLower(consulDC)+"/kv", -1)
	} else {
		consulUrl := consulSvcBaseUrl + "/"
		sqldata = strings.Replace(sqldata, "CONSUL_SERVICES_URL", consulUrl+"/ui", -1)
		// change DC name to lowercase since consul creates datacenters with lowercase names
		sqldata = strings.Replace(sqldata, "CONSUL_KEYVALUE_URL", consulUrl+strings.ToLower(consulDC)+"/kv", -1)
	}

	sqldata = strings.Replace(sqldata, "DASHBOARD_URL", "/hedge/hedge-grafana", -1)
	sqldata = strings.Replace(sqldata, "LINK_TYPE", "frame", -1)

	var dbExists bool
	db.Raw("select exists (select schema_name FROM information_schema.schemata WHERE schema_name = '" + dbName + "');").Scan(&dbExists)
	r.appService.LoggingClient().Infof("Database schema (%s) exists? %b", dbName, dbExists)

	if !dbExists {
		ret := db.Exec(sqldata)
		if ret.Error != nil {
			r.appService.LoggingClient().Errorf("DB Init failed: %v", ret.Error)
			return ret.Error
		}
		r.appService.LoggingClient().Info("DB Initialized successfully")

	} else {
		r.appService.LoggingClient().Infof("DB schema already exists: %s", dbName)
	}
	r.appService.LoggingClient().Infof("RepositoryService.InitializeHedgeDB():: End")
	return nil
}

// GetUser Get User
func (r *UserManagementRepository) GetUser(kongUsername string) ([]dto.User, error) {
	r.appService.LoggingClient().Debugf("RepositoryService.GetUser():: Start")
	db := r.dbConnection
	var result []dto.User
	var err error
	if kongUsername != "all" {
		db.Preload("Roles").Where("Kong_Username = ? AND Status=?", kongUsername, "Active").Find(&result)
	} else {
		// Not sure if we need to get the resources as well in here when it is not there for all above, This is throwing error in logs
		db.Preload("Roles").Preload("Resources").Where("Status = ? AND Sys_User = ?", "Active", 0).Find(&result)
	}
	if db.Error != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the user from DB : %v", db.Error)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("RepositoryService.GetUser():: End")
	return result, nil
}

func (r *UserManagementRepository) GetUserEmail(email string) ([]dto.User, error) {
	r.appService.LoggingClient().Debugf("RepositoryService.GetUserEmail():: Start")
	db := r.dbConnection
	var result []dto.User
	var err error
	db.Preload("Roles").Where("email = ? AND Status=?", email, "Active").Find(&result)
	if db.Error != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the user from DB : %v", db.Error)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("RepositoryService.GetUserEmail():: End")
	return result, nil
}

// GetInactiveUser Gets User that has been set inactive
func (r *UserManagementRepository) GetInactiveUser(kongUsername string) ([]dto.User, error) {
	r.appService.LoggingClient().Debugf("RepositoryService.GetInactiveUser():: Start")
	db := r.dbConnection
	var result []dto.User
	var err error
	db.First(&result, "kong_username = ?", kongUsername)
	if db.Error != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the user from DB : %v", db.Error)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("RepositoryService.GetInactiveUser():: End")
	return result, nil
}

// CreateCredential Creates User Credentials
func (r *UserManagementRepository) CreateCredential(username, password string) error {
	r.appService.LoggingClient().Debugf("user_mgmt.CreateCredential():: Start")
	cp, err := r.appService.GetAppSetting(credentialsPathVariable)

	// Execute htpasswd command to add a new user
	cmd := exec.Command("htpasswd", "-B", "-b", cp, username, password)
	err = cmd.Run()
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while executing command htpasswd, error: %v", err)
		return err
	}

	r.appService.LoggingClient().Debugf("user_mgmt.CreateCredential():: End")
	return nil
}

// DeleteCredential Deletes User Credentials
func (s *UserManagementRepository) DeleteCredential(username string) error {
	s.appService.LoggingClient().Debugf("user_mgmt.DeleteCredentials():: Start")
	cp, err := s.appService.GetAppSetting(credentialsPathVariable)

	// Execute htpasswd command to delete a user
	cmd := exec.Command("htpasswd", "-D", cp, username)
	err = cmd.Run()
	if err != nil {
		return err
	}

	s.appService.LoggingClient().Debugf("user_mgmt.DeleteKongUser():: End")
	return nil
}

// CreateUser Create User
func (r *UserManagementRepository) CreateUser(newUser dto.User) error {
	r.appService.LoggingClient().Debugf("RepositoryService.CreateUser():: Start")
	db := r.dbConnection
	r.appService.LoggingClient().Debugf("Inserting new user with Name: " + newUser.FullName)
	err := db.Omit("modified_by", "modified_on").Create(&newUser).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Creating the user : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("The user has been created successfully!")
	r.appService.LoggingClient().Debugf("RepositoryService.CreateUser():: End")
	return err
}

// UpdateUser Update User
func (r *UserManagementRepository) UpdateUser(updatedUser *dto.User) error {
	r.appService.LoggingClient().Debugf("RepositoryService.UpdateUser():: Start")
	db := r.dbConnection
	var user dto.User
	//Check if the given user is existed in the DB
	err := db.First(&user, "kong_username  = ? AND sys_user = ?", updatedUser.KongUsername, 0).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the user from database with userId: %v ", updatedUser.KongUsername)
		return err
	}
	user.Email = updatedUser.Email
	user.FullName = updatedUser.FullName
	user.Status = updatedUser.Status
	err = db.Model(&user).Where("kong_username = ? AND sys_user = ?", updatedUser.KongUsername, 0).Updates(&user).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Updating the user : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("RepositoryService.UpdateUser():: End")
	return nil
}

// CreateUserRole Create Role
func (r *UserManagementRepository) CreateUserRole(userRole *dto.UserRole) error {
	r.appService.LoggingClient().Debugf("UserManagementRepository.CreateUserRole() :: Start")
	db := r.dbConnection
	err := db.Create(&userRole).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while executing the Database Query", err)
		return err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.CreateUserRole() :: End")
	return nil
}

// DeleteUserRoles Delete Role Resource Permission
func (r *UserManagementRepository) DeleteUserRoles(userId string) error {
	r.appService.LoggingClient().Debugf("RepositoryService.DeleteRoleResourcePermission():: Start")
	db := r.dbConnection
	var userRole dto.UserRole
	userRole.UserKongUsername = userId
	err := db.Delete(&userRole, "user_kong_username = ?", userId).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Deleting the UserRole : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.UserRole() :: End")
	return nil
}

// DeleteUser Delete User
func (r *UserManagementRepository) DeleteUser(userName string) error {
	r.appService.LoggingClient().Debugf("RepositoryService.DeleteUser():: Start")
	db := r.dbConnection
	var user dto.User
	var ur dto.UserRole
	//Check if the given user is existed in the DB
	err := db.First(&user, "kong_username = ? AND sys_user = ?", userName, 0).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the user from database : %v ", err)
		return err
	}
	err = db.Model(&user).Where("kong_username = ? AND sys_user = ?", userName, 0).Update("status", "Inactive").Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Updating the user : %v", err)
		return err
	}
	err = db.Where("user_kong_username = ?", userName).Delete(&ur).Error
	if err != nil {
		return err
	}
	//err = db.Model(user).Association("Resources").Clear()
	//if err != nil {
	//	if err.Error() != "unsupported relations: Resources" {
	//		return err
	//	}
	//}
	r.appService.LoggingClient().Debugf("RepositoryService.UpdateUser():: End")
	return nil
}

// GetResource Get Resource
func (r *UserManagementRepository) GetResource(resourceName string) (dto.Resources, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetResource() :: Start")
	db := r.dbConnection
	var result dto.Resources
	var err error
	if resourceName != "all" {
		err = db.Where("Name = ? AND Active=?", resourceName, true).Find(&result).Error
	} else {
		err = db.Where("Active=?", true).Find(&result).Error
	}
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the resource : %v", err)
		return result, err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetResource() :: End")
	return result, nil
}

// CreateResource Create Resource
func (r *UserManagementRepository) CreateResource(newResource dto.Resources) error {
	r.appService.LoggingClient().Debugf("UserManagementRepository.CreateResource() :: Start")
	db := r.dbConnection
	r.appService.LoggingClient().Debugf("Inserting a new Resource with the Name: " + newResource.Name)
	if len(newResource.ParentResource) > 0 {
		db = db.Omit("Id", "modified_by", "modified_on").Create(&newResource)
	} else {
		db = db.Omit("Id", "parent_resource", "modified_by", "modified_on").Create(&newResource)
	}

	if db.Error != nil {
		err := db.Error
		r.appService.LoggingClient().Errorf("Error while Creating the Resource : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.CreateResource() :: End")
	return nil
}

// DeleteResource Delete Resource
func (r *UserManagementRepository) DeleteResource(resourceName string) error {
	r.appService.LoggingClient().Debugf("UserManagementRepository.DeleteResource() :: Start")
	db := r.dbConnection
	var resource dto.Resources
	//Check if the given resource is existed in the DB
	err := db.First(&resource, "Name = ?", resourceName).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the Resource from database : %v ", err)
		return err
	}
	//err = db.Model(&resource).Association("Role").Clear()
	err = db.Model(&resource).Update("active", false).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Updating the resource : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.DeleteResource() :: End")
	return nil
}

// CreateRole Create Role
func (r *UserManagementRepository) CreateRole(newRole *dto.Role) (*dto.Role, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.CreateRole() :: Start")
	db := r.dbConnection
	err := db.Model(newRole).Omit("modified_by", "modified_on").Association("Resources").Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while associating the Database", err)
		return nil, err
	}
	err = db.Omit("modified_by", "modified_on").Create(newRole).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while executing the Database Query", err)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.CreateRole() :: End")
	return newRole, nil
}

// UpdateRole Update Role
func (r *UserManagementRepository) UpdateRole(updatedRole *dto.Role) (*dto.Role, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.updateRole() :: Start")
	db := r.dbConnection
	err := db.Model(&dto.Role{}).Where("name = ?", updatedRole.Name).Updates(dto.Role{Description: updatedRole.Description, RoleType: updatedRole.RoleType, DefaultResourceName: updatedRole.DefaultResourceName, Resources: updatedRole.Resources}).Error
	//db.Save(&updatedRole)
	if err != nil {
		r.appService.LoggingClient().Debugf("Error while executing the Database Query", err)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.updateRole() :: End")
	return updatedRole, nil
}

// GetRoles Get Role
func (r *UserManagementRepository) GetRoles(roleName string) ([]*dto.Role, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetRoles() :: Start")
	db := r.dbConnection
	var rrp dto.RoleResourcePermission
	var roles []*dto.Role
	var role *dto.Role
	if roleName != "all" {
		db.Preload("Resources").Find(&role, "name = ?", roleName)
		roles = append(roles, role)
		for _, role := range roles {
			if len(role.Resources) > 0 {
				for i, resource := range role.Resources {
					db.Preload("RoleResourcePermission").Where("role_name = ? AND resources_name = ?", role.Name, resource.Name).Find(&rrp)
					role.Resources[i].AllowedPermissions = rrp.Permission
				}
			}
		}
	} else {
		db.Preload("Resources").Find(&roles)
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetRoles() :: End")
	return roles, db.Error
}

// DeleteRole Deletes a Role
func (r *UserManagementRepository) DeleteRole(roleName string) (string, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.DeleteRole() :: Start")
	var role *dto.Role
	db := r.dbConnection
	err := db.First(&role, "name = ?", roleName).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the user from database : %v ", err)
		return "", err
	}
	err = db.Preload("Resources").Delete(&role).Error
	if err != nil {
		r.appService.LoggingClient().Debugf("Error while executing the Preload Query :", err)
		return "", fmt.Errorf("cannot delete role assigned to user")
	}
	role.Name = roleName
	err = db.Model(role).Association("Resources").Clear()
	if err != nil {
		return "", err
	}
	err = db.Raw("DELETE FROM user_role WHERE role_name = ?", roleName).Error
	if err != nil {
		r.appService.LoggingClient().Debugf("Error while deleting from user_role :", err)
		return "", err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.DeleteRole() :: End")
	return "", nil
}

// GetResourcesByUser Gets Resources by User
func (r *UserManagementRepository) GetResourcesByUser(userName string) ([]dto.Resources, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetResourcesByUser() :: Start")
	//populateCache()
	db := r.dbConnection
	var rrp dto.RoleResourcePermission
	var resources []dto.Resources
	var user *dto.User

	db.Preload("Roles").Find(&user, "kong_username = ?", userName)
	roles := user.Roles
	parentResouceNames := make(map[string]struct{}, 0)
	for _, role := range roles {
		db.Preload("Resources").Find(&role, "name = ?", role.Name)

		if role.Resources != nil {
			i := 0
			for _, resource := range role.Resources {
				// Ignore resources that are not active so it doesn't show up in UI menu
				if resource.Active {
					resources = append(resources, resource)
					db.Preload("role_resource_permission").Find(&rrp, "role_name = ? AND resources_name = ?", role.Name, resource.Name)
					resource.AllowedPermissions = rrp.Permission
					if resource.ParentResource != "" {
						parentResouceNames[resource.ParentResource] = struct{}{}
					}
					i += 1
				}
			}
		}
	}

	if resources != nil {
		resources = unique(resources)
	}
	// Get parent resource if it is missing
	missingParentResources := getMissingResources(resources, parentResouceNames)

	for _, missingParentName := range missingParentResources {
		res, err := r.GetMenuResource(missingParentName)
		if err == nil && res != nil && len(res) > 0 {
			resources = append(resources, res[0])
		}
	}

	r.appService.LoggingClient().Debugf("UserManagementRepository.GetResourcesByUser() :: End")
	return resources, db.Error
}

func getMissingResources(resources []dto.Resources, parents map[string]struct{}) []string {
	missingResources := make([]string, 0)
	for parentRes, _ := range parents {
		found := false
		for _, resource := range resources {
			if resource.Name == parentRes {
				found = true
				break
			}
		}
		if !found {
			missingResources = append(missingResources, parentRes)
		}
	}
	return missingResources
}

// GetResourcesUrlByUser Gets Resources by User - For Kong auth plugin
func (r *UserManagementRepository) GetResourcesUrlByUser(userName string) ([]string, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetResourcesUrlByUser() :: Start")
	db := r.dbConnection
	var resources []dto.Resources
	var user *dto.User

	db.Preload("Roles").Find(&user, "kong_username = ?", userName)
	roles := user.Roles
	for _, role := range roles {
		db.Preload("Resources").Find(&role, "name = ?", role.Name)
		var resList = role.Resources
		resources = append(resources, resList...)
	}
	if resources != nil {
		resources = unique(resources)
	}
	var url string
	var resNames []string
	var userRoleUrls []string
	hasDashboardAccess := false
	for _, resource := range resources {
		resNames = append(resNames, resource.Name)
		// For resource with frame ie external resources, allow the configured Uri in resource table itself as the allowed URL
		if resource.LinkType == "frame" && resource.Uri != "" && resource.ParentResource != "" {
			userRoleUrls = append(userRoleUrls, resource.Uri)
			if !hasDashboardAccess && strings.Contains(resource.Uri, "grafana") {
				hasDashboardAccess = true
			}
		}
	}
	rows, err := db.Raw("SELECT DISTINCT ur.url FROM (SELECT DISTINCT u.url, ru.resource_name FROM hedge.urls u "+
		"JOIN hedge.resource_urls ru ON u.name=ru.url_name) ur WHERE ur.resource_name IN ?", resNames).Rows()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		rows.Scan(&url)
		userRoleUrls = append(userRoleUrls, url)
	}
	// For now, just add the public URL to support dashboard static resources
	if hasDashboardAccess {
		// add the following supporting URLs
		userRoleUrls = append(userRoleUrls, "/hedge/hedge-grafana/public")
		// hedge/hedge-grafana/api/dashboards/uid/ECByUMYMk
		// /hedge/hedge-grafana/api/frontend-metrics
		// /hedge/hedge-grafana/api/live/ws HTTP/1.1"
		// hedge/hedge-grafana/api/ds
		// hedge/hedge-grafana/api/playlist
		// Disallow  /hedge/hedge-grafana/api/search
		// Just too many APIs to target specifically, so let Grafana permission mgmt take care of this
		userRoleUrls = append(userRoleUrls, "/hedge/hedge-grafana/api")
	}

	r.appService.LoggingClient().Debugf("UserManagementRepository.GetResourcesUrlByUser() :: End")
	return userRoleUrls, db.Error
}

func unique(resources []dto.Resources) []dto.Resources {
	var unique []dto.Resources
	type key struct{ name, url string }
	m := make(map[key]int)
	for _, v := range resources {
		k := key{v.Name, v.Uri}
		if i, ok := m[k]; ok {
			unique[i] = v
		} else {
			m[k] = len(unique)
			unique = append(unique, v)
		}
	}
	return unique
}

// GetMenuResource Get Menu
func (r *UserManagementRepository) GetMenuResource(menuName string) ([]dto.Resources, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetMenuResource() :: Start")
	db := r.dbConnection
	var resources []dto.Resources
	var err error
	if menuName != "all" {
		err = db.Where("Name = ?", menuName).Find(&resources).Error
	} else {
		err = db.Find(&resources).Error
	}
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the menu : %v", err)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetMenu() :: End")
	return resources, nil
}

// CreateMenuResources Create Menu
func (r *UserManagementRepository) CreateMenuResources(newMenuResources *[]dto.Resources) error {
	r.appService.LoggingClient().Debugf("UserManagementRepository.CreateMenuResources() :: Start")
	db := r.dbConnection
	var err error
	// begin a transaction
	tx := db.Begin()
	for _, newMenu := range *newMenuResources {
		r.appService.LoggingClient().Debugf("Inserting a new Menu with the Name: " + newMenu.Name)
		newMenu.CreatedOn = time.Now().Format(time.RFC3339)
		newMenu.CreatedBy = "admin"
		//if len(newMenu.ParentMenuitemName) > 0 && len(newMenu.ResourceRefName) > 0 {
		if len(newMenu.ParentResource) > 0 {
			//if Parent Resource is not null
			err = tx.Omit("modified_on", "modified_by").Create(&newMenu).Error
		} else {
			//if Parent Resource is NULL
			err = tx.Omit("parent_resource", "modified_on", "modified_by").Create(&newMenu).Error

		}
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while Creating the Menu : %v", err)
			// rollback the transaction in case of error
			tx.Rollback()
			return err
		}
	}
	// commit the transaction
	tx.Commit()
	r.appService.LoggingClient().Debugf("UserManagementRepository.CreateMenu() :: End")
	return nil
}

// UpdateMenuResources Update Menu
func (r *UserManagementRepository) UpdateMenuResources(updatedMenus *[]dto.Resources) (*[]dto.Resources, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.UpdateMenuResources() :: Start")
	db := r.dbConnection
	// begin a transaction
	tx := db.Begin()
	var menus []dto.Resources
	for _, updatedMenu := range *updatedMenus {
		var menu dto.Resources
		//Check if the given menu is existed in the DB
		err := db.First(&menu, "Name = ?", updatedMenu.Name).Error
		updatedMenu.ModifiedOn = time.Now().Format(time.RFC3339)
		updatedMenu.ModifiedBy = "admin"
		menu.DisplayName = updatedMenu.DisplayName
		if len(updatedMenu.Uri) > 0 {
			menu.Uri = updatedMenu.Uri
		}
		if len(updatedMenu.LinkType) > 0 {
			menu.LinkType = updatedMenu.LinkType
		}
		if len(updatedMenu.ParentResource) > 0 {
			menu.ParentResource = updatedMenu.ParentResource
		}
		menu.ModifiedBy = updatedMenu.ModifiedBy
		menu.ModifiedOn = updatedMenu.ModifiedOn

		if err != nil {
			//if menus are not available in the DB, create new entries in DB
			updatedMenu.CreatedOn = time.Now().Format(time.RFC3339)
			updatedMenu.CreatedBy = "admin"
			menu.CreatedOn = updatedMenu.CreatedOn
			menu.CreatedBy = updatedMenu.CreatedBy
			menu.Name = updatedMenu.Name
			if len(updatedMenu.ParentResource) > 0 {
				//if both ParentMenuItemName and ResourceRefName are not null
				err = tx.Omit("modified_on", "modified_by").Create(&menu).Error
			} else {
				if len(updatedMenu.ParentResource) < 1 {
					//if both ParentMenuItemName and ResourceRefName are NULL
					err = tx.Omit("parent_resource", "modified_on", "modified_by").Create(&menu).Error
				}
				/*if len(updatedMenu.ParentMenuitemName) > 0 && len(updatedMenu.ResourceRefName) < 1 {
					//if ParentMenuItemName is not null and ResourceRefName is NULL
					err = tx.Omit("resource_ref_name", "modified_on", "modified_by").Create(&menu).Error
				}
				if len(updatedMenu.ParentMenuitemName) < 1 && len(updatedMenu.ResourceRefName) > 0 {
					//if ParentMenuItemName is null and ResourceRefName is not NULL
					err = tx.Omit("parent_menuitem_name", "modified_on", "modified_by").Create(&menu).Error
				}*/
			}
			menus = append(menus, menu)
		} else {
			//if menus are available in the DB, update the entries in DB

			err := tx.Where("name = ?", updatedMenu.Name).Updates(&menu).Error
			//tx.Save(menu)
			if err != nil {
				r.appService.LoggingClient().Debugf("Error while executing the Database Query", err)
				// Rollback the transaction
				tx.Rollback()
				return nil, err
			}

			menus = append(menus, menu)

			r.deleteSubMenuResources(updatedMenus, db)
		}

		//db.Save(&updatedMenu).Association("Resources").Replace([]dto.Resources{{Name: updatedMenu.Resources.}})
		if err != nil {
			r.appService.LoggingClient().Debugf("Error while executing the Database Query", err)
			// Rollback the transaction
			tx.Rollback()
			return nil, err
		}
	}

	// commit the transaction
	tx.Commit()
	r.appService.LoggingClient().Debugf("UserManagementRepository.UpdateMenuResources() :: End")
	return &menus, nil
}

func (r *UserManagementRepository) deleteSubMenuResources(inputMenuItems *[]dto.Resources, db *gorm.DB) {
	//check if the inputMenuItems contains all database Resources.
	//if No, delete the additional database entries for Resources.
	var parentMenuItem string
	var dbSubmenuItems []dto.Resources
	var deletedSubmenuItem dto.Resources
	for _, inputMenuItem := range *inputMenuItems {
		if len(inputMenuItem.ParentResource) < 1 {
			parentMenuItem = inputMenuItem.Name
			break
		}
	}
	if len(parentMenuItem) > 0 {
		var submenuItemsToDelete []dto.Resources
		// Get all subMenuItems from DB
		db.Where("parent_resource = ?", parentMenuItem).Find(&dbSubmenuItems)
		//iterate over the Database Resources
		for _, dbSubmenuItem := range dbSubmenuItems {
			for _, inputMenuItem := range *inputMenuItems {
				if inputMenuItem.Name == dbSubmenuItem.Name || len(inputMenuItem.ParentResource) < 1 {
					continue
				} else {
					submenuItemsToDelete = append(submenuItemsToDelete, dbSubmenuItem)
				}
			}
		}
		for _, submenuItemToDelete := range submenuItemsToDelete {
			err := db.Delete(&deletedSubmenuItem, "name = ?", submenuItemToDelete.Name).Error
			//db.Model(menu).Association("Roles").Clear()
			//err = db.Model(&menu).Where("Name = ?", menuName).Update("active", false).Error
			if err != nil {
				r.appService.LoggingClient().Errorf("Error while Deleting the menu : %v", err)
				//return err
			}
		}
	}
}

// DeleteMenuResource Delete Menu
func (r *UserManagementRepository) DeleteMenuResource(menuName string) error {
	r.appService.LoggingClient().Debugf("UserManagementRepository.DeleteMenuResource() :: Start")
	db := r.dbConnection
	var menuResource dto.Resources
	//Check if the given menu is existed in the DB
	err := db.First(&menuResource, "Name = ?", menuName).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the Menu from database : %v ", err)
		return err
	}
	err = db.Preload("Resource").Delete(&menuResource, "name = ?", menuName).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Deleting the menu : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.DeleteMenuResource() :: End")
	return nil
}

/*func (r *UserManagementRepository) GetMenusByUser(userName string) ([]dto.Menu, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetMenusByUser() :: Start")
	db := r.dbConnection
	var menus []dto.Menu
	var user *dto.User
	var role *dto.Role
	db.Preload("Roles").Find(&user, "name = ?", userName)
	roles := user.Roles
	for i := range roles {
		db.Preload("Menu").Find(&role, "name = ?", roles[i].Name)
		var menuList []dto.Menu = role.Menu
		menus = append(menus, menuList...)
	}
if menus != nil {
		menus = uniqueMenu(menus)
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetMenusByUser() :: End")
	return menus, db.Error
}

func uniqueMenu(menus []dto.Menu) []dto.Menu {
	var unique []dto.Menu
	type key struct{ name, url string }
	m := make(map[key]int)
	for _, v := range menus {
		k := key{v.Name, v.LinkUri}
		if i, ok := m[k]; ok {
			unique[i] = v
		} else {
			m[k] = len(unique)
			unique = append(unique, v)
		}
	}
	return unique
}*/

// GetUserPreference Get UserPreference
func (r *UserManagementRepository) GetUserPreference(userName string) ([]dto.UserPreference, error) {
	r.appService.LoggingClient().Debugf("RepositoryService.GetUserPreference():: Start")
	db := r.dbConnection
	var result []dto.UserPreference
	var err error
	if len(userName) > 0 {
		db.Where("kong_username = ? ", userName).Find(&result)
	}

	if db.Error != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the userPreference from DB : %v", db.Error)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("RepositoryService.GetUserPreference():: End")
	return result, nil
}

// GetDefaultUrl Get HomePage based on Roles
func (r *UserManagementRepository) GetDefaultUrl(userName string) (*dto.Role, error) {
	r.appService.LoggingClient().Debugf("RepositoryService.GetDefaultUrl():: Start")
	db := r.dbConnection
	var result *dto.Role
	var err error
	if len(userName) > 0 {
		db.Raw("SELECT r.default_page, r.default_page_type FROM hedge.user_roles ur JOIN hedge.role r ON r.name=ur.role_name WHERE ur.user_kong_username=? LIMIT 1;", userName).Find(&result)
	}

	if db.Error != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the DefaultUrl from DB : %v", db.Error)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("RepositoryService.GetDefaultUrl():: End")
	return result, nil
}

// CreateUserPreference Create UserPreference
func (r *UserManagementRepository) CreateUserPreference(newUserPreference dto.UserPreference) error {
	r.appService.LoggingClient().Debugf("RepositoryService.CreateUserPreference():: Start")
	db := r.dbConnection
	r.appService.LoggingClient().Debugf("Inserting new user with Name: " + newUserPreference.KongUsername)
	err := db.Omit("modified_by", "modified_on").Create(&newUserPreference).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Creating the user : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("The userPreference has been created successfully!")
	r.appService.LoggingClient().Debugf("RepositoryService.CreateUserPreference():: End")
	return err
}

// UpdateUserPreference Update UserPreference
func (r *UserManagementRepository) UpdateUserPreference(updatedUserPreference *dto.UserPreference) error {
	r.appService.LoggingClient().Debugf("RepositoryService.UpdateUserPreference():: Start")
	db := r.dbConnection
	var userPreference dto.UserPreference
	//Check if the given userPreference is existed in the DB
	err := db.First(&userPreference, "kong_username = ?", updatedUserPreference.KongUsername).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the userPreference from database with userName: %v ", updatedUserPreference.KongUsername)
		return err
	}
	userPreference.ResourceName = updatedUserPreference.ResourceName
	err = db.Model(&userPreference).Where("kong_username = ?", updatedUserPreference.KongUsername).Updates(&userPreference).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Updating the userPreference : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("RepositoryService.UpdateUserPreference():: End")
	return nil
}

// DeleteUserPreference Delete UserPreference
func (r *UserManagementRepository) DeleteUserPreference(userName string) error {
	r.appService.LoggingClient().Debugf("RepositoryService.DeleteUserPreference():: Start")
	db := r.dbConnection
	var userPreference dto.UserPreference
	//Check if the given userPreference exist in the DB
	err := db.First(&userPreference, "kong_username = ?", userName).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while fetching the userPreference from database : %v ", err)
		return err
	}
	err = db.Where("kong_username = ?", userName).Delete(&userPreference).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Updating the userPreference : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("RepositoryService.UpdateUserPreference():: End")
	return nil
}

// CreateRoleResourcePermission Create new RoleResourcePermission
func (r *UserManagementRepository) CreateRoleResourcePermission(roleResourcePermission *dto.RoleResourcePermission) (*dto.RoleResourcePermission, error) {
	r.appService.LoggingClient().Debugf("RepositoryService.CreateRoleResourcePermission():: Start")
	db := r.dbConnection
	r.appService.LoggingClient().Debugf("Inserting new Role-Resource: " + roleResourcePermission.RoleName)
	err := db.Create(&roleResourcePermission).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Inserting Role-Resource : %v", err)
		return nil, err
	}
	r.appService.LoggingClient().Debugf("The RoleResourcePermission has been created successfully!")
	r.appService.LoggingClient().Debugf("RepositoryService.CreateRoleResourcePermission():: End")
	return roleResourcePermission, nil
}

// UpdateRoleResourcePermission Update RoleResourcePermission
//func (r *UserManagementRepository) UpdateRoleResourcePermission(resource *dto.RoleResourcePermission) (*dto.RoleResourcePermission, error) {
//	r.appService.LoggingClient().Debugf("RepositoryService.CreateRoleResourcePermission():: Start")
//	db := r.dbConnection
//	result := db.First(&resource, "role_name = ? AND resources_name = ?", resource.RoleName, resource.ResourcesName)
//	if result.RowsAffected == 0 {
//		r.appService.LoggingClient().Errorf("Error while fetching Role-Resource : %v", result.Error)
//		return nil, result.Error
//	}
//	r.appService.LoggingClient().Debugf("Updating new Role-Resource: " + resource.RoleName + "-" + resource.ResourcesName)
//	err := db.Model(&resource).Select("permission").Where("role_name = ? AND resources_name = ?", resource.RoleName, resource.ResourcesName).Updates(&resource).Error
//	if err != nil {
//		r.appService.LoggingClient().Errorf("Error while Updating Role-Resource : %v", err)
//		return nil, err
//	}
//	r.appService.LoggingClient().Debugf("RepositoryService.CreateRoleResourcePermission():: End")
//	return resource, nil
//}

// DeleteRoleResourcePermission Delete Role Resource Permission
func (r *UserManagementRepository) DeleteRoleResourcePermission(roleName string) error {
	r.appService.LoggingClient().Debugf("RepositoryService.DeleteRoleResourcePermission():: Start")
	db := r.dbConnection
	var roleResourcePermission dto.RoleResourcePermission
	//Check if the given menu is existed in the DB
	//err := db.First(&roleResourcePermission, "role_name = ? AND resources_name = ?", roleName, resourceName).Error
	//if err != nil {
	//	r.appService.LoggingClient().Errorf("Error while fetching the user from database : %v ", err)
	//	return err
	//}
	err := db.Delete(&roleResourcePermission, "role_name = ?", roleName).Error
	if err != nil {
		r.appService.LoggingClient().Errorf("Error while Deleting the DeleteRoleResourcePermission : %v", err)
		return err
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.DeleteRoleResourcePermission() :: End")
	return nil
}

// GetAllDashboardResources Get all Grafana Dashboards
func (r *UserManagementRepository) GetAllDashboardResources() ([]map[string]string, error) {
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetAllDashboardResources() :: Start")
	db := r.dbConnection
	var name string
	var uri string
	var resources []map[string]string
	var err error
	rows, err := db.Raw("SELECT name, uri FROM hedge.resources WHERE uri LIKE '%/d/%'").Rows()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		rows.Scan(&name, &uri)
		res := map[string]string{"name": name, "uri": uri}
		resources = append(resources, res)
	}
	r.appService.LoggingClient().Debugf("UserManagementRepository.GetAllDashboardResources() :: End")
	return resources, nil
}
