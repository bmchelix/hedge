/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

------------------------------ DDL ------------------------------
CREATE SCHEMA IF NOT EXISTS hedge
    AUTHORIZATION hedge;

CREATE TABLE IF NOT EXISTS hedge."user" (
   full_name VARCHAR(50) NOT NULL,
   kong_username VARCHAR(50) UNIQUE,
   external_user_id VARCHAR(50),
   email VARCHAR (255) UNIQUE NOT NULL,
   status VARCHAR(50) NOT NULL,
   created_on TIMESTAMP NOT NULL,
   created_by VARCHAR(50) NOT NULL,
   modified_on TIMESTAMP,
   modified_by VARCHAR(50),
   sys_user INTEGER DEFAULT 0 NOT NULL
);

CREATE TABLE IF NOT EXISTS hedge."resources" (
     id serial unique,
     name VARCHAR(50) NOT NULL,
     PRIMARY KEY (name),
     display_name VARCHAR(100),
     uri VARCHAR(100),
     link_type VARCHAR(50),
     ui_id VARCHAR(50),
     active BOOLEAN NOT NULL,
     parent_resource VARCHAR(50),
     allowed_permissions VARCHAR(500),
     created_on TIMESTAMP NOT NULL,
     created_by VARCHAR(50) NOT NULL,
     modified_on TIMESTAMP,
     modified_by VARCHAR(50),
     FOREIGN KEY (parent_resource)
         REFERENCES hedge."resources" (name)
         ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS hedge."role" (
   name VARCHAR(50) UNIQUE NOT NULL,
   description VARCHAR(50),
   role_type VARCHAR(50) NOT NULL,
   default_resource_name varchar(50),
 --  FOREIGN KEY (default_resource_name)
 --      REFERENCES hedge."resources" (name),
   created_on TIMESTAMP NOT NULL,
   created_by VARCHAR(50) NOT NULL,
   modified_on TIMESTAMP,
   modified_by VARCHAR(50)
);

CREATE TABLE IF NOT EXISTS hedge."user_roles" (
	user_kong_username VARCHAR(50) NOT NULL,
	role_name VARCHAR(50) NOT NULL,
	PRIMARY KEY (user_kong_username, role_name),
	FOREIGN KEY (role_name)
		REFERENCES hedge."role" (name),
	FOREIGN KEY (user_kong_username)
		REFERENCES hedge."user" (kong_username)
);


CREATE TABLE IF NOT EXISTS hedge."urls"
(
    name        VARCHAR(50) NOT NULL,
    url         VARCHAR(100) NOT NULL,
    description VARCHAR(100),
    PRIMARY KEY (name)
    );

CREATE TABLE IF NOT EXISTS hedge."resource_urls"
(
    resource_name VARCHAR(50)  NOT NULL,
    url_name         VARCHAR(50)  NOT NULL,
    description VARCHAR(100),
    PRIMARY KEY (resource_name, url_name),
    FOREIGN KEY (resource_name)
    REFERENCES hedge."resources" (name)
    ON DELETE CASCADE,
    FOREIGN KEY (url_name)
    REFERENCES hedge."urls" (name)
    ON DELETE CASCADE
    );

CREATE TABLE IF NOT EXISTS hedge."user_preference" (
   kong_username VARCHAR(50) NOT NULL,
   PRIMARY KEY (kong_username),
   resource_name VARCHAR(50),
   created_on TIMESTAMP NOT NULL,
   created_by VARCHAR(50) NOT NULL,
   modified_on TIMESTAMP,
   modified_by VARCHAR(50),
   FOREIGN KEY (kong_username)
		REFERENCES hedge."user" (kong_username)
		ON DELETE CASCADE,
   FOREIGN KEY (resource_name)
		REFERENCES hedge."resources" (name)
		ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS hedge."role_resource_permission" (
    role_name VARCHAR(50) NOT NULL,
    resources_name VARCHAR(50) NOT NULL,
    permission VARCHAR(50) DEFAULT 'R',
    PRIMARY KEY (role_name, resources_name),
    FOREIGN KEY (resources_name)
        REFERENCES hedge."resources" (name)
        ON DELETE CASCADE,
    FOREIGN KEY (role_name)
        REFERENCES hedge."role" (name)
        ON DELETE CASCADE
    );

------------------------------ user ------------------------------ 	
INSERT INTO hedge.user(full_name, kong_username, email, status, created_on, created_by, sys_user)
	VALUES ('kong', 'kong', 'kong@bmc.com', 'Active', current_timestamp, 'admin', 1)
	ON CONFLICT DO NOTHING;
	
INSERT INTO hedge.user(full_name, kong_username, email, status, created_on, created_by, sys_user)
	VALUES ('USERNAME', 'USERNAME', 'USER_EMAIL', 'Active', current_timestamp, 'admin', 1)
	ON CONFLICT DO NOTHING;

------------------------------ role ------------------------------ 	Add default_resource_name
INSERT INTO hedge.role(
	name, description, role_type, default_resource_name, created_on, created_by )
	VALUES ('DashboardUser', 'Dashboard User who can view and build dashboards', 'Business', 'dashboard', current_timestamp, 'admin')
	ON CONFLICT DO NOTHING;

INSERT INTO hedge.role(
	name, description, role_type, default_resource_name, created_on, created_by)
	VALUES ('UserAdmin', 'Dashboard viewer', 'Business', 'admin_user', current_timestamp, 'admin')
	ON CONFLICT DO NOTHING;

INSERT INTO hedge.role(
	name, description, role_type, default_resource_name, created_on, created_by)
	VALUES ('OTAnalyst', 'Operations Technology Analyst', 'Platform', 'intelligence_ml', current_timestamp, 'admin')
	ON CONFLICT DO NOTHING;	

INSERT INTO hedge.role(
	name, description, role_type, default_resource_name, created_on, created_by)
	VALUES ('PlatformAdmin', 'Platform Administrator', 'Platform','devices_device', current_timestamp, 'admin')
	ON CONFLICT DO NOTHING;

INSERT INTO hedge.role(
    name, description, role_type, default_resource_name, created_on, created_by)
VALUES ('ETL', 'Hedge ETL', 'Platform','etl', current_timestamp, 'admin')
    ON CONFLICT DO NOTHING;

------------------------------ resources (top menu) ------------------------------
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('analytics', 'Dashboards', null, 'menu', null, null, true, current_timestamp, 'admin', 'R')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, link_type,active, created_on, created_by, allowed_permissions)
VALUES ('devices', 'Device Management', 'menu', true, current_timestamp, 'admin', 'R')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, link_type, active, created_on, created_by, allowed_permissions)
VALUES ('intelligence', 'Intelligence', 'menu', true, current_timestamp, 'admin', 'R')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, link_type, active, created_on, created_by, allowed_permissions)
VALUES ('admin', 'Administration', 'menu', true, current_timestamp, 'admin', 'R')
    ON CONFLICT DO NOTHING;

