# hedge remediate
This service subscribes to hedge/command topic of MQTT and based on the command it receives in tis topic
it executes the command. At present, the supported command type is "DeviceCommand"
Every command can have parameters, so the generic way to pass the parameters is via command_parameters

