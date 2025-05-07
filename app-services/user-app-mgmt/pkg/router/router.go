/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

package router

import (
	"encoding/json"
	"errors"
	"fmt"
	emailverifier "github.com/AfterShip/email-verifier"
	"github.com/edgexfoundry/app-functions-sdk-go/v3/pkg/interfaces"
	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgconn"
	"github.com/labstack/echo/v4"
	"hedge/app-services/user-app-mgmt/pkg/converter"
	"hedge/app-services/user-app-mgmt/pkg/dto"
	"hedge/app-services/user-app-mgmt/pkg/service"
	"hedge/common/utils"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Router struct {
	appService            interfaces.ApplicationService
	appConfig             map[string]string
	isExternalAuth        bool
	userManagementService *service.UserManagementService
	validate              *validator.Validate
}

var verifier = emailverifier.NewVerifier()

func NewRouter(appService interfaces.ApplicationService, appConfig map[string]string) *Router {
	router := new(Router)
	router.appService = appService
	router.appConfig = appConfig
	if isExtAuth, ok := appConfig["Is_External_Auth"]; ok {
		if strings.EqualFold(isExtAuth, "true") {
			router.isExternalAuth = true
		} else {
			router.isExternalAuth = false
		}
	} else {
		router.isExternalAuth = false
	}
	router.userManagementService = service.NewUserManagementService(appService, appConfig)
	err := router.userManagementService.InitializeHedgeDB()
	if err != nil {
		appService.LoggingClient().Errorf("Error while initializing HedgeDB : %v", err)
		return nil
	}
	router.validate = validator.New()
	router.validate.RegisterValidation("matchRegex", matchRegex)

	return router
}

func matchRegex(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(fl.Param())
	return re.MatchString(fl.Field().String())
}

// CallGrafanaRoutine Fires the background job to sync dashboards
func (r *Router) CallGrafanaRoutine(config map[string]string) {
	dataSyncIntervalStr, found := config["Grafana_DataSync_Interval_mins"]

	var dataSyncDuration time.Duration
	var err error = nil
	if found {
		dataSyncDuration, err = time.ParseDuration(dataSyncIntervalStr)
	}
	if !found || err != nil {
		// backup default to something if not configured
		dataSyncDuration = 5 * time.Minute
	}
	go r.CallGrafanaService(config, dataSyncDuration)
	r.appService.LoggingClient().Infof("Started Go Routine CallGrafanaService")
}

func (r *Router) RegisterRoutes() {
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/usercontext", interfaces.Authenticated, func(c echo.Context) error {
		var userId string
		var userContext dto.UserContext
		var responseList []dto.Resource
		// Get the userId from the header
		userId, hedgeErr := utils.GetUserIdFromHeader(c.Request(), r.appService)
		if hedgeErr != nil {
			r.appService.LoggingClient().Errorf("Error while reading user from header : %v", hedgeErr)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return hedgeErr
		}
		users, err := r.userManagementService.GetUser(userId)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error reading the file: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		if len(users) == 0 {
			r.appService.LoggingClient().Errorf("No user found")
			c.Response().WriteHeader(http.StatusNotFound)
			return err
		}
		c.Response().Header().Set("Content-Type", "application/json")
		var resourcesList []dto.Resources
		var landingPage dto.LinkDetails
		resourcesList, err = r.userManagementService.GetResourcesByUser(userId)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error reading the file: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		if len(resourcesList) > 0 {
			//Convert MenuEntity To Resource
			for _, resource := range resourcesList {
				if len(resource.ParentResource) < 1 {
					//for parent menuItems
					responseList = converter.ConvertResourceEntityToMenuItemRes(resource, resourcesList, responseList)
				}
			}
		}
		sort.Slice(responseList[:], func(i, j int) bool {
			return responseList[i].Id < responseList[j].Id
		})
		userContext.Resources = responseList
		userContext.KongUsername = userId
		userContext.FullName = users[0].FullName
		userContext.ApplicationName, err = r.appService.GetAppSetting("Usr_application_name")
		landingPage, err = r.userManagementService.GetUserContextLandingPage(userId)
		if landingPage.LinkUri == "" {
			//Make the best guess about DefaultResource
			defaultResourceIndex := -1
			for i, role := range users[0].Roles {
				if role.DefaultResourceName != "" {
					defaultResourceIndex = i
					break
				}
				if defaultResourceIndex < 0 {
					// As a fallback, at least the person has a role, so use that info, we can't return with no home page
					defaultResourceIndex = 0
				}
			}
			var res dto.Resources
			if defaultResourceIndex >= 0 {
				res, err = r.userManagementService.GetResource(users[0].Roles[defaultResourceIndex].DefaultResourceName)
				if err != nil {
					r.appService.LoggingClient().Errorf("Could not get resource: %v", err)
				} else {
					landingPage.LinkUri = res.Uri
					landingPage.LinkType = res.LinkType
					landingPage.LinkId = res.UiId
				}
			} else {
				r.appService.LoggingClient().Errorf("No role for user with defaultResourceName found, user: %s", users[0].FullName)
			}
		}
		userContext.LandingPage = landingPage
		//on success
		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(userContext)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetUser:: End")
		return nil
	}, http.MethodGet)

	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user/auth_type", interfaces.Authenticated, func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "application/json")
		c.Response().WriteHeader(http.StatusOK)
		authType := map[string]bool{
			"isExternalAuth": r.isExternalAuth,
		}
		json.NewEncoder(c.Response()).Encode(authType)
		return nil
	}, http.MethodGet)

	// Subrequest for intercepting /auth_type request in Nginx auth.js (CSRF protection)
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user/auth_type_sub", interfaces.Authenticated, func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "application/json")
		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(r.isExternalAuth)
		return nil
	}, http.MethodGet)

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	//Create a new Hedge user. In case User is maintained in SSO/active directory, we create a user in here without auth details
	// SSO User creation is required, so we can assign roles to him/her within hedge
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateUser:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		var newUser dto.User
		var user service.User
		var credential service.LoginInputs

		reqBody, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading user creation inputs : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		err = json.Unmarshal(reqBody, &user)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while unmarshalling user, err : %v", err)
			var er = make(map[string]string)
			er["error"] = "Error while unmarshalling user, error:" + err.Error()
			json.NewEncoder(c.Response()).Encode(er)
			return err
		}

		// Perform validation based on the dto.User struct fields and tags
		err = r.validate.Struct(user)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while validating unmarshalled request payload : %v", err)
			var er = make(map[string]string)
			c.Response().WriteHeader(http.StatusBadRequest)
			er["error"] = "Error while validating unmarshalled request payload, error:" + err.Error()
			json.NewEncoder(c.Response()).Encode(er)
			return err
		}

		usr, err := r.userManagementService.GetUser(user.UserId)
		if len(usr) > 0 {
			r.appService.LoggingClient().Errorf("Error while creating user, user exist : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			var er = make(map[string]string)
			er["error"] = "UserId already exist"
			json.NewEncoder(c.Response()).Encode(er)
			return err
		}
		err = json.Unmarshal(reqBody, &newUser)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while unmarshalling user dto, err : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			return err
		}
		// Email id is blank for sso user typically
		if newUser.Email != "" {
			usr, err = r.userManagementService.GetUserEmail(newUser.Email)
			if len(usr) > 0 {
				r.appService.LoggingClient().Errorf("Error while creating user : %v", err)
				c.Response().WriteHeader(http.StatusBadRequest)
				var er = make(map[string]string)
				er["error"] = "Email already exist"
				json.NewEncoder(c.Response()).Encode(er)
				return err
			}
		}

		if !r.isExternalAuth {
			json.Unmarshal(reqBody, &credential)
			credential.Username = user.UserId
			if len(user.UserId) < 3 {
				r.appService.LoggingClient().Error("Error while creating user")
				c.Response().WriteHeader(http.StatusBadRequest)
				var er = make(map[string]string)
				er["error"] = "UserId cannot be less than 3 characters"
				json.NewEncoder(c.Response()).Encode(er)
				return err
			}

			if _, ok := validMailAddress(newUser.Email, r); !ok {
				r.appService.LoggingClient().Error("Error while creating user")
				c.Response().WriteHeader(http.StatusBadRequest)
				var er = make(map[string]string)
				er["error"] = "Invalid email"
				json.NewEncoder(c.Response()).Encode(er)
				return err
			}
			//call service
			err = r.userManagementService.CreateCredential(credential)
			if err != nil {
				r.appService.LoggingClient().Error("internal error creating creds")
				c.Response().WriteHeader(http.StatusInternalServerError)
				var er = make(map[string]string)
				er["error"] = "internal error unable to create credentials"
				json.NewEncoder(c.Response()).Encode(er)
				return err
			}
		}
		// Check if user is active or inactive
		users, err := r.userManagementService.GetInactiveUser(user.UserId)
		if len(users) == 1 {
			if users[0].Status == "Inactive" {
				result, err := r.userManagementService.UpdateUser(&newUser, user.UserId)
				if err != nil {
					r.appService.LoggingClient().Errorf("Error while updating the user : %v", user.UserId)
					c.Response().WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(c.Response()).Encode("error updating user")
					return err
				}
				c.Response().WriteHeader(http.StatusCreated)
				json.NewEncoder(c.Response()).Encode(result)
			}
		} else {
			result, err := r.userManagementService.CreateUser(newUser)
			if err != nil {
				r.appService.LoggingClient().Errorf("Error while creating the new user : %v", err)
				c.Response().WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(c.Response()).Encode("error creating new user: userId, check if all mandatory parameters are entered")
				return err
			}
			c.Response().WriteHeader(http.StatusCreated)
			json.NewEncoder(c.Response()).Encode(result)
		}
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateUser:: End")
		return nil
	}, http.MethodPost)

	//Get the User by userName (To fetch list of all users, userId= 'all')
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user/:userId", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetUser:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		userId := c.Param("userId")
		users, err := r.userManagementService.GetUser(userId)

		//if error
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in fetching the output object data AdeUserOp: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error getting user for userid: " + userId)
			return err
		}

		//if user not found
		if len(users) < 1 {
			c.Response().WriteHeader(http.StatusNotFound)
			users = []dto.User{}
			json.NewEncoder(c.Response()).Encode(users)
			return err
		}

		//on success
		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(users)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetUser:: End")
		return nil
	}, http.MethodGet)

	//Delete the User
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user/:userId", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() DeleteUser:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		userId := c.Param("userId")
		err := r.userManagementService.DeleteUser(userId)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while deleting the user : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error deleting user id, check if all related roles for the user is deleted")
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode("The user has been deleted successfully!")
		r.appService.LoggingClient().Debugf("The user with name %v has been deleted successfully", userId)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() DeleteUser:: End")
		return nil
	}, http.MethodDelete)

	//Update the User
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() UpdateUser:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		var updatedUser *dto.User
		reqBody, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		json.Unmarshal(reqBody, &updatedUser)
		if len(updatedUser.KongUsername) < 3 {
			r.appService.LoggingClient().Errorf("Error while updating user : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			var er = make(map[string]string)
			er["error"] = "UserId cannot be less than 3 characters"
			json.NewEncoder(c.Response()).Encode(er)
			return err
		}

		// Perform validation based on the dto.User struct fields and tags
		err = r.validate.Struct(updatedUser)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while validating unmarshalled request payload : %v", err)
			var er = make(map[string]string)
			er["error"] = "Error while validating unmarshalled request payload, error:" + err.Error()
			json.NewEncoder(c.Response()).Encode(er)
			return err
		}

		result, err := r.userManagementService.UpdateUser(updatedUser, updatedUser.KongUsername)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while updating the user : %v", updatedUser.KongUsername)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error updating user details")
			return err
		}
		if result == nil {
			r.appService.LoggingClient().Errorf("User Not found: %v", updatedUser.KongUsername)
			c.Response().WriteHeader(http.StatusNotFound)
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(result)
		r.appService.LoggingClient().Debugf("The user with userId %v has been updated successfully", updatedUser.KongUsername)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() updateUser:: End")
		return nil
	}, http.MethodPut)

	// Update user password
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user/pwd", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() UpdatePassword:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		if r.isExternalAuth {
			r.appService.LoggingClient().Errorf("password change not allowed since external authentication is configured")
			c.Response().WriteHeader(http.StatusMethodNotAllowed)
			return nil
		}
		var admin = false
		var currUser dto.User
		var user service.User
		var credential service.LoginInputs
		reqBody, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		userId, hedgeErr := utils.GetUserIdFromHeader(c.Request(), r.appService)
		if hedgeErr != nil {
			r.appService.LoggingClient().Errorf("Error while reading user from header : %v", hedgeErr)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return hedgeErr
		}
		userRoles, err := r.userManagementService.GetUser(userId)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while retrieving user from DB : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		if len(userRoles) > 0 {
			if userRoles[0].Roles != nil {
				for _, role := range userRoles[0].Roles {
					if role.Name == "UserAdmin" || role.Name == "PlatformAdmin" {
						admin = true
						break
					}
				}
			}
		} else {
			r.appService.LoggingClient().Errorf("Error while retrieving user and roles : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}

		json.Unmarshal(reqBody, &user)
		json.Unmarshal(reqBody, &currUser)
		json.Unmarshal(reqBody, &credential)
		if admin == false && currUser.KongUsername != userId {
			r.appService.LoggingClient().Errorf("Cannot change other users password")
			c.Response().WriteHeader(http.StatusForbidden)
			return err
		}

		credential.Username = user.UserId
		err = r.userManagementService.DeleteCredential(currUser.KongUsername)
		if err == nil {
			r.userManagementService.CreateCredential(credential)
			if err != nil {
				r.appService.LoggingClient().Error("internal error creating creds")
				c.Response().WriteHeader(http.StatusInternalServerError)
				var er = make(map[string]string)
				er["error"] = "internal error unable to create credentials"
				json.NewEncoder(c.Response()).Encode(er)
				return err
			}
		} else {
			r.appService.LoggingClient().Errorf("Error while updating the user : %v", currUser.KongUsername)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error deleting user")
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(userRoles)
		r.appService.LoggingClient().Debugf("The password for userId %v has been updated successfully", currUser.KongUsername)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() UpdatePassword:: End")
		return nil
	}, http.MethodPut)

	//Get the resources for the given user
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user/:userId/resources", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetResourcesForUser:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		userName := c.Param("userId")
		resources, err := r.userManagementService.GetResourcesByUser(userName)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in fetching the output object data: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		if resources == nil {
			c.Response().WriteHeader(http.StatusOK)
			resources = []dto.Resources{}
		} else {
			c.Response().WriteHeader(http.StatusOK)
		}
		json.NewEncoder(c.Response()).Encode(resources)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetResourcesForUser:: End")
		return nil
	}, http.MethodGet)

	//Create UserRole
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user/role", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() AssignUserRole:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		var userRole dto.UserRole
		var roles service.Roles
		reqBody, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		json.Unmarshal(reqBody, &roles)
		json.Unmarshal(reqBody, &userRole)
		err = r.userManagementService.DeleteUserRoles(userRole.UserKongUsername)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while updating user-roles : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		_, err = r.userManagementService.CreateUserRole(&userRole, roles)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while assigning role to user : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error assigning role to the user")
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		c.Response().Write(reqBody)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() AssignUserRole:: End")
		return nil
	}, http.MethodPost)

	//Get the list of resource URLs that the user has access to, this is used in Authorization plugin, so minimal output payload is
	// outputted
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user/:userId/resource_url/all", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetResourcesUrlForUser:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		userId := c.Param("userId")

		userRoleUrls, err := r.userManagementService.GetResourcesUrlByUser(userId)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in fetching the output object data: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		if userRoleUrls == nil {
			c.Response().WriteHeader(http.StatusNotFound)
		} else {
			c.Response().WriteHeader(http.StatusOK)
		}

		json.NewEncoder(c.Response()).Encode(userRoleUrls)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetResourcesUrlForUser:: End")
		return nil
	}, http.MethodGet)

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	//Create a Role
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/role", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateRole:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		var newRole *dto.Role
		reqBody, err := io.ReadAll(c.Request().Body)
		defer c.Request().Body.Close()
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}

		json.Unmarshal(reqBody, &newRole)

		// Perform validation based on the dto.Role struct fields and tags
		err = r.validate.Struct(newRole)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while validating unmarshalled request payload : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			return err
		}

		//call service
		newRoleName := newRole.Name
		newRole, err = r.userManagementService.CreateRole(newRole)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while creating the new user : %v", err)
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				uniqueViolationCode := "23505"
				if pgErr.Code == uniqueViolationCode {
					r.appService.LoggingClient().Errorf("Unique violation PgError code %v", pgErr.Code)
					c.Response().WriteHeader(http.StatusConflict)
					c.Response().Write([]byte(fmt.Sprintf("Role %v already exists", newRoleName)))
					return err
				}
			}

			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}

		//fmt.Println(newRole)
		//js, err := json.Marshal(newRole)
		//if err != nil {
		//	r.appService.LoggingClient().Errorf("Error while marshaling the newUser object : %v", err)
		//	c.Response().WriteHeader(http.StatusInternalServerError)
		//	return
		//}
		c.Response().WriteHeader(http.StatusCreated)
		c.Response().Header().Set("Content-Type", "application/json")
		//c.Response().Write(js)
		err = json.NewEncoder(c.Response()).Encode(newRole)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while encoding response payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateRole:: End")
		return nil
	}, http.MethodPost)

	//Get the Role by RoleName (To fetch list of all Roles, roleName= 'all')
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/role/:roleName", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetRole:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		roleName := c.Param("roleName")
		roles, err := r.userManagementService.GetRole(roleName)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in fetching the output object data: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(roles)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetRole:: End")
		return nil
	}, http.MethodGet)

	//Update a Role
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/role", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() UpdateRole:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		// Read the req object to extract username and password dto
		var updatedRole *dto.Role
		reqBody, err := io.ReadAll(c.Request().Body)
		defer c.Request().Body.Close()
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}

		json.Unmarshal(reqBody, &updatedRole)

		// Perform validation based on the dto.Role struct fields and tags
		err = r.validate.Struct(updatedRole)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while validating unmarshalled request payload : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			return err
		}

		//call service
		updatedRole, err = r.userManagementService.UpdateRole(updatedRole)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while creating the new user : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}

		//fmt.Println(updatedRole)
		//js, err := json.Marshal(updatedRole)
		//if err != nil {
		//	r.appService.LoggingClient().Errorf("Error while marshaling the newUser object : %v", err)
		//	c.Response().WriteHeader(http.StatusInternalServerError)
		//	return
		//}
		c.Response().WriteHeader(http.StatusOK)
		c.Response().Header().Set("Content-Type", "application/json")
		//c.Response().Write(js)
		err = json.NewEncoder(c.Response()).Encode(updatedRole)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while encoding response payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() UpdateRole:: End")
		return nil
	}, http.MethodPut)

	//Delete the Role by RoleName (To fetch list of all Roles, roleName= 'all')
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/role/:roleName", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() DeleteRole:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		roleName := c.Param("roleName")
		result, err := r.userManagementService.DeleteRole(roleName)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in fetching the output object data: %v", err)
			c.Response().WriteHeader(http.StatusConflict)
			json.NewEncoder(c.Response()).Encode("error deleting the role " + roleName + ": " + err.Error())
			return err
		}
		r.appService.LoggingClient().Debugf("The result of Delete Role: %v", result)

		json.NewEncoder(c.Response()).Encode("Role was deleted successfully")
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() DeleteRole:: End")
		return nil
	}, http.MethodDelete)

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	//Create a Role-Resource-Permission
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/role/resource", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateRoleResourcePermission:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		reqBody, err := io.ReadAll(c.Request().Body)
		defer c.Request().Body.Close()
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			json.NewEncoder(c.Response()).Encode("error reading the request payload")
			return err
		}

		var roleResource *dto.RoleResourcePermission
		var roleResources service.RoleResources
		json.Unmarshal(reqBody, &roleResource)
		json.Unmarshal(reqBody, &roleResources)
		if len(roleResources.ResourcesName) != len(roleResources.Permissions) {
			r.appService.LoggingClient().Errorf("Resources or permissions missing")
			c.Response().WriteHeader(http.StatusBadRequest)
			return err
		}

		// Perform validation based on the service.RoleResources struct fields and tags
		err = r.validate.Struct(roleResources)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while validating unmarshalled request payload : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			return err
		}

		//call service
		err = r.userManagementService.DeleteRoleResourcePermission(roleResources.RoleName)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in fetching the output object data: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		if len(roleResources.ResourcesName) == 0 {
			r.appService.LoggingClient().Debugf("Sent empty resources")
			c.Response().WriteHeader(http.StatusOK)
			return err
		}

		for i := range roleResources.ResourcesName {
			roleResource.ResourcesName = roleResources.ResourcesName[i]
			roleResource.Permission = roleResources.Permissions[i]
			roleResource, err = r.userManagementService.CreateRoleResourcePermission(roleResource)
			if err != nil {
				r.appService.LoggingClient().Errorf("Error while creating the new role-resource : %v", err)
				c.Response().WriteHeader(http.StatusInternalServerError)
				return err
			}
		}

		c.Response().WriteHeader(http.StatusOK)
		c.Response().Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(c.Response()).Encode(roleResources)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while encoding response payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateRoleResourcePermission:: End")
		return nil
	}, http.MethodPost)

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	//Get the Resources by ResourceName (To fetch list of all the Resources, resourceName= 'all')
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/resource/:resourceName", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetMenu:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		menuName := c.Param("resourceName")
		fetchResourcesOnlyStr := c.QueryParam("fetchResourcesOnly")
		fetchResourcesOnly := false
		if fetchResourcesOnlyStr == "true" {
			fetchResourcesOnly = true
		}
		menuItemResList, err := r.userManagementService.GetMenuResource(menuName, fetchResourcesOnly)
		//if error
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in fetching the output object data: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error getting menu items, contact your system admin")
			return err
		}
		//if resource not found
		if len(menuItemResList) < 1 {
			c.Response().WriteHeader(http.StatusNotFound)
			json.NewEncoder(c.Response()).Encode("No record found")
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(menuItemResList)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetMenus:: End")
		return nil
	}, http.MethodGet)

	//Create a Resource
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/resource", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateMenu:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		var menuInput dto.Resource
		reqBody, err := io.ReadAll(c.Request().Body)
		defer c.Request().Body.Close()
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		err = json.Unmarshal(reqBody, &menuInput)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while unmarshal request payload : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			return err
		}

		// Perform validation based on the dto.Resource struct fields and tags
		err = r.validate.Struct(menuInput)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while validating unmarshalled request payload : %v", err)
			c.Response().WriteHeader(http.StatusBadRequest)
			return err
		}

		//call service
		err = r.userManagementService.CreateMenuResources(menuInput)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while creating the menus : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		c.Response().WriteHeader(http.StatusCreated)
		err = json.NewEncoder(c.Response()).Encode(menuInput)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while encoding response payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateMenu:: End")
		return nil
	}, http.MethodPost)

	//Update a Resource
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/resource", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() UpdateMenu:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		var menuInput dto.Resource
		reqBody, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		json.Unmarshal(reqBody, &menuInput)
		updatedMenusItemResp, err := r.userManagementService.UpdateMenuResources(menuInput)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while creating the new user : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(updatedMenusItemResp)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() UpdateMenu:: End")
		return nil
	}, http.MethodPut)

	//Delete the Resource
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/resource/:resourceName", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() DeleteMenuResource:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		menuName := c.Param("resourceName")
		err := r.userManagementService.DeleteMenuResource(menuName)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in Deleting the menu: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error deleting menu items for menu name: " + menuName)
			return err
		}
		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode("The Resource has been deleted successfully!")
		r.appService.LoggingClient().Debugf("The user with name %v has been deleted successfully", menuName)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() DeleteMenuResource:: End")
		return nil
	}, http.MethodDelete)

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	//Create a UserPreference
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user_preference", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateUserPreference:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		var newUserPreference dto.UserPreference

		reqBody, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}

		json.Unmarshal(reqBody, &newUserPreference)

		isAuthorized, err := r.checkIfUserAuthorized(c, newUserPreference.KongUsername)

		if !isAuthorized {
			r.appService.LoggingClient().Errorf("Error while creating the new userPreference : %v", err)
			c.Response().WriteHeader(http.StatusForbidden)
			json.NewEncoder(c.Response()).Encode("error creating user preference")
			return err
		}

		//call service
		result, err := r.userManagementService.CreateUserPreference(newUserPreference)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while creating the new userPreference : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error creating user preference")
			return err
		}

		c.Response().WriteHeader(http.StatusCreated)
		json.NewEncoder(c.Response()).Encode(result)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() CreateUserPreference:: End")
		return nil
	}, http.MethodPost)

	//Get the UserPreference by userName
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user_preference/:userId", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetUserPreference:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		userName := c.Param("userId")

		isAuthorized, err := r.checkIfUserAuthorized(c, userName)
		if !isAuthorized {
			r.appService.LoggingClient().Errorf("Error while getting userPreference : %v", err)
			c.Response().WriteHeader(http.StatusForbidden)
			json.NewEncoder(c.Response()).Encode("error getting user preference")
			return err
		}

		userPreferences, err := r.userManagementService.GetUserPreference(userName)

		//if error
		if err != nil {
			r.appService.LoggingClient().Errorf("Error in fetching the output object data UserPreference: %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error getting user preference")
			return err
		}

		//if userPreference not found
		if len(userPreferences) < 1 {
			c.Response().WriteHeader(http.StatusNotFound)
			userPreferences = []dto.UserPreference{}
			json.NewEncoder(c.Response()).Encode(userPreferences)
			return err
		}

		//on success
		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(userPreferences)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() GetUserPreference:: End")
		return nil
	}, http.MethodGet)

	//Update the UserPreference
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user_preference", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() UpdateUserPreference:: Start")
		c.Response().Header().Set("Content-Type", "application/json")
		var updatedUserPreference *dto.UserPreference
		reqBody, err := io.ReadAll(c.Request().Body)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while reading the request payload : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			return err
		}
		json.Unmarshal(reqBody, &updatedUserPreference)

		isAuthorized, err := r.checkIfUserAuthorized(c, updatedUserPreference.KongUsername)

		if !isAuthorized {
			r.appService.LoggingClient().Errorf("Operation denied: user can change only his own preference. User %v", updatedUserPreference.KongUsername)
			c.Response().WriteHeader(http.StatusForbidden)
			json.NewEncoder(c.Response()).Encode("error updating user preference")
			return err
		}

		result, err := r.userManagementService.UpdateUserPreference(updatedUserPreference, updatedUserPreference.KongUsername)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while updating the userPreference : %v", updatedUserPreference.KongUsername)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error updating user preference")
			return err
		}

		if result == nil {
			r.appService.LoggingClient().Errorf("UserPreference Not found: %v", updatedUserPreference.KongUsername)
			c.Response().WriteHeader(http.StatusNotFound)
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode(result)
		r.appService.LoggingClient().Debugf("The userPreference with name %v has been updated successfully", updatedUserPreference.KongUsername)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() updateUserPreference:: End")
		return nil
	}, http.MethodPut)

	//Delete the UserPreference
	r.appService.AddCustomRoute("/api/v3/usr_mgmt/user_preference/:userId", interfaces.Authenticated, func(c echo.Context) error {
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() DeleteUserPreference:: Start")
		c.Response().Header().Set("Content-Type", "application/json")

		userName := c.Param("userId")

		isAuthorized, err := r.checkIfUserAuthorized(c, userName)
		if !isAuthorized {
			r.appService.LoggingClient().Errorf("Error while deleting the userPreference : %v", err)
			c.Response().WriteHeader(http.StatusForbidden)
			json.NewEncoder(c.Response()).Encode("error deleting user preference")
			return err
		}

		err = r.userManagementService.DeleteUserPreference(userName)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error while deleting the userPreference : %v", err)
			c.Response().WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(c.Response()).Encode("error deleting user preference")
			return err
		}

		c.Response().WriteHeader(http.StatusOK)
		json.NewEncoder(c.Response()).Encode("The userPreference has been deleted successfully!")
		r.appService.LoggingClient().Debugf("The userPreference with name %v has been deleted successfully", userName)
		r.appService.LoggingClient().Debugf("router.AddCustomRoute() DeleteUserPreference:: End")
		return nil
	}, http.MethodDelete)

}

