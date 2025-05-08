#!/bin/sh

echo -e "Waiting for Hedge Admin to be up";
# 'getent' for DNS check, 'nc' for port availability check
while ! ( getent hosts hedge-admin > /dev/null &&
          nc -z hedge-admin 48098 );
do sleep 5;
  echo -e "still waiting for Hedge Admin...";
done;
echo -e "  >> Hedge Admin has started. Continuing ahead..";

#### Validation ends ####
##########################

# Create sample Protocols for device-virtual service
#curl --location 'hedge-admin:48098/api/v2/node_mgmt/deviceservice/protocols' \
#--header 'Content-Type: application/json' \
#--data '{
#    "device-virtual": [
#        {
#            "protocolName": "VirtualProtocol1",
#            "protocolProperties": [
#                "VirtualParam1",
#                "VirtualParam2"
#            ]
#        },
#        {
#            "protocolName": "VirtualProtocol2",
#            "protocolProperties": [
#                "VirtualParam2.1",
#                "VirtualParam2.2"
#            ]
#        }
#    ]
#}'

echo -e "\n\n"
#Sync all device service protocols in to Redis
curl --location --request POST 'hedge-admin:48098/api/v3/node_mgmt/deviceservice/initialize/all' \
--header 'Content-Type: application/json' \
--data ''
