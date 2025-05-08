

## Docker Commands - Local Training 
docker build --no-cache --label "Name=timeseries-train" -t timeseries-train:latest -f timeseries/multivariate_deepVAR/train/Dockerfile . 

docker run -e LOCAL=True -e JOBDIR=/anomaly-api/res/edge/models -e OUTPUTDIR=/anomaly-api/res/edge/models -e TRAINING_FILE_ID=/anomaly-api/res/edge/test_data/deepVAR-TimeSeries_new3.zip -v ${PWD}/timeseries/multivariate_deepVAR/tests:/anomaly-api/res/edge timeseries-train:latest


## Docker Commands - Local Inference
docker build --label "Name=timeseries-infer" -t timeseries-infer:latest -f timeseries/multivariate_deepVAR/infer/Dockerfile . 

docker run -p 55000:55000 -e LOCAL=True -e JOBDIR=/anomaly-api/res/edge/models -e MODELDIR=/anomaly-api/res/edge/models -v ${PWD}/timeseries/multivariate_deepVAR/tests:/anomaly-api/res/edge timeseries-infer:latest


Make sure to be at this current working directory 
`cd /edge-iot/edge-ml-service/python-code` 