func (r *Router) CallGrafanaService(setting map[string]string, dataSyncDuration time.Duration) {
	for range time.Tick(dataSyncDuration) {
		r.appService.LoggingClient().Infof("Attempting to sync Grafana dashboards")
		err := r.userManagementService.SaveNewGrafanaDashboard(setting)
		if err != nil {
			r.appService.LoggingClient().Errorf("Error Synchronizing Grafana dashboards: %v", err)
		}
	}
}

// checkIfUserAuthorized compares userId provided from request's header with username provided in request body (as kongUsername)
// return true if the names are equals, thus signifying
func (r *Router) checkIfUserAuthorized(c echo.Context, userNameFromRequestBody string) (bool, error) {
	userId, hedgeErr := utils.GetUserIdFromHeader(c.Request(), r.appService)
	if hedgeErr != nil {
		r.appService.LoggingClient().Errorf("Error while reading user from header : %v", hedgeErr)
		return false, errors.New(hedgeErr.Message())
	}

	if userId == userNameFromRequestBody && userNameFromRequestBody != "" {
		return true, nil
	} else {
		return false, errors.New("user is unauthorized to perform this action")
	}
}

func validMailAddress(address string, r *Router) (string, bool) {
	ret, err := verifier.Verify(address)
	if err != nil {
		r.appService.LoggingClient().Errorf("Error verifying email: %v", err)
		return "", false
	}
	if !ret.Syntax.Valid {
		return "", false
	}
	return "", true
}
