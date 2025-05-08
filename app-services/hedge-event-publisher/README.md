# Event Publisher

Events Publisher service publishes events that can be used to trigger workflows or to visualize in a dashboard. It is developed using edgex foundry App Functions SDK.
In general, Events ( refer common.model.Events) can be published directly to MQTT. However, the issue with this approach is we end up publishing
multiple events for the same issue (eg using Kuiper rules) and then when we automatically want to close this open event, we need a correlation-i.
This logic is part of event-publisher implementation.

## Overview

The service pipeline is triggered by the rule and the pipeline ensures that we transform the rule output to Events data-structure and then eventPublisher to Events topic of MQTT as well as persist to Event store which is distro Elastic
You can trigger this pipeline by using the http trigger from a tool like postman and sending the rule output in the JSON body as payload. 
