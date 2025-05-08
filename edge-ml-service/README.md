# Hedge Anomaly Detection
BMC Edge ML Service

## How to Run Local Docker Testing 

<h3> (1) Test Both Inferencing & Autoencoder Together </h3>

- If you would like to test the autoencoder / inferencing using a local docker container, you can do so by running this command from this (edge-iot/edge-ml-service) directory: 

> /bin/bash ./run_docker_test.sh
 
- This will build the docker compose file and automatically run the inferencing after the autoencoder so that it picks up the output model from the autoencoder and uses that in the inferencing.

<h3> (2) Test Autoencoder Alone </h3>

- If you would like to test just the autoencoder, navigate to (edge-iot/edge-ml-service/anomaly-gcp-trng/autoencoderv2/scripts) folder and then run: 

> docker-compose down --rmi all && docker-compose build --no-cache && docker-compose up -d

- You can then check the logs in the docker image once its built and confirm that it is working through the logs. It should have trained a full model which you can then find the output of in the files. 

<h3> (3) Test Inferencing Alone </h3>

- If you would like to test just the inferencing, navigate to (edge-iot/edge-ml-service/cmd/ml-anomaly-inferencing-python/scripts) folder and then run: 

> ./run_docker_inf.sh

- Once the image has built you can exec inside of it and test that it is working by running:

> python test_api_calls2.py

- The output should look similar to the below if its working:

> INFERENCING CHECK
> 
> 226982.2067533636
> 
> 226982.2067533636
> 
> REINITIALIZATION CHECK
> 
> 200
> 
> {'message': 'Reinitialization Successful', 'model_loaded': ['models', 'config', 'normalization']}

- Reinitalization Check must be status 200 if its working, otherwise it is not
