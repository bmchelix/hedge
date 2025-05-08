/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package service

import (
	"hedge/app-services/user-app-mgmt/pkg/dto"
	"hedge/app-services/user-app-mgmt/pkg/repository"
	"hedge/common/client"
	mrepo "hedge/mocks/hedge/app-services/user-app-mgmt/pkg/repository"
	"hedge/mocks/hedge/common/infrastructure/interfaces/utils"
	"errors"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/stretchr/testify/mock"
	"reflect"
	"testing"
	"time"
)

type fields struct {
	appService               interfaces.ApplicationService
	dbConfig                 map[string]string
	userManagementRepository *repository.UserMgmtRepository
	dashboardURL             string
}

var mumr = mrepo.MockUserMgmtRepository{}
var ums *UserManagementService
var ums2 *UserManagementService
var umr repository.UserMgmtRepository
var u *utils.HedgeMockUtils
var alter *utils.HedgeMockUtils
var httpMock *utils.MockClient
var dbmap map[string]string
var flds fields
var flds2 fields

func init() {
	u = utils.NewApplicationServiceMock(map[string]string{})
	alter = utils.NewApplicationServiceMock(map[string]string{"Kong_Server": "localhost"})
	dbmap = u.AppSettings
	UserManagementRepositoryImpl = &mumr
	ums = NewUserManagementService(u.AppService, dbmap)
	ums2 = NewUserManagementService(u.AppService, map[string]string{"Grafana_Server": "localhost"})
	ums.appService = u.AppService
	httpMock = utils.NewMockClient()
	client.Client = httpMock
	httpMock.RegisterExternalMockRestCall("localhost/consumers", "POST", nil, 400, errors.New("error"))
	httpMock.RegisterExternalMockRestCall("localhost/consumers/name/basic-auth", "POST", nil, 400, errors.New("error"))
	httpMock.RegisterExternalMockRestCall("/consumers/name/basic-auth", "POST", nil)
	httpMock.RegisterExternalMockRestCall("/consumers", "POST", nil)
	httpMock.RegisterExternalMockRestCall("/consumers/User", "DELETE", nil)
	httpMock.RegisterExternalMockRestCall("localhost/consumers/User", "DELETE", nil, 400, errors.New("error"))
	httpMock.RegisterExternalMockRestCall("/fail", "GET", nil, 400, errors.New("error"))
	httpMock.RegisterExternalMockRestCall("/dash", "GET", []map[string]string{{"Type": "dash-db"}, {"Grafana_Admin": "admin"}}, 200, nil)
	flds = fields{
		appService:               u.AppService,
		dbConfig:                 dbmap,
		userManagementRepository: &UserManagementRepositoryImpl,
		dashboardURL:             "/dash",
	}
	flds2 = fields{
		appService:               alter.AppService,
		dbConfig:                 dbmap,
		userManagementRepository: &UserManagementRepositoryImpl,
		dashboardURL:             "/fail",
	}
}

