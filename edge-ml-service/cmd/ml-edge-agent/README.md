## Purpose

The ML Edge Agent:
 - Recieves mesages from the Core that a new model has been deployed
 - Downloads and saves the model to the `res/models` directory
 - Communicates with the ML Broker that a new model has been deployed

### Testing
To test the MQTT connection locally:
 - Run the MQTT Subscriber `mosquitto_sub -t ModelDeploy -h clm-pun-uojx84`
 - Go to the site `http://clm-pun-uojx84:8000/hedge/hedge-node-red/#flow/a7eada35.6ec468` and run the trigger that is labeled `Publish to Edge Agent`. 
