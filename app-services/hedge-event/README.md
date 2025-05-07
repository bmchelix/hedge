# Hedge Event

hedge-event service is the northbound application service that stores the HedgeEvent or Alerts to open-search database
The input in here is the common.models.HedgeEvent published to hedge/events topic in MQTT
The event in here is published by event-publisher service after de-deplication etc.