func TestNewUserManagementService(t *testing.T) {
	type args struct {
		appService interfaces.ApplicationService
		dbConfig   map[string]string
	}
	arg := args{
		appService: u.AppService,
		dbConfig:   dbmap,
	}
	arg2 := args{
		appService: u.AppService,
		dbConfig:   map[string]string{"Grafana_Server": "localhost"},
	}
	tests := []struct {
		name string
		args args
		want *UserManagementService
	}{
		{"NewUserMgmtServicePassed", arg, ums},
		{"NewUserMgmtServicePassed2", arg2, ums2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewUserManagementService(tt.args.appService, tt.args.dbConfig); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewUserManagementService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_CreateCredential(t *testing.T) {
	type args struct {
		credential LoginInputs
	}
	cred := LoginInputs{
		Username: "name",
		Password: "123",
	}
	arg := args{credential: cred}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Create credential - Passed", flds, arg, false},
		{"Create credential - Failed", flds2, arg, true},
	}

	mumr.On("CreateCredential", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Once()
	mumr.On("CreateCredential", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("failed creating credentials")).Once()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.CreateCredential(tt.args.credential); (err != nil) != tt.wantErr {
				t.Errorf("CreateCredential() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_CreateMenuResources(t *testing.T) {
	type args struct {
		newMenuItemRes dto.Resource
	}
	resource := dto.Resource{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		AllowedPermissions: "",
		Tooltip:            "",
		IsTerminalItem:     false,
		DisplaySequence:    0,
		LinkDetails:        dto.LinkDetails{},
		SubResources:       nil,
	}
	resource2 := dto.Resource{
		Id:                 0,
		Name:               "Test",
		DisplayName:        "Test",
		AllowedPermissions: "",
		Tooltip:            "",
		IsTerminalItem:     false,
		DisplaySequence:    0,
		LinkDetails:        dto.LinkDetails{},
		SubResources:       nil,
	}
	resources := dto.Resources{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          "",
		CreatedBy:          "",
		ModifiedOn:         "",
		ModifiedBy:         "",
	}
	resources2 := dto.Resources{
		Id:                 0,
		Name:               "Test",
		DisplayName:        "Test",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          "",
		CreatedBy:          "",
		ModifiedOn:         "",
		ModifiedBy:         "",
	}
	arg := args{newMenuItemRes: resource}
	arg2 := args{newMenuItemRes: resource2}
	mumr.On("CreateMenuResources", &[]dto.Resources{resources}).Return(nil)
	mumr.On("CreateMenuResources", &[]dto.Resources{resources2}).Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Create menu resource - Passed", flds, arg, false},
		{"Create menu resource - Failed", flds2, arg2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.CreateMenuResources(tt.args.newMenuItemRes); (err != nil) != tt.wantErr {
				t.Errorf("CreateMenuResources() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_CreateResource(t *testing.T) {
	type args struct {
		newResource *dto.Resources
	}
	resources := &dto.Resources{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          time.Now().Format(time.RFC3339),
		CreatedBy:          "admin",
		ModifiedOn:         time.Now().Format(time.RFC3339),
		ModifiedBy:         "admin",
	}
	resources2 := &dto.Resources{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          "",
		CreatedBy:          "",
		ModifiedOn:         "",
		ModifiedBy:         "",
	}
	mumr.On("CreateResource", *resources).Return(nil)
	//mumr.On("CreateResource", mock.Anything).Return(errors.New("error"))
	arg := args{newResource: resources}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.Resources
		wantErr bool
	}{
		{"Create resource - Passed", flds, arg, resources2, false},
		//{"Create resource - Failed", flds2, args{newResource: resources}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.CreateResource(*tt.args.newResource)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateResource() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_CreateRole(t *testing.T) {
	type args struct {
		newRole *dto.Role
	}
	role := &dto.Role{
		Name:                "",
		Description:         "",
		RoleType:            "",
		DefaultResourceName: "",
		CreatedOn:           "",
		CreatedBy:           "",
		ModifiedOn:          "",
		ModifiedBy:          "",
		Resources: []dto.Resources{{
			Id:                 0,
			Name:               "",
			DisplayName:        "",
			Uri:                "",
			LinkType:           "",
			UiId:               "",
			Active:             false,
			ParentResource:     "",
			AllowedPermissions: "",
			CreatedOn:          "",
			CreatedBy:          "",
			ModifiedOn:         "",
			ModifiedBy:         "",
		}},
	}
	role2 := &dto.Role{
		Name:                "Test",
		Description:         "",
		RoleType:            "",
		DefaultResourceName: "",
		CreatedOn:           "",
		CreatedBy:           "",
		ModifiedOn:          "",
		ModifiedBy:          "",
		Resources: []dto.Resources{{
			Id:                 0,
			Name:               "",
			DisplayName:        "",
			Uri:                "",
			LinkType:           "",
			UiId:               "",
			Active:             false,
			ParentResource:     "",
			AllowedPermissions: "",
			CreatedOn:          "",
			CreatedBy:          "",
			ModifiedOn:         "",
			ModifiedBy:         "",
		}},
	}
	arg := args{newRole: role}
	mumr.On("CreateRole", role).Return(role, nil)
	mumr.On("CreateRole", role2).Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.Role
		wantErr bool
	}{
		{"Create role - Passed", flds, arg, role, false},
		{"Create role - Failed", flds, args{newRole: role2}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.CreateRole(tt.args.newRole)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_CreateRoleResourcePermission(t *testing.T) {
	type args struct {
		roleResource *dto.RoleResourcePermission
	}
	perm := &dto.RoleResourcePermission{
		RoleName:      "",
		ResourcesName: "",
		Permission:    "",
	}
	perm2 := &dto.RoleResourcePermission{
		RoleName:      "1",
		ResourcesName: "2",
		Permission:    "3",
	}
	arg := args{roleResource: perm}
	mumr.On("CreateRoleResourcePermission", perm).Return(perm, nil)
	mumr.On("CreateRoleResourcePermission", perm2).Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.RoleResourcePermission
		wantErr bool
	}{
		{"Create role permission - Passed", flds, arg, perm, false},
		{"Create role permission - Failed", flds, args{roleResource: perm2}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.CreateRoleResourcePermission(tt.args.roleResource)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateRoleResourcePermission() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateRoleResourcePermission() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_CreateUser(t *testing.T) {
	type args struct {
		newUser dto.User
	}
	user := dto.User{
		FullName:       "",
		Email:          "",
		KongUsername:   "Name",
		ExternalUserId: "",
		Status:         "",
		CreatedOn:      time.Now().Format(time.RFC3339),
		CreatedBy:      "admin",
		ModifiedOn:     "",
		ModifiedBy:     "",
		Roles:          nil,
	}
	user2 := dto.User{
		FullName:       "",
		Email:          "",
		KongUsername:   "",
		ExternalUserId: "",
		Status:         "",
		CreatedOn:      time.Now().Format(time.RFC3339),
		CreatedBy:      "admin",
		ModifiedOn:     "",
		ModifiedBy:     "",
		Roles:          nil,
	}
	userarr := []dto.User{}
	userarr = append(userarr, user)
	arg := args{user}
	arg2 := args{user2}
	mumr.On("GetUser", user.KongUsername).Return(userarr, nil)
	mumr.On("CreateUser", user).Return(nil)
	mumr.On("CreateUser", mock.Anything).Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.User
		wantErr bool
	}{
		{"Create user - Passed", flds, arg, &user, false},
		{"Create user - Failed", flds2, arg2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.CreateUser(tt.args.newUser)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_CreateUserPreference(t *testing.T) {
	type args struct {
		newUserPreference dto.UserPreference
	}
	userPref := dto.UserPreference{
		KongUsername: "Name",
		ResourceName: "",
		CreatedOn:    time.Now().Format(time.RFC3339),
		CreatedBy:    "admin",
		ModifiedOn:   "",
		ModifiedBy:   "",
	}
	userPrefArr := []dto.UserPreference{}
	userPrefArr = append(userPrefArr, userPref)
	arg := args{userPref}
	mumr.On("CreateUserPreference", userPref).Return(nil)
	mumr.On("GetUserPreference", userPref.KongUsername).Return(userPrefArr, nil)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.UserPreference
		wantErr bool
	}{
		{"Create user pref - Passed", flds, arg, &userPref, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.CreateUserPreference(tt.args.newUserPreference)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUserPreference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateUserPreference() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_CreateUserRole(t *testing.T) {
	type args struct {
		newUserRole *dto.UserRole
		roles       Roles
	}
	usrrole := &dto.UserRole{
		UserKongUsername: "Name",
		RoleName:         "Role",
	}
	arg := args{
		newUserRole: usrrole,
		roles:       Roles{},
	}
	arg2 := args{
		newUserRole: usrrole,
		roles:       Roles{Role: []string{"Role"}},
	}
	mumr.On("CreateUserRole", mock.Anything).Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.UserRole
		wantErr bool
	}{
		{"Create user role - Passed", flds, arg, usrrole, false},
		{"Create user role - Failed", flds, arg2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.CreateUserRole(tt.args.newUserRole, tt.args.roles)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateUserRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateUserRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_DeleteMenuResource(t *testing.T) {
	type args struct {
		menuName string
	}
	arg := args{menuName: "Menu"}
	mumr.On("DeleteMenuResource", "Menu").Return(nil)
	mumr.On("DeleteMenuResource", "Test").Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Delete menu - Passed", flds, arg, false},
		{"Delete menu - Failed", flds, args{menuName: "Test"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.DeleteMenuResource(tt.args.menuName); (err != nil) != tt.wantErr {
				t.Errorf("DeleteMenuResource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_DeleteResource(t *testing.T) {
	type args struct {
		resourceName string
	}
	arg := args{resourceName: "Resource"}
	mumr.On("DeleteResource", "Resource").Return(nil)
	mumr.On("DeleteResource", "Test").Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Delete resource - Passed", flds, arg, false},
		{"Delete resource - Failed", flds, args{resourceName: "Test"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.DeleteResource(tt.args.resourceName); (err != nil) != tt.wantErr {
				t.Errorf("DeleteResource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_DeleteRole(t *testing.T) {
	type args struct {
		roleName string
	}
	arg := args{roleName: "Role"}
	mumr.On("DeleteRole", "Role").Return("", nil)
	mumr.On("DeleteRole", "Test").Return("", errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{"Delete role - Passed", flds, arg, "OK", false},
		{"Delete role - Failed", flds, args{roleName: "Test"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.DeleteRole(tt.args.roleName)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DeleteRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_DeleteRoleResourcePermission(t *testing.T) {
	type args struct {
		roleName string
	}
	arg := args{roleName: "Role"}
	mumr.On("DeleteRoleResourcePermission", "Role").Return(nil)
	mumr.On("DeleteRoleResourcePermission", "Test").Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Delete resource perm - Passed", flds, arg, false},
		{"Delete resource perm - Failed", flds, args{roleName: "Test"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.DeleteRoleResourcePermission(tt.args.roleName); (err != nil) != tt.wantErr {
				t.Errorf("DeleteRoleResourcePermission() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_DeleteUser(t *testing.T) {
	type args struct {
		userName string
	}
	arg := args{userName: "User"}
	mumr.On("DeleteUser", "User").Return(nil)
	mumr.On("DeleteUser", mock.Anything).Return(errors.New("error"))
	mumr.On("DeleteCredential", mock.AnythingOfType("string")).Return(nil).Once()
	mumr.On("DeleteCredential", mock.AnythingOfType("string")).Return(errors.New("failed creating credentials")).Once()

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Delete user - Passed", flds, arg, false},
		{"Delete user - Failed", flds, args{userName: "Test"}, true},
		{"Delete user - Failed2", flds2, arg, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.DeleteUser(tt.args.userName); (err != nil) != tt.wantErr {
				t.Errorf("DeleteUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_DeleteUserPreference(t *testing.T) {
	type args struct {
		userName string
	}
	arg := args{userName: "User"}
	mumr.On("DeleteUserPreference", "User").Return(nil)
	mumr.On("DeleteUserPreference", "Test").Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Delete User pref - Passed", flds, arg, false},
		{"Delete User pref - Failed", flds, args{userName: "Test"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.DeleteUserPreference(tt.args.userName); (err != nil) != tt.wantErr {
				t.Errorf("DeleteUserPreference() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_DeleteUserRoles(t *testing.T) {
	type args struct {
		userId string
	}
	arg := args{userId: "User"}
	mumr.On("DeleteUserRoles", "User").Return(nil)
	mumr.On("DeleteUserRoles", "Test").Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"DeleteUserRoles - Passed", flds, arg, false},
		{"DeleteUserRoles - Failed", flds, args{userId: "Test"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.DeleteUserRoles(tt.args.userId); (err != nil) != tt.wantErr {
				t.Errorf("DeleteUserRoles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_GetInactiveUser(t *testing.T) {
	type args struct {
		userName string
	}
	user := dto.User{
		FullName:       "",
		Email:          "",
		KongUsername:   "",
		ExternalUserId: "",
		Status:         "",
		CreatedOn:      "",
		CreatedBy:      "",
		ModifiedOn:     "",
		ModifiedBy:     "",
		Roles:          nil,
	}
	users := []dto.User{}
	users = append(users, user)
	arg := args{userName: "User"}
	arg2 := args{userName: "Test"}
	mumr.On("GetInactiveUser", "User").Return(users, nil)
	mumr.On("GetInactiveUser", "Test").Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []dto.User
		wantErr bool
	}{
		{"Get inactive users - Passed", flds, arg, users, false},
		{"Get inactive users - Failed", flds, arg2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetInactiveUser(tt.args.userName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetInactiveUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetInactiveUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetMenuResource(t *testing.T) {
	type args struct {
		menuName           string
		fetchResourcesOnly bool
	}
	arg := args{
		menuName:           "Menu",
		fetchResourcesOnly: true,
	}
	resourceArr := []dto.Resource{}
	resourceArr2 := []dto.Resource{{
		Id:                 0,
		Name:               "Menu",
		DisplayName:        "Menu",
		AllowedPermissions: "RW",
		Tooltip:            "",
		IsTerminalItem:     false,
		DisplaySequence:    0,
		LinkDetails:        dto.LinkDetails{},
		SubResources:       nil,
	}}
	resourcesArr := []dto.Resources{dto.Resources{
		Id:                 0,
		Name:               "Menu",
		DisplayName:        "Menu",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          "",
		CreatedBy:          "",
		ModifiedOn:         "",
		ModifiedBy:         "",
	}}
	mumr.On("GetMenuResource", "Menu").Return(resourcesArr, nil)
	mumr.On("GetMenuResource", "Test").Return(nil, errors.New("error"))
	mumr.On("GetMenuResource", "Test2").Return(resourcesArr, nil)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []dto.Resource
		wantErr bool
	}{
		{"Get menu resource - Passed", flds, arg, resourceArr, false},
		{"Get menu resource - Failed", flds, args{menuName: "Test", fetchResourcesOnly: true}, nil, true},
		{"Get menu resource - Failed2", flds, args{menuName: "Test2", fetchResourcesOnly: false}, resourceArr2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetMenuResource(tt.args.menuName, tt.args.fetchResourcesOnly)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMenuResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMenuResource() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetResource(t *testing.T) {
	type args struct {
		resourceName string
	}
	resourses := dto.Resources{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          "",
		CreatedBy:          "",
		ModifiedOn:         "",
		ModifiedBy:         "",
	}
	mumr.On("GetResource", "Resource").Return(resourses, nil)
	mumr.On("GetResource", mock.Anything).Return(resourses, errors.New("error"))
	arg := args{resourceName: "Resource"}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    dto.Resources
		wantErr bool
	}{
		{"Get resource - Passed", flds, arg, resourses, false},
		{"Get resource - Failed", flds, args{resourceName: "Test"}, resourses, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetResource(tt.args.resourceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetResource() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetResourcesByUser(t *testing.T) {
	type args struct {
		userName string
	}
	resources := []dto.Resources{}
	arg := args{userName: "User"}
	mumr.On("GetResourcesByUser", "User").Return(resources, nil)
	mumr.On("GetResourcesByUser", "Test").Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []dto.Resources
		wantErr bool
	}{
		{"GetResourcesByUser - Passed", flds, arg, resources, false},
		{"GetResourcesByUser - Failed", flds, args{userName: "Test"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetResourcesByUser(tt.args.userName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetResourcesByUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetResourcesByUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetResourcesUrlByUser(t *testing.T) {
	type args struct {
		userName string
	}
	arg := args{userName: "User"}
	mumr.On("GetResourcesUrlByUser", "User").Return([]string{}, nil)
	mumr.On("GetResourcesUrlByUser", "Test").Return(nil, errors.New("erro"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{"GetResourcesUrlByUser - Passed", flds, arg, []string{}, false},
		{"GetResourcesUrlByUser - Failed", flds, args{userName: "Test"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetResourcesUrlByUser(tt.args.userName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetResourcesUrlByUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetResourcesUrlByUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetRole(t *testing.T) {
	type args struct {
		roleName string
	}
	role := dto.Role{
		Name:                "",
		Description:         "",
		RoleType:            "",
		DefaultResourceName: "",
		CreatedOn:           "",
		CreatedBy:           "",
		ModifiedOn:          "",
		ModifiedBy:          "",
		Resources:           nil,
	}
	roles := []*dto.Role{&role}
	arg := args{roleName: "Role"}
	mumr.On("GetRoles", "Role").Return(roles, nil)
	mumr.On("GetRoles", "Test").Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*dto.Role
		wantErr bool
	}{
		{"Get roles - Passed", flds, arg, roles, false},
		{"Get roles - Failed", flds, args{roleName: "Test"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetRole(tt.args.roleName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetUser(t *testing.T) {
	type args struct {
		userName string
	}
	arg := args{userName: "User"}
	arg2 := args{userName: "Test"}
	user := dto.User{
		FullName:       "",
		Email:          "",
		KongUsername:   "",
		ExternalUserId: "",
		Status:         "",
		CreatedOn:      "",
		CreatedBy:      "",
		ModifiedOn:     "",
		ModifiedBy:     "",
		Roles:          nil,
	}
	users := []dto.User{}
	users = append(users, user)
	mumr.On("GetUser", "User").Return(users, nil)
	mumr.On("GetUser", "Test").Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []dto.User
		wantErr bool
	}{
		{"Get user - Passed", flds, arg, users, false},
		{"Get user - Failed", flds, arg2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetUser(tt.args.userName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetUserContextLandingPage(t *testing.T) {
	type args struct {
		userName string
	}
	arg := args{userName: "User"}
	link := dto.LinkDetails{
		IsExternalUrl: false,
		LinkUri:       "",
		LinkType:      "",
		LinkId:        "",
	}
	userPref := dto.UserPreference{
		KongUsername: "",
		ResourceName: "",
		CreatedOn:    "",
		CreatedBy:    "",
		ModifiedOn:   "",
		ModifiedBy:   "",
	}
	userPrefs := []dto.UserPreference{}
	userPrefs = append(userPrefs, userPref)
	resources := dto.Resources{
		Id:                 0,
		Name:               "Resource",
		DisplayName:        "",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          "",
		CreatedBy:          "",
		ModifiedOn:         "",
		ModifiedBy:         "",
	}
	mumr.On("GetUserPreference", "User").Return(userPrefs, nil)
	mumr.On("GetResource", "").Return(resources, nil)
	//mumr.On("GetUserPreference - Failed", "Test").Return(userPrefs, errors.New("error"))

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    dto.LinkDetails
		wantErr bool
	}{
		{"Get user ctx - Passed", flds, arg, link, false},
		//{"Get user ctx - Failed", flds, args{userName: "Test"}, dto.LinkDetails{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetUserContextLandingPage(tt.args.userName)
			if err != nil {
				tt.wantErr = true
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserContextLandingPage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetUserContextLandingPage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetUserEmail(t *testing.T) {
	type args struct {
		userName string
	}
	user := dto.User{
		FullName:       "",
		Email:          "",
		KongUsername:   "",
		ExternalUserId: "",
		Status:         "",
		CreatedOn:      "",
		CreatedBy:      "",
		ModifiedOn:     "",
		ModifiedBy:     "",
		Roles:          nil,
	}
	users := []dto.User{}
	users = append(users, user)
	arg := args{userName: "User"}
	arg2 := args{userName: "Test"}
	mumr.On("GetUserEmail", "User").Return(users, nil)
	mumr.On("GetUserEmail", "Test").Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []dto.User
		wantErr bool
	}{
		{"Get user email - Passed", flds, arg, users, false},
		{"Get user email - Failed", flds, arg2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetUserEmail(tt.args.userName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetUserEmail() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_GetUserPreference(t *testing.T) {
	type args struct {
		userName string
	}
	userpref := dto.UserPreference{
		KongUsername: "",
		ResourceName: "",
		CreatedOn:    "",
		CreatedBy:    "",
		ModifiedOn:   "",
		ModifiedBy:   "",
	}
	userspref := []dto.UserPreference{}
	userspref = append(userspref, userpref)
	arg := args{userName: "User"}
	mumr.On("GetUserPreference", "User").Return(userspref, nil)
	mumr.On("GetUserPreference", "Test").Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []dto.UserPreference
		wantErr bool
	}{
		{"Get user pref - Passed", flds, arg, userspref, false},
		{"Get user pref - Failed", flds, args{userName: "Test"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.GetUserPreference(tt.args.userName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserPreference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetUserPreference() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_InitializeHedgeDB(t *testing.T) {
	mumr.On("InitializeHedgeDB").Return(nil)
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"Initialize db - Passed", flds, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.InitializeHedgeDB(); (err != nil) != tt.wantErr {
				t.Errorf("InitializeHedgeDB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_SaveNewGrafanaDashboard(t *testing.T) {
	type args struct {
		setting map[string]string
	}
	setting := map[string]string{"Grafana_Pass": "pass", "Grafana_Admin": "admin"}
	arg := args{setting: setting}
	mumr.On("GetAllDashboardResources").Return([]map[string]string{{"Grafana_Pass": "pass"}, {"Grafana_Admin": "admin"}}, nil)
	//mumr.On("GetAllDashboardResources").Return(errors.New("Grafana_Pass missing from configuration"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"Save grafana dash - Passed", flds, arg, false},
		{"Save grafana dash - Failed1", flds, args{setting: map[string]string{}}, true},
		{"Save grafana dash - Failed2", flds2, arg, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			if err := s.SaveNewGrafanaDashboard(tt.args.setting); (err != nil) != tt.wantErr {
				t.Errorf("SaveNewGrafanaDashboard() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUserManagementService_UpdateMenuResources(t *testing.T) {
	type args struct {
		menuInput dto.Resource
	}
	resource := &dto.Resource{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		AllowedPermissions: "",
		Tooltip:            "",
		IsTerminalItem:     false,
		DisplaySequence:    0,
		LinkDetails:        dto.LinkDetails{},
		SubResources:       nil,
	}
	resource2 := &dto.Resource{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		AllowedPermissions: "",
		Tooltip:            "",
		IsTerminalItem:     false,
		DisplaySequence:    0,
		LinkDetails:        dto.LinkDetails{},
		SubResources:       nil,
	}
	resourceArr := []dto.Resource{}
	resourceArr = append(resourceArr, *resource2)
	resource2.SubResources = resourceArr
	resources := dto.Resources{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          "",
		CreatedBy:          "",
		ModifiedOn:         "",
		ModifiedBy:         "",
	}
	resourcesArr := []dto.Resources{}
	resourcesArr = append(resourcesArr, resources)
	arg := args{menuInput: *resource}
	mumr.On("UpdateMenuResources", &resourcesArr).Return(&resourcesArr, nil)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.Resource
		wantErr bool
	}{
		{"UpdateMenuResources - Passed", flds, arg, resource2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.UpdateMenuResources(tt.args.menuInput)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateMenuResources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateMenuResources() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_UpdateRole(t *testing.T) {
	type args struct {
		updatedRole *dto.Role
	}
	role := &dto.Role{
		Name:                "",
		Description:         "",
		RoleType:            "",
		DefaultResourceName: "",
		CreatedOn:           "",
		CreatedBy:           "",
		ModifiedOn:          "",
		ModifiedBy:          "",
		Resources:           nil,
	}
	arg := args{updatedRole: role}
	//role2 := &dto.Role{
	//	Name:                "",
	//	Description:         "",
	//	RoleType:            "",
	//	DefaultResourceName: "",
	//	CreatedOn:           "",
	//	CreatedBy:           "",
	//	ModifiedOn:          "",
	//	ModifiedBy:          "",
	//	Resources:           nil,
	//}
	//arg2 := args{updatedRole: role2}
	mumr.On("UpdateRole", role).Return(role, nil)
	//mumr.On("UpdateRole", role2).Return(nil, errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.Role
		wantErr bool
	}{
		{"Update role - Passed", flds, arg, role, false},
		//{"Update role - Failed", flds2, arg2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.UpdateRole(tt.args.updatedRole)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_UpdateUser(t *testing.T) {
	type args struct {
		updatedUser *dto.User
		userName    string
	}
	user := dto.User{
		FullName:       "",
		Email:          "",
		KongUsername:   "",
		ExternalUserId: "",
		Status:         "",
		CreatedOn:      "",
		CreatedBy:      "",
		ModifiedOn:     "",
		ModifiedBy:     "",
		Roles:          nil,
	}
	user2 := dto.User{
		FullName:       "",
		Email:          "",
		KongUsername:   "test",
		ExternalUserId: "",
		Status:         "",
		CreatedOn:      "",
		CreatedBy:      "",
		ModifiedOn:     "",
		ModifiedBy:     "",
		Roles:          nil,
	}
	users := []dto.User{}
	users = append(users, user)
	arg := args{
		updatedUser: &user,
		userName:    "User",
	}
	arg2 := args{
		updatedUser: &user2,
		userName:    "Test",
	}
	mumr.On("UpdateUser", &user).Return(nil)
	mumr.On("UpdateUser", mock.Anything).Return(errors.New("error"))
	mumr.On("GetUser", "User").Return(users, nil)
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.User
		wantErr bool
	}{
		{"Update user - Passed", flds, arg, &user, false},
		{"Update user - Failed", flds2, arg2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.UpdateUser(tt.args.updatedUser, tt.args.userName)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateUser() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserManagementService_UpdateUserPreference(t *testing.T) {
	type args struct {
		updatedUserPreference *dto.UserPreference
		userName              string
	}
	userpref := dto.UserPreference{
		KongUsername: "",
		ResourceName: "",
		CreatedOn:    "",
		CreatedBy:    "",
		ModifiedOn:   "",
		ModifiedBy:   "",
	}
	userpref2 := dto.UserPreference{
		KongUsername: "",
		ResourceName: "",
		CreatedOn:    "",
		CreatedBy:    "",
		ModifiedOn:   "",
		ModifiedBy:   "",
	}
	prefArr := []dto.UserPreference{}
	prefArr = append(prefArr, userpref)
	arg := args{
		updatedUserPreference: &userpref,
		userName:              "User",
	}
	mumr.On("UpdateUserPreference", &userpref).Return(nil)
	mumr.On("GetUserPreference", "User").Return(prefArr, nil)
	//mumr.On("UpdateUserPreference", mock.Anything).Return(errors.New("error"))
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *dto.UserPreference
		wantErr bool
	}{
		{"UpdateUserPreference - Passed", flds, arg, &userpref2, false},
		//{"UpdateUserPreference - Failed", flds, args{
		//	updatedUserPreference: &userpref,
		//	userName:              "test",
		//}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := UserManagementService{
				appService:               tt.fields.appService,
				dbConfig:                 tt.fields.dbConfig,
				userManagementRepository: *tt.fields.userManagementRepository,
				dashboardURL:             tt.fields.dashboardURL,
			}
			got, err := s.UpdateUserPreference(tt.args.updatedUserPreference, tt.args.userName)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateUserPreference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UpdateUserPreference() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_populateDefaultFieldsResources(t *testing.T) {
	type args struct {
		resource *dto.Resources
	}
	resource := dto.Resources{
		Id:                 0,
		Name:               "",
		DisplayName:        "",
		Uri:                "",
		LinkType:           "",
		UiId:               "",
		Active:             false,
		ParentResource:     "",
		AllowedPermissions: "",
		CreatedOn:          time.Now().Format(time.RFC3339),
		CreatedBy:          "admin",
		ModifiedOn:         time.Now().Format(time.RFC3339),
		ModifiedBy:         "admin",
	}
	arg := args{resource: &resource}
	tests := []struct {
		name    string
		args    args
		want    *dto.Resources
		wantErr bool
	}{
		{"Populate fields - Passed", arg, &resource, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := populateDefaultFieldsResources(tt.args.resource)
			if (err != nil) != tt.wantErr {
				t.Errorf("populateDefaultFieldsResources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("populateDefaultFieldsResources() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_populateDefaultFieldsRole(t *testing.T) {
	type args struct {
		role *dto.Role
	}
	role := dto.Role{
		Name:                "",
		Description:         "",
		RoleType:            "",
		DefaultResourceName: "",
		CreatedOn:           time.Now().Format(time.RFC3339),
		CreatedBy:           "admin",
		ModifiedOn:          time.Now().Format(time.RFC3339),
		ModifiedBy:          "admin",
		Resources: []dto.Resources{{
			Id:                 0,
			Name:               "",
			DisplayName:        "",
			Uri:                "",
			LinkType:           "",
			UiId:               "",
			Active:             false,
			ParentResource:     "",
			AllowedPermissions: "",
			CreatedOn:          "",
			CreatedBy:          "",
			ModifiedOn:         "",
			ModifiedBy:         "",
		}},
	}
	arg := args{role: &role}
	tests := []struct {
		name    string
		args    args
		want    *dto.Role
		wantErr bool
	}{
		{"populate fields - Passed", arg, &role, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := populateDefaultFieldsRole(tt.args.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("populateDefaultFieldsRole() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("populateDefaultFieldsRole() got = %v, want %v", got, tt.want)
			}
		})
	}
}
