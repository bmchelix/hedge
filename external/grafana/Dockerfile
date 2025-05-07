# (c) Copyright 2020-2025 BMC Software, Inc.
#
# Contributors: BMC Software, Inc. - BMC Helix Edge

ARG GRAFANA_VERSION
FROM grafana/grafana:${GRAFANA_VERSION}

ARG GF_UID="472"
ARG GF_GID="0"

ENV PATH="/usr/share/grafana/bin:$PATH" \
  GF_PATHS_CONFIG="/etc/grafana/grafana.ini" \
  GF_PATHS_DATA="/var/lib/grafana" \
  GF_PATHS_HOME="/usr/share/grafana" \
  GF_PATHS_LOGS="/var/log/grafana" \
  GF_PATHS_PLUGINS="/var/lib/grafana/plugins" \
  GF_PATHS_PROVISIONING="/etc/grafana/provisioning"

#BMC Addition
ADD contents/provisioning/datasources/datasources.yaml                      "$GF_PATHS_PROVISIONING/datasources/bmc-datasources.yaml"
ADD contents/provisioning/dashboards/dashboards.yaml                        "$GF_PATHS_PROVISIONING/dashboards/bmc-dashboards.yaml"
ADD contents/resources/dashboards/                                          "$GF_PATHS_DATA/resources/dashboards/"
#ADD contents/resources/xxx-branding/                                        "$GF_PATHS_HOME/public/img/"
#ADD contents/resources/images/                                              "$GF_PATHS_HOME/public/img/images/"
ADD contents/resources/css/index.html                                       "$GF_PATHS_HOME/public/views/"

#Copy customized grafana configuration
ADD contents/resources/config/grafana.ini                                   "$GF_PATHS_CONFIG"

# Temporary copy in here as well to get the sample dashboards to work, the above is for fleet dashboards
#ADD contents/resources/images/                                              "$GF_PATHS_HOME/public/img/plugins/"

################################### Panel plugins ######################################
#XXX Custom plugins - Maps, Anomaly, Ticket Viewer
#Telco map (id: panodata-map-panel-sf)
#ADD contents/custom-plugins/_builds/panodata-map-panel.tgz                  "$GF_PATHS_PLUGINS/telco-tower-map"
#ADD contents/custom-plugins/maps/diffs/panodata-map-panel-sf/src/*          "$GF_PATHS_PLUGINS/telco-tower-map/src/"
#ADD contents/custom-plugins/maps/diffs/panodata-map-panel-sf/dist/*         "$GF_PATHS_PLUGINS/telco-tower-map/dist/"

#Wind Farms map (id: panodata-map-panel-usa)
#ADD contents/custom-plugins/_builds/panodata-map-panel.tgz                  "$GF_PATHS_PLUGINS/panodata-map-panel-usa"
#ADD contents/custom-plugins/maps/diffs/panodata-map-panel-usa/src/*         "$GF_PATHS_PLUGINS/panodata-map-panel-usa/src/"
#ADD contents/custom-plugins/maps/diffs/panodata-map-panel-usa/dist/*        "$GF_PATHS_PLUGINS/panodata-map-panel-usa/dist/"

#Wind Towers Map (id: panodata-map-panel-ca-hills)
#ADD contents/custom-plugins/_builds/panodata-map-panel.tgz                  "$GF_PATHS_PLUGINS/windmills-farm-map"
#ADD contents/custom-plugins/maps/diffs/panodata-map-panel-ca-hills/src/*    "$GF_PATHS_PLUGINS/windmills-farm-map/src/"
#ADD contents/custom-plugins/maps/diffs/panodata-map-panel-ca-hills/dist/*   "$GF_PATHS_PLUGINS/windmills-farm-map/dist/"

#Anomaly Custom Panel
#ADD contents/custom-plugins/_builds/anomaly-custom-panel.tgz                "$GF_PATHS_PLUGINS/anomaly-custom-panel"
#ADD contents/custom-plugins/anomaly-custom-panel/src/plugin.json            "$GF_PATHS_PLUGINS/anomaly-custom-panel/dist/plugin.json"
#ADD contents/custom-plugins/anomaly-custom-panel/src/SimplePanel.tsx        "$GF_PATHS_PLUGINS/anomaly-custom-panel/src/SimplePanel.tsx"
#ADD contents/custom-plugins/anomaly-custom-panel/dist/module.js             "$GF_PATHS_PLUGINS/anomaly-custom-panel/dist/module.js"

#Ticket Viewer Panel
#ADD contents/custom-plugins/_builds/ticket-viewer-panel.tgz                 "$GF_PATHS_PLUGINS/ticket-viewer-panel"
#ADD contents/custom-plugins/ticket-viewer-panel/src/plugin.json             "$GF_PATHS_PLUGINS/ticket-viewer-panel/dist/plugin.json"
#ADD contents/custom-plugins/ticket-viewer-panel/src/SimplePanel.tsx         "$GF_PATHS_PLUGINS/ticket-viewer-panel/src/SimplePanel.tsx"
#ADD contents/custom-plugins/ticket-viewer-panel/dist/module.js              "$GF_PATHS_PLUGINS/ticket-viewer-panel/dist/module.js"

#Panel Digital Twin
#ADD contents/custom-plugins/_builds/panel-digital-twin.tgz                 "$GF_PATHS_PLUGINS/panel-digital-twin"
#ADD contents/custom-plugins/panel-digital-twin/src/plugin.json             "$GF_PATHS_PLUGINS/panel-digital-twin/dist/plugin.json"
#ADD contents/custom-plugins/panel-digital-twin/src/module.ts               "$GF_PATHS_PLUGINS/panel-digital-twin/src/module.ts"
#ADD contents/custom-plugins/panel-digital-twin/dist/module.js              "$GF_PATHS_PLUGINS/panel-digital-twin/dist/module.js"


#ade-panel-record-details is the ADE plugin for showing record details
#ADD contents/custom-plugins/_builds/ade-panel-record-details.tgz            "$GF_PATHS_PLUGINS/ade-panel-record-details"

#Track-it customized plugin to track vehicles and fuelling stations in the map
#ADD contents/custom-plugins/_builds/track-it.tgz                            "$GF_PATHS_PLUGINS/track-it"

################################### Datasource plugins ######################################
#bmc-dtwin-datasource for Hedge visualization digital twin
#ADD contents/custom-plugins/_builds/bmc-dtwin-datasource.tgz                "$GF_PATHS_PLUGINS/bmc-dtwin-datasource"

WORKDIR $GF_PATHS_HOME

EXPOSE 3000

USER grafana
ENTRYPOINT [ "/run.sh" ]
