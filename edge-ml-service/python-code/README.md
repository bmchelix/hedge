## ML Models for HEDGE 
This repository contains various machine learning models and utilities under the python-code directory. The structure is designed to be modular, so each model type (e.g., anomaly detection, classification, clustering, regression, etc.) is contained within its own subdirectory. Common utilities or helper code can be found under common and base-image.

## Prerequisites (Python Version)
- Python 3.8+ or higher
- Docker (if you plan to build and run containers locally).
- [Optional] Virtual environment tool (such as venv or conda) if you prefer to isolate dependencies.

## Installation (For Any of the Model)
1. Clone the repository:

```bash
git clone https://github.bmc.com/CTO-BIL/edge-iot.git
   
cd edge-iot/edge-ml-service/python-code
   ```
2. Create and activate a virtual environment (recommended):
```bash
python -m venv venv

source venv/bin/activate  # Linux/Mac
# or
venv\Scripts\activate     # Windows
```
3. Install dependencies (For local testing) 
```bash
pip install -r anomaly/autoencoder/train/requirements.txt
```

**Folder Structure**

```
python-code
├── anomaly
│   ├── autoencoder
│       ├── train
│       │   └── src
│       │       └── main
│       │           └── task.py
│       ├── infer
│       │   └── src
│       │       └── main
│       │           └── task.py
│       └── tests
│           └── test_data
│           └── models
├── base-image
│       └── Dockerfile
├── byoa-pkg
│       └── ...
├── classification
│   └── random_forest
│       └── ...
├── clustering
│   └── gaussian_mixture_model
│       └── ...
├── common
│   ├── src
│       ├── data
│       │   ├── impl
│       │   │   └── cloud_training_util.py
│       │   │   └── local_training_util.py
            └──hedge_util.py
│       ├── ml
│       │   └── hedge_api.py
│       │   └── hedge_inference.py
│       │   └── hedge_status.py
            └── hedge_training.py
            └── watcher.py
│       └── util
│           └── config_extractor.py
│           └── custom_encoder.py
│           └── data_transformer.py
│           └── env_util.py
│           └── exceptions.py
│           └── infer_exception.py
│           └── logger_util.py      
├── regression
│   └── lightgbm
│       └── ...
├── simulation
│   └── forecasting
│       └── ...
├── tests
│       └── ...
└── timeseries
    └── multivariate_deepVAR
        └── ...

```

**Details** 
* _anomaly_: Contains code for anomaly detection (e.g., autoencoder-based training and inference).
* _base-image_: Docker base images or foundational components used across different models.
* _byoa-pkg_: “Bring Your Own Algorithm” package or custom user-defined algorithms.
* _classification_: Classification models (Random Forest).
* _clustering_: Clustering models (Gaussian Mixture Model).
* _common_: Shared libraries, helper functions, or utility scripts used by multiple modules.
* _regression_: Regression models (LightGBM).
* _simulation_: Simulation models (WIP) 
* _tests_: Central location for test suites (though some modules also have local tests).
* _timeseries_: Time-series analysis and forecasting models.


<br>

## How to run local testing 

Below are instructions to test the anomaly detection code without Docker, running locally on your machine.

1. Training Data File Placement

Place the training data zip file under /test_data folder. For example: 
```bash
edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data
└── training_data.zip

```

2. Update `env.yaml` File

Update env.yaml with the fully qualified paths for the following attributes:

``` 
OutputDir: <LOCAL_PATH>/edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/models
training_file_id: <LOCAL_PATH>/edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data/60cb7d10-4c8c-429c-9063-2ec0e609c920.zip
ModelDir: <LOCAL_PATH>/edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/models
Local: True
```

3. Test Anomaly Detection - Train

* Change directory

```
cd edge-iot/edge-ml-service/python-code
```

* Run the training script: 

```
python anomaly/autoencoder/train/src/main/task.py
```

* Monitor the console output to verify the training steps and confirm the model is saved successfully.

4. Test Anomaly Detection - Infer

* Change directory

```
cd edge-iot/edge-ml-service/python-code
```

* Run the training script: 

```
python anomaly/autoencoder/infer/src/main/task.py
```

* Check the logs or console output to confirm the inference logic is running correctly.

<br>


## How to Run Local Docker Testing (For Anomaly Detection)

Below are instructions specifically for testing the Anomaly Detection module (anomaly/autoencoder). Adjust paths or environment variables as needed.

### Training Data File Placement
```
edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data

```
For example:
```
edge-iot/edge-ml-service/python-code/anomaly/autoencoder/train/src/main/tests/test_data
└── training_data.zip

```

### Test Anomaly Detection - Train
1. Navigate to the python-code folder:

```
cd edge-iot/edge-ml-service/python-code
```
2. Build the Docker image
```
docker build --no-cache --label "Name=anomaly-detection-train" \
    -t anomaly-detection-train:latest \
    -f anomaly/autoencoder/train/Dockerfile .

```
3. Run the container
```
docker run -e LOCAL=True \
           -e MODELDIR=/anomaly-api/res/edge/models \
           -e OUTPUTDIR=/anomaly-api/res/edge/models \
           -e TRAINING_FILE_ID=/anomaly-api/res/edge/test_data/60cb7d10-4c8c-429c-9063-2ec0e609c920.zip \
           -d \
           -v ${PWD}/anomaly/autoencoder/train/src/main/tests:/anomaly-api/res/edge \
           anomaly-detection-train:latest \
           bash


```
4. Monitor the logs 
```
docker ps  # find the running container ID
docker logs <container_id>
```



### Test Anomaly Detection - Train
1. Navigate to the python-code folder:

```
cd edge-iot/edge-ml-service/python-code
```
2. Build the Docker image
```
docker build --no-cache  --label "Name=anomaly-detection-infer" \
    -t anomaly-detection-infer:latest \
    -f anomaly/autoencoder/infer/Dockerfile .

```
3. Run the container
```
docker run -p 48096:48096 \
           -e LOCAL=True \
           -e MODELDIR=/anomaly-api/res/edge/models \
           -e OUTPUTDIR=/anomaly-api/res/edge/models \
           -d \
           -v ${PWD}/anomaly/autoencoder/train/src/main/tests:/anomaly-api/res/edge \
           anomaly-detection-infer:latest 



```
4. Once the container is up, you can verify the inference service is running:
* In a browser, go to: http://localhost:48096/docs
* You should see the API documentation if everything is set up correctly.
