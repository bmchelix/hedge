INSERT INTO hedge.resources( name, display_name, uri, link_type, ui_id, active, parent_resource, allowed_permissions, created_on, created_by, modified_on, modified_by)
VALUES ( 'Airstation_List', 'US Coast Guard Demo', '/hedge/hedge-grafana/d/AirstationMap/airstation-list?kiosk', 'frame', 'Airstation_List?kiosk=os', TRUE, 'analytics', 'RW', current_timestamp, 'admin', NULL, NULL);

INSERT INTO hedge.resources( name, display_name, uri, link_type, ui_id, active, parent_resource, allowed_permissions, created_on, created_by, modified_on, modified_by)
VALUES ( 'US_Airforce_Base_Stations', 'US Airforce Demo', '/hedge/hedge-grafana/d/FoRcEbaSe/us-airforce-base-stations', 'frame', 'US_Airforce_Base_Stations', TRUE, 'analytics', 'RW', current_timestamp, 'admin', NULL, NULL);

INSERT INTO hedge.resources( name, display_name, uri, link_type, ui_id, active, parent_resource, allowed_permissions, created_on, created_by, modified_on, modified_by)
VALUES ( '3dPrinters', '3D Printer Demo', '/hedge/hedge-grafana/d/d75f92f4-fd85-4758-9af9-e21f145fd3b4/site-view', 'frame', '3dPrinters', TRUE, 'analytics', 'RW', current_timestamp, 'admin', NULL, NULL);

INSERT INTO hedge.resources( name, display_name, uri, link_type, ui_id, active, parent_resource, allowed_permissions, created_on, created_by, modified_on, modified_by)
VALUES ( 'Next_Gen_Green_Energy_Ltd', 'Windmill Demo', '/hedge/hedge-grafana/d/CiXtTqyGk/next-gen-green-energy-ltd', 'frame', 'Next_Gen_Green_Energy_Ltd', TRUE, 'analytics', 'RW', current_timestamp, 'admin', NULL, NULL);

INSERT INTO hedge.resources( name, display_name, uri, link_type, ui_id, active, parent_resource, allowed_permissions, created_on, created_by, modified_on, modified_by)
VALUES ( 'Tower_Map', 'Telecom Demo', '/hedge/hedge-grafana/d/uRYgTqsMk/tower-map', 'frame', 'Tower_Map', TRUE, 'analytics', 'RW', current_timestamp, 'admin', NULL, NULL);

INSERT INTO hedge.resources( name, display_name, uri, link_type, ui_id, active, parent_resource, allowed_permissions, created_on, created_by, modified_on, modified_by)
VALUES ( 'Manufacturing_plant', 'Manufacturing Demo', '/hedge/hedge-grafana/d/7xEoo0pSz/manufacturing-plant', 'frame', 'Manufacturing_plant', TRUE, 'analytics', 'RW', current_timestamp, 'admin', NULL, NULL);

INSERT INTO hedge.resources( name, display_name, uri, link_type, ui_id, active, parent_resource, allowed_permissions, created_on, created_by, modified_on, modified_by)
VALUES ( 'Water_Cooler_Map', 'Water Cooler Demo', '/hedge/hedge-grafana/d/J9u3kswGz/water-cooler-map', 'frame', 'Water_Cooler_Map', TRUE, 'analytics', 'RW', current_timestamp, 'admin', NULL, NULL);


INSERT INTO hedge.role_resource_permission( role_name, resources_name, permission)
VALUES ( 'DashboardUser', 'Airstation_List', 'RW');

INSERT INTO hedge.role_resource_permission( role_name, resources_name, permission)
VALUES ( 'DashboardUser', 'US_Airforce_Base_Stations', 'RW');

INSERT INTO hedge.role_resource_permission(role_name, resources_name, permission)
VALUES('PlatformAdmin', '3dPrinters', 'RW');
INSERT INTO hedge.role_resource_permission(role_name, resources_name, permission)
VALUES('DashboardUser', '3dPrinters', 'RW');

INSERT INTO hedge.role_resource_permission( role_name, resources_name, permission)
VALUES ( 'DashboardUser', 'Next_Gen_Green_Energy_Ltd', 'RW');

INSERT INTO hedge.role_resource_permission( role_name, resources_name, permission)
VALUES ( 'DashboardUser', 'Tower_Map', 'RW');

INSERT INTO hedge.role_resource_permission( role_name, resources_name, permission)
VALUES ( 'DashboardUser', 'Manufacturing_plant', 'RW');

INSERT INTO hedge.role_resource_permission( role_name, resources_name, permission)
VALUES ( 'DashboardUser', 'Water_Cooler_Map', 'RW');

INSERT INTO hedge."urls"(name, url, description)
VALUES('3dPrinters','/hedge/hedge-grafana/d/d75f92f4-fd85-4758-9af9-e21f145fd3b4/site-view', 'URL to support 3D Printer ');

INSERT INTO hedge."resource_urls"(resource_name, url_name, description)
VALUES('3dPrinters', '3dPrinters', '/hedge/3dPrinters');



