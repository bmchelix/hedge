# hedge export
This application service built using edgex foundry app sdk is responsible to export machine data to a long term persistent store.
In our case this persistent storage is Victoria Matric for metric/timeseries data and open-search for Alerts/events.
The data being exported also includes predictions.
While the metric data is received in hedge/metrics topic, the prediction data is received in hedge/ml/predictions.
data for hedge/metrics is published by data-enrichment service while the ml-broker is the one which publishes ML predictions to ml/predictions topic.

predictions being published is optional, however, there are use cases where we want to view the prediction trend.
As of now, anomaly score is being presented as a healthscore and we can view machine health trend.

In our deployment model, this service is deployed in the management ( aka core) layer.
