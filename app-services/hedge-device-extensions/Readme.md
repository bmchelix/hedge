# Device Extensions
This service extends edgex core-metadata service to add the following additional features:
1. Device Attributes which is the ability to define additional attributes in a profile and allow the user to 
add the value in device
2. Contextual Attribute which is the business data that is associated with an instance of a device and that can change over time. Example of this could be a driver of a rental vehicle
  This data is ingested via a rest call exposed via export-biz-data service and synced using meta-sync. So we can use this context data is configurations, the meta data ie name of the contextual attribute is also added to profile
3. Associations wherein a generic parent child kind of relationship gets added
3. Location of the device

Since the above data is on top of what edgex-core-metadata service provides, they are stored separately. However, to get the complete device or profile data, this service provides wrapper REST APIs
that allows us to get the complete data about a profile or a given device

If we refer to our deployment model across nodes(edge ) and management layer(core), it requires that the core has data across all devices/profiles across all nodes.
So we will also find places in the code that notifies the changes so meta-sync service can sync the meta-data.
