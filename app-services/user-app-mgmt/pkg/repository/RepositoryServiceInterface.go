/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package repository

import (
	"hedge/app-services/user-app-mgmt/pkg/dto"
)

type UserMgmtRepository interface {
	InitializeHedgeDB() error
	GetUser(kongUsername string) ([]dto.User, error)
	GetUserEmail(email string) ([]dto.User, error)
	GetInactiveUser(kongUsername string) ([]dto.User, error)
	CreateCredential(username, password string) error
	DeleteCredential(username string) error
	CreateUser(newUser dto.User) error
	UpdateUser(updatedUser *dto.User) error
	CreateUserRole(userRole *dto.UserRole) error
	DeleteUserRoles(userId string) error
	DeleteUser(userName string) error
	GetResource(resourceName string) (dto.Resources, error)
	CreateResource(newResource dto.Resources) error
	DeleteResource(resourceName string) error
	CreateRole(newRole *dto.Role) (*dto.Role, error)
	UpdateRole(updatedRole *dto.Role) (*dto.Role, error)
	GetRoles(roleName string) ([]*dto.Role, error)
	DeleteRole(roleName string) (string, error)
	GetResourcesByUser(userName string) ([]dto.Resources, error)
	GetResourcesUrlByUser(userName string) ([]string, error)
	GetMenuResource(menuName string) ([]dto.Resources, error)
	CreateMenuResources(newMenuResources *[]dto.Resources) error
	UpdateMenuResources(updatedMenus *[]dto.Resources) (*[]dto.Resources, error)
	DeleteMenuResource(menuName string) error
	GetUserPreference(userName string) ([]dto.UserPreference, error)
	GetDefaultUrl(userName string) (*dto.Role, error)
	CreateUserPreference(newUserPreference dto.UserPreference) error
	UpdateUserPreference(updatedUserPreference *dto.UserPreference) error
	DeleteUserPreference(userName string) error
	CreateRoleResourcePermission(roleResourcePermission *dto.RoleResourcePermission) (*dto.RoleResourcePermission, error)
	DeleteRoleResourcePermission(roleName string) error
	GetAllDashboardResources() ([]map[string]string, error)
}
