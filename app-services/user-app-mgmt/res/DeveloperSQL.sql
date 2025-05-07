/*******************************************************************************
* Contributors: BMC Software, Inc. - BMC Helix Edge
*
* (c) Copyright 2020-2025 BMC Software, Inc.
*******************************************************************************/

CREATE SCHEMA hedge;
ALTER TABLE usr_role_resource_access SET SCHEMA hedge;
ALTER TABLE usr_user_roles SET SCHEMA hedge;
ALTER TABLE usr_resources SET SCHEMA hedge;
ALTER TABLE usr_role SET SCHEMA hedge;
ALTER TABLE usr_user SET SCHEMA hedge;

ALTER TABLE hedge.resources
ADD COLUMN display_name VARCHAR(100),
ADD COLUMN link_type VARCHAR(50),
ADD COLUMN ui_id VARCHAR(50),
ADD COLUMN parent_resource VARCHAR(50);

show search_path;


SHOW max_connections;
select count(*) from pg_stat_activity;
SELECT * FROM pg_stat_activity;
SELECT pid, state, usename datname, datid from pg_stat_activity;
SELECT pid, usename, usesysid, datid, datname, application_name, backend_start, state_change, state FROM pg_stat_activity WHERE state = 'idle';
//Kill an Idle Connection:
SELECT pg_terminate_backend(1254);   
SELECT datid, usename, datname, pid, state FROM pg_stat_activity WHERE state = 'idle';
show idle_in_transaction_session_timeout;
------------------------------------------------------------------------------------------

SELECT * FROM hedge.user WHERE Name = 'Steve' AND Status='Active'

delete from hedge.user where kong_customer_id IN ('567', '123_updated')

select * from hedge.role where name = 'GrafanaDashboardEditor';

select * from hedge.resources; //frameView/Analytics
 ///hedge-grafana //"grafana-dashboard" "node-red"
select * from hedge.role_resource_access where resources_name = 'grafana-dashboard';
select * from hedge.user;// Add fullName, image, Replace name with userName and reference_id with kong_customer_id
select * from hedge.user_roles where user_name = 'kong' //role_name = 'dashboard-viewer';
select * from hedge.role_resource_access where role_name in (select role_name from hedge.user_roles where user_name = 'kong')
select * from hedge.role //RoleType = BUSINESS, PLATFORM (GG)
select * from hedge.resources where name IN (select resources_name from hedge.role_resource_access where role_name in (select role_name from hedge.user_roles where user_name = 'kong')) 
select * from hedge.menuitem;
select * from hedge.user_preference;
select * from hedge.resources where name IN ('mainmenu', 'subMenu1', 'subMenu2')
select * from hedge.role_resource_permission;
SELECT * FROM hedge.user_preference WHERE user_name = 'kong'
------------------------------
//drop table hedge.role_resource_permission;
------------------------------
select * from hedge.resources where name ='grafana-dashboard'
delete from hedge.resources where name IN ('grafana-dashboard')
Update hedge.resources set active = true where name ='grafana-dashboard'
Update hedge.resources set allowed_permissions = 'READ, WRITE' where name IN ('mainmenu', 'subMenu1', 'subMenu2')
Update hedge.resources set Active=true
-------------
drop table hedge.role_menu_access
drop table hedge.user_preference
drop table hedge.resources
drop table hedge.role_resource_access
----------------- insert -------------------
INSERT INTO hedge.menuitem(
	name, display_name, link_uri, link_type, parent_menuitem_name, resource_ref_name, created_on, created_by)
	VALUES ('profile_vehicles', 'Profile', '/listprofile', 'view', 'vehicles', null, current_timestamp, 'admin');
insert into hedge.user_roles values('kong', 'workflow-viewer');
insert into hedge.user_roles values('kong', 'dashboard-editor')
insert into hedge.role_resource_access values('rule-editor', 'grafana-dashboard')
insert into hedge.role_resource_permission 
	values('workflow-editor', 'mainmenu', 'READ'),
		   ('workflow-editor', 'mainmenu', 'WRITE')
		   
drop TABLE role_resource_permission Modify CONSTRAINT constraint_name UNIQUE (column1, column2, ... column_n); 


INSERT INTO hedge.resources(
	name, uri, active, created_on, created_by)
	VALUES ('hedge-ui', '/api/v3', true, current_timestamp, 'admin');

------
Update hedge.resources set active = true where name ='grafana-dashboard'
delete from hedge.role_resource_access where role_name = 'workflow-editor' and resources_name = 'rule-editor'

ALTER TABLE hedge.resources
RENAME COLUMN permissions TO allowed_permissions;

ALTER TABLE hedge.user
RENAME COLUMN user_name TO name;
ALTER TABLE hedge.user
RENAME COLUMN reference_id TO kong_customer_id;


delete from hedge.role_resource_access where resources_name = 'resName2'
delete from hedge.resources where name = 'resName2'

alter table hedge.resources set 

ALTER TABLE hedge.role_resource_access
ALTER COLUMN rolename TYPE  boolean 
USING active::boolean;

ALTER TABLE hedge.role_resource_access 
RENAME COLUMN resource_name TO resources_name;


CREATE TABLE IF NOT EXISTS "hedge.role_resource_access1" (
	role_name VARCHAR(50) NOT NULL,
	resource_name VARCHAR(50) NOT NULL,
	PRIMARY KEY (role_name, resource_name),
	FOREIGN KEY (resource_name)
		REFERENCES hedge.resources (name),
	FOREIGN KEY (role_name)
		REFERENCES "hedge.role" (name)
);

Update hedge.resources set ui_id = 'digitwin' where name = 'digitalTwinDataSimulation'
Update hedge.role_resource_access set resources_name = '/hedge-node-red'
Update hedge.user_roles set role_name = 'dashboard-editor' where user_name = 'kong' and role_name = 'workflow-editor'
Update hedge.user_roles set user_name = 'kong1' where role_name = 'dashboard-editor'; //"dashboard-viewer"
---------------------Delete --------------
Delete from hedge.role_resource_access where resources_name = 'grafana-dashboard';
Delete from hedge.user_roles where role_name = 'dashboard-viewer' and user_name = 'kong'

---
CREATE TABLE IF NOT EXISTS hedge."user_preference" (
   user_name VARCHAR(50) UNIQUE NOT NULL,
   resource_name VARCHAR(50),
   created_on TIMESTAMP NOT NULL,
   created_by VARCHAR(50) NOT NULL,
   modified_on TIMESTAMP,
   modified_by VARCHAR(50),
   FOREIGN KEY (user_name)
		REFERENCES hedge."user" (name)
		ON DELETE CASCADE,
   FOREIGN KEY (resource_name)
		REFERENCES hedge."resources" (name)
		ON DELETE SET NULL
);
INSERT INTO hedge.user_preference(
    user_name, resource_name, created_on, created_by)
VALUES ('kong', 'mainmenu', current_timestamp, 'admin')
    ON CONFLICT DO NOTHING;