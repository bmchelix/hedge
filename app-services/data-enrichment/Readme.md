# Data Enrichment
This application service built using edgex foundry app sdk is a pipeline service that enriches the machine data that is collected by device services with additional context data.

The reason we add context data to the data pipeline is so that we can write more powerful and context aware rules & machine learning training and inferencing.
An example of a context data for fleet management is the name of a rental driver. This enables us to visualize the data by driver name and when we detect an event ( eg Harsh driving), we can report it against the driver name.
Another example is the machine data where we can add YearOfMake of the machine as a context data. 
Since machine age could be an important consideration when we train ML Model (e.g. Anomaly detection), it is useful to consider yearOfMake also as one of the features for training and inferencing.

The context data for enrichment is picked from device metadata which could be device extension attributes or context data that gets added if we have an external integration.

The target for enriched data are two data-streams:
1. One to redis to topic: hedge/events/device/#. This one is so that kuiper rules and ML inferencing continue to work on this
2. MQTT topic: BMCMetrics. This one is to export the data to northbound permanent storage so we can view in grafana dashboard

For #2 above, there is a configurable option wherein we only export downsampled or aggregated data.
This configuration can be accessed from UI.

Since this service is meant to handle high load, care has been taken for performance and concurrency. The device tags are cached for enrichment. Whenever there is a change in device meta data or profile metadata, the cache needs to be refreshed.
Support notification from edgex is used to received such cache update notifications based on which data-enrichment service refreshes its cache.
