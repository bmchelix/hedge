# Device Virtual
The very first challenge when we start with IoT projects is around how do we get the data so we can plan the upstream workflows.
The reason this is challenging is that we need device connectivity into OT environments which takes time.
The device-virtual is designed to solve this problem and to provide enable development environment where we can
continue our development without the need for connectivity to actual physical device.
It provides two methods by which we can generate data. If we know the data values, we can ingest data using REST APIs.
However, this requires that we keep firing rest APIs via some script if we need the data continuously.
The other approach, a preferred one, is to configure the data-generation seed for each metric where we configure min and max value between which a random number is generated.
This seed data-generation configuration of defined as part of profile attribute definition.
So we can use device-virtual to iterate and plan the profile, rules and even Machine learning training and deployment.
The simulated data can be viewed in the grafana dashboard immediately. 
The following profile definition has been added that serves as example templates.
* WindTurbine
* HVAC
* Telco-Tower-OSS
* Telco-Tower
* WaterCooler
* AirplaneAviationParamsProfile
* AirplaneControlSurfacesProfile
* AirplaneLandingGearProfile

This version of Virtual Device Service is implemented based on [Device SDK GO](https://github.com/edgexfoundry/device-sdk-go),


# Docker compose file settings
Adding service:
```yaml
 hedge-virtual-device:
   profiles: ["all", "virtual", "demo"]
   command: /device-virtual -cp=consul.http://${CURRENT_HEDGE_NODE_SERVER_NAME}:8500 --registry --configDir=/res
   container_name: device-virtual
   entrypoint:
     - /tmp/hedge-scripts/hedge_device_service_wait_install.sh
   environment:
     <<: *x-edgex-env-variables
     SERVICE_HOST: device-virtual
     MESSAGEQUEUE_HOST: edgex-redis
   hostname: device-virtual
   image: ${REGISTRY}device-virtual:${DOCKER_TAG:-latest}
   networks:
     edgex-network: {}
   ports:
     - 127.0.0.1:49991:49991/tcp
   read_only: true
   restart: always
   security_opt:
     - no-new-privileges:true
   user: 2002:2001
   volumes:
     - edgex-init:/edgex-init:ro,z
     - virtual-device-data:/data
     - virtual-devices:/res/devices
     - hedge-scripts:/tmp/hedge-scripts:ro,z
     - /tmp/edgex/secrets/device-virtual:/tmp/edgex/secrets/device-virtual:ro,z
```
# How to use
There is in-built data-generation based on attribute data that we specify in the profile definition
Refer to cmd/res/profile directory for e.g. profiles. The configuration specify the min and max values and a random number
is generated between those ranges.
To simulate a different range, we have a built-in command using which we can change the min/max or avg value
This is used to simulate failures/anomaly or to test the rules
Below is an example REST API to generate the simulation data:

curl --location --request PUT 'http://localhost:49991/api/v3/dataGenerator/commands' \
--header 'Content-Type: application/json' \

--data 
'[
{
"name": "ChangeAverage",
"parameters": {
"device": "AltaNS12",
"metric": "GenGearOilLevelPercent",
"average": "28"
}
},
{
"name": "ChangeMinMax",
"parameters": {
"device": "AltaNS12",
"metric": "GenGearOilLevelPercent",
"min": "60",
"max": "70"
}
},
{
"name": "ResetValue",
"parameters": {
"device": "Alta NS12",
"metric": "GenGearOilLevelPercent"
}
},
{
"name": "Wait",
"parameters": {
"waitDuration": "20s"
}
},
{
"name": "RepeatDefaultAndNewValues",
"parameters": {
"device": "AltaNS12",
"metric": "GenGearOilLevelPercent",
"average": "48",
"repeatInterval": "1m"
}
},
{
"name": "ChangeDefault",
"parameters": {
"device": "AltaNS12",
"metric": "GenGearOilLevelPercent",
"min": "80",
"max": "85"
}
}
]'

To ingest data using REST API, here is an example curl command:

curl --location --request POST 'http://localhost:49991/api/v3/resource/AltaNS14' \
--header 'Content-Type: application/json' \
--data '{
"readings": [
{
"name": "GenGearOilLevelPercent",
"value": "38",
"origin" : 1741043700000000,
"tags": {
"yearOfMake": "1990"
}
}
]
}'

### How to generate more load for testing
If we want to generate load and test other services for load, the easiest way out is to update data collection interval(autoevents) for each of the devices.
To have multiple instances of this service running in case device-virtual doesn't scale, we will
need to compile another binary from same source code by changing the service name ( eg device-virtual_1)

Thereafter, follow the below steps:
1. Update docker-compose-node.yml by copying device-virtual section and update the following
-  Update name, SERVICE_HOST, hostname, image to: device-virtual_{n}
Donot change the command, it is still device-virtual 
2. Update volume map as below:
- virtual-devices-{n}:/res/devices
- /tmp/edgex/secrets/device--virtual{n}:/tmp/edgex/secrets/device-virtual-{n}:ro,z

- Make sure to add the new volume mapping under volume section
  >device-virtual-{n}: {}

Make a note of the above volume mapping to add new device instances and ensure the device names are different from other instances

3. Open docker-compose-common.yml and make the following updates the following env vars by adding entry for device-virtual-{n}
- ADD_KNOWN_SECRETS
- ADD_SECRETSTORE_TOKENS
- ADD_REGISTRY_ACL_ROLES

4. Since the env variables have changed, the services that use the above need to be re-created
> docker stop edgex-security-secretstore-setup edgex-core-consul
> docker rm edgex-security-secretstore-setup edgex-core-consul
5. Start the new instance of hedge-virtual-device-{n} by usual make run command
> make run ENV=node profile=virtual edgex-security-secretstore-setup edgex-core-consul hedge-virtual-device{n}