------------------------------ resources (sub-menus) ------------------------------
-- Dashboards --
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('dashboard', 'Dashboard Builder', 'DASHBOARD_URL', 'LINK_TYPE', 'Dashboards', 'analytics', true, current_timestamp, 'admin', 'RW')
    ON CONFLICT DO NOTHING;

-- Device Management --
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id, parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('devices_profile', 'Profiles', '/profile/listprofile', 'view', null, 'devices', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id, parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('devices_device', 'Devices', '/device/listdevice', 'view', null, 'devices', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id, parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('devices_nodes', 'Nodes', '/nodes/nodeslist', 'view', null, 'devices', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
-- INSERT INTO hedge.resources(
--    name, display_name, uri, link_type, ui_id, parent_resource, active, created_on, created_by, allowed_permissions)
-- VALUES ('devices_discover', 'Discover', '/discovery', 'view', null, 'devices', false, current_timestamp, 'admin', 'R')
-- ON CONFLICT DO NOTHING;;

-- Intelligence --
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('intelligence_rule', 'Rule Editor', '/rule-manager', 'view', null, 'intelligence', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id, parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('intelligence_ml', 'Machine Learning', '/ml', 'view', null, 'intelligence', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('intelligence_workflow', 'Workflow Builder', '/processDesigner', 'view', 'process_designer', 'intelligence', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('intelligence_digitaltwin', 'Digital Twin', '/twin', 'view', null, 'intelligence', true, current_timestamp, 'admin', 'W')
ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('integrations_monitoring', 'Events', 'HELIX_URL', 'newTab', null, 'intelligence', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('integrations_ticketing', 'Tickets', 'DWP_URL', 'newTab', null, 'intelligence', false, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;

-- Administration --
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('admin_role', 'Role Management', '/helix-admin/roles', 'view', null, 'admin', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('admin_user', 'User Management', '/helix-admin/users', 'view', null, 'admin', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('admin_serviceHealth', 'Service Health', 'CONSUL_SERVICES_URL', 'newTab', null, 'admin', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('admin_serviceConfiguration', 'Service Configuration', 'CONSUL_KEYVALUE_URL', 'newTab', null, 'admin', true, current_timestamp, 'admin', 'W')
    ON CONFLICT DO NOTHING;

-- Device services (for granting role permissions) --
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('devicesvc_rest', 'Device REST', null, 'newTab', null, null, true, current_timestamp, 'admin', 'RW')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id,  parent_resource, active, created_on, created_by, allowed_permissions)
VALUES ('devicesvc_virtual', 'Device Virtual', null, 'newTab', null, null, true, current_timestamp, 'admin', 'RW')
    ON CONFLICT DO NOTHING;


---------------- ETL JOB ----------------
INSERT INTO hedge.resources(
    name, display_name, uri, link_type, ui_id, active, created_on, created_by, allowed_permissions)
VALUES ('etl', 'ETL', '/hedge/etl', 'newTab', null, true, current_timestamp, 'admin', 'W');


------------------------------ user_roles ------------------------------
INSERT INTO hedge.user_roles(
    user_kong_username, role_name)
VALUES ('USERNAME', 'PlatformAdmin')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.user_roles(
    user_kong_username, role_name)
VALUES ('USERNAME', 'OTAnalyst')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.user_roles(
    user_kong_username, role_name)
VALUES ('USERNAME', 'DashboardUser')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.user_roles(
    user_kong_username, role_name)
VALUES ('USERNAME', 'UserAdmin')
    ON CONFLICT DO NOTHING;
INSERT INTO hedge.user_roles(
    user_kong_username, role_name)
VALUES ('USERNAME', 'ETL')
    ON CONFLICT DO NOTHING;


------------------------------ role_resource_access ------------------------------
INSERT INTO hedge.role_resource_permission(
	role_name, resources_name, permission)
	VALUES ('PlatformAdmin', 'devices_device', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'devices_profile', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'devices_nodes', 'RW');
-- INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
-- VALUES ('PlatformAdmin', 'devices_discover', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission, sys_res_user)
--VALUES ('PlatformAdmin', 'devices', 'RW', 1);

INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'intelligence_ml', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'intelligence_digitaltwin', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('PlatformAdmin', 'ml_configurations', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('PlatformAdmin', 'ml_jobs', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('PlatformAdmin', 'ml_model_deployments', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('PlatformAdmin', 'ml_inferencing', 'RW');

--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('PlatformAdmin', 'admin','RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'admin_user', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'admin_role', 'RW');

INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'admin_serviceHealth', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'admin_serviceConfiguration', 'RW');

INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'integrations_ticketing', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'integrations_monitoring', 'RW');

--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('PlatformAdmin', 'intelligence', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'intelligence_rule', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'intelligence_workflow', 'RW');
-- INSERT INTO hedge.role_resource_permission(
--     role_name, resources_name, permission)
-- VALUES ('PlatformAdmin', 'intelligence_simulator', 'R');

INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'dashboard', 'RW');

INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'devicesvc_rest', 'RW');

INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('PlatformAdmin', 'devicesvc_virtual', 'RW');

---- Resources OTAnalyst that can use
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
 VALUES ('OTAnalyst', 'devices_device', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('OTAnalyst', 'devices_profile', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('OTAnalyst', 'devices_nodes', 'RW');
-- INSERT INTO hedge.role_resource_permission(
--     role_name, resources_name, permission)
-- VALUES ('OTAnalyst', 'devices_discover', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('OTAnalyst', 'devices', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('OTAnalyst', 'dashboard', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('OTAnalyst', 'intelligence', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('OTAnalyst', 'intelligence_rule', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('OTAnalyst', 'intelligence_workflow', 'RW');
-- INSERT INTO hedge.role_resource_permission(
--     role_name, resources_name, permission)
-- VALUES ('OTAnalyst', 'intelligence_simulator', 'R');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('OTAnalyst', 'intelligence_ml', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('OTAnalyst', 'intelligence_digitaltwin', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('OTAnalyst', 'ml_configurations', 'RW');
--INSERT INTO hedge.role_resource_permission(
--    role_name, resources_name, permission)
--VALUES ('OTAnalyst', 'ml_jobs', 'RW');

--DashboardUser
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('DashboardUser', 'dashboard', 'RW');

--UserAdmin
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('UserAdmin', 'admin', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('UserAdmin', 'admin_user', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('UserAdmin', 'admin_role', 'RW');
INSERT INTO hedge.role_resource_permission(
   role_name, resources_name, permission)
VALUES ('UserAdmin', 'admin_serviceConfiguration', 'RW');
INSERT INTO hedge.role_resource_permission(
    role_name, resources_name, permission)
VALUES ('UserAdmin', 'admin_serviceHealth', 'RW');

----------------------hedge."urls" --------------------------
INSERT INTO hedge."urls"(name, url, description)
VALUES ('Device', '/hedge/api/v3/device', 'Multiple URLs to support Device data');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('DeviceMetaData', '/hedge/api/v3/metadata', 'URLs to support Device metadata');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('Event', '/hedge/api/v3/event', 'URLs to support Event query');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('UserManagement', '/hedge/api/v3/usr_mgmt', 'URLs to support Event query');
-- anomaly URL can be removed later
INSERT INTO hedge."urls"(name, url, description)
VALUES ('MLConfiguration', '/hedge/api/v3/ml_management', 'URL to support ML configuration');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('DigitalTwin', '/hedge/api/v3/twin', 'URL to support ML Digital Twin Config and Simulation');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('MLInferencing', '/hedge/api/v3/ml_inferencing', 'URL to support anomaly inferencing testing');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('Dashboard', 'DASHBOARD_URL', 'URL to support Dashboard access');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('Rule', '/hedge/api/v3/rules', 'URL to support Rule Editor');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('Workflow', '/hedge/hedge-node-red', 'URL to support Workflow aka node-red Editor');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('NodeManagement', '/hedge/api/v3/node_mgmt', 'URL for Node Management');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('Content', '/hedge/api/v3/content', 'URL for Content');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('RuleSQL', '/hedge/api/v3/sql', 'URL for sql to support rules');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('DeviceREST', '/hedge/rest-device', 'URL For REST device service');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('DeviceVirtual', '/hedge/virtual-device', 'URL For Virtual device service commands');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('ServiceConfiguration', '/hedge/consul', 'URL for Consul');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('ServiceHealth', '/hedge/consul', 'URL for Consul');
INSERT INTO hedge."urls"(name, url, description)
VALUES ('postgrest', '/hedge/etl', 'URL to access postgrest /hedge/etl');

-------------------Resource to URL mapping hedge."resource_urls" --------------------------
--- /listdevice, /listprofile
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devices_device', 'Device', '/listdevice');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devices_device', 'DeviceMetaData', '/listdevice');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devices_device', 'Event', '/listdevice Table data');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devices_profile', 'Device', '/listprofile');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devices_profile', 'DeviceMetaData', '/listprofile');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devices_nodes', 'NodeManagement', '/nodeslist');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devices_device', 'NodeManagement', '/node_group');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devices_nodes', 'Content', '/content');
-- dashboard --
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('dashboard', 'Dashboard', '/hedge/hedge-dashboard');
--anomaly_configuration--
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('intelligence_ml', 'MLConfiguration', '/ml_management');

INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('intelligence_digitaltwin', 'DigitalTwin', '/twin');
--INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
--VALUES ('ml_jobs', 'MLConfiguration', '/ml_management');
--INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
--VALUES ('ml_model_deployments', 'MLConfiguration', '/ml_management');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('intelligence_ml', 'MLInferencing', '/ml_inferencing');
--
--intelligence_workflow ----
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('intelligence_workflow', 'Workflow', '/hedge/hedge-node-red');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('intelligence_workflow', 'NodeManagement', '/node_group');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('intelligence_rule', 'Rule', '/rule-manager');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('intelligence_rule', 'NodeManagement', '/node_group');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('intelligence_rule', 'RuleSQL', '/sql');

--admin_resource--
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('admin_role', 'UserManagement', '/hedge/usr_mgmt');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('admin_user', 'UserManagement', '/hedge/usr_mgmt');

--admin_serviceHealth-
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('admin_serviceConfiguration', 'ServiceConfiguration', '/hedge/consul');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('admin_serviceHealth', 'ServiceHealth', '/hedge/consul');

--REST device service
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devicesvc_rest', 'DeviceREST', '/hedge/rest-device');
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('devicesvc_virtual', 'DeviceVirtual', '/hedge/virtual-device');

INSERT INTO hedge.user_preference (kong_username, resource_name, created_on, created_by, modified_on, modified_by)
VALUES ('USERNAME', 'devices_device', current_timestamp, 'admin', null, null);

-- ETL postgrest
INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES ('etl', 'postgrest', '/hedge/etl');
