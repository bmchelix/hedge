# Hedge Anomaly Detection

## How to Run Local Docker Testing 


<h3> (1) Training data file placement </h3>

- Make sure the training data zip file is placed under the (edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data) folder in the below hierarchy. 

For e.g, 
  * edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data
    * training_data.zip (Replace this file with the necessary file) 


<h3> (2) Test Anomaly Detection - Train </h3>

- If you would like to test just the anomaly detection training, navigate to (edge-iot/edge-ml-service/python-code) folder and then run the 2 commands one after the other: 

> docker build --no-cache --label "Name=anomaly-detection-train" -t anomaly-detection-train:latest -f anomaly/autoencoder/train/Dockerfile . 

> docker run -e LOCAL=True -e JOBDIR=/anomaly-api/res/edge/models -e OUTPUTDIR=/anomaly-api/res/edge/models -e TRAINING_FILE_ID=/anomaly-api/res/edge/test_data/60cb7d10-4c8c-429c-9063-2ec0e609c920.zip -d -v ${PWD}/anomaly/autoencoder/train/src/main/tests:/anomaly-api/res/edge anomaly-detection-train:latest bash

- You can then check the logs in the docker image once its built and confirm that it is working through the logs. It should have trained a full model which you can then find the output of in the files. 

<h3> (3) Test Anomaly Detection - Inferencing  </h3>

- If you would like to test just the inferencing, navigate to (edge-iot/edge-ml-service/python-code) folder and then run: 

> docker build --no-cache  --label "Name=anomaly-detection-infer" -t anomaly-detection-infer:latest -f anomaly/autoencoder/infer/Dockerfile . 

> docker run -p 48096:48096 -e LOCAL=True -e JOBDIR=/anomaly-api/res/edge/models -e OUTPUTDIR=/anomaly-api/res/edge/models -d -v ${PWD}/anomaly/autoencoder/train/src/main/tests:/anomaly-api/res/edge anomaly-detection-infer:latest bash


Once the image has built you can exec inside of it and test that it is working by running this on browser:

> http://localhost:48096/docs



## How to Run Local Testing

<h3> (1) Training data file placement </h3>

- Make sure the training data zip file is placed under the (edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data) folder in the below hierarchy. 

For e.g, 
  * edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data
    * training_data.zip (Replace this file with the necessary file) 


<h3> (2) Update env.yaml file </h3>

- Update env.yaml file with fully qualified path for the below attributes

For e.g, 
  * OutputDir - `/Users/USER_NAME/edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/models`
  * training_file_id - `/Users/USER_NAME/edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data/60cb7d10-4c8c-429c-9063-2ec0e609c920.zip`
  * JobDir - `/Users/USER_NAME/edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/models`
  * Local - `True`


<h3> (3) Test Anomaly Detection - Train </h3>

- Change directory to this folder - `cd edge-iot/edge-ml-service/python-code`

- Run this command - `python anomaly/autoencoder/train/src/main/task.py` 

<h3> (4) Test Anomaly Detection - Infer </h3>

- Change directory to this folder - `cd edge-iot/edge-ml-service/python-code`

- Run this command - `python anomaly/autoencoder/infer/src/main/task.py`


## How to deploy to ADE Environment

WIP