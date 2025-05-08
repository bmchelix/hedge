#!/bin/sh

echo -e "Waiting for Hedge ML Management to be up";
# 'getent' for DNS check, 'nc' for port availability check
while ! ( getent hosts hedge-ml-management > /dev/null &&
          nc -z hedge-ml-management 48095 );
do sleep 30;
  echo -e "still waiting for Hedge ML Management...";
done;
echo -e "  >> Hedge ML Management has started. Continuing ahead..";

#### Validation ends ####
#########################

DOCKER_TAG=$VERSION
echo -e "  >> Setting prediction algorithms definition version=$DOCKER_TAG";

curl --location --request POST 'http://hedge-ml-management:48095/api/v3/ml_management/algorithm?generateDigest=true' \
--data-raw "{
    \"name\": \"AutoEncoder-Anomaly\",
    \"description\": \"Helix Edge AutoEncoder-Anomaly based Anomaly Detection Algorithm\",
    \"type\": \"Anomaly\",
    \"enabled\": true,
    \"supervised\": false,
    \"outputFeaturesPredefined\": false,
    \"trainingComputeProvider\": \"AIF\",
    \"isTrainingExternal\": false,
    \"isDeploymentExternal\": false,
    \"allowDeploymentAtEdge\": true,
    \"publishPredictionsRequired\": true,
	  \"predictionName\": \"anomaly_score\",
    \"autoEventGenerationRequired\": true,
    \"trainerImagePath\": \"hedge-ml-trg-anomaly-autoencoder:$DOCKER_TAG\",
    \"predictionImagePath\": \"hedge-ml-pred-anomaly-autoencoder:$DOCKER_TAG\",
    \"defaultPredictionEndpointUrl\": \"/api/v3/predict\",
    \"defaultPredictionPort\": 48910,
    \"predictionOutputKeys\": [
        \"prediction\"
    ],
    \"thresholdUserParameters\": {},
    \"hyperParameters\": {},
    \"isOotb\": true,
    \"supportSlidingWindow\" : true
}"

curl --location --request POST 'http://hedge-ml-management:48095/api/v3/ml_management/algorithm?generateDigest=true' \
--data "{
    \"name\": \"RandomForest-Classification\",
    \"description\": \"Helix Edge RandomForest-Classification based Classification Algorithm\",
    \"type\": \"Classification\",
    \"enabled\": true,
    \"supervised\": true,
    \"inputFeaturePredefined\": false,
    \"outputFeaturesPredefined\": false,
    \"trainingComputeProvider\": \"AIF\",
    \"isTrainingExternal\": false,
    \"isDeploymentExternal\": false,
    \"allowDeploymentAtEdge\": true,
    \"publishPredictionsRequired\": true,
	  \"predictionName\": \"classification\",
    \"autoEventGenerationRequired\": true,
    \"trainerImagePath\": \"hedge-ml-trg-classification-randomforest:$DOCKER_TAG\",
    \"predictionImagePath\": \"hedge-ml-pred-classification-randomforest:$DOCKER_TAG\",
    \"defaultPredictionEndpointUrl\": \"/api/v3/predict\",
    \"defaultPredictionPort\": 48920,
    \"predictionOutputKeys\": [
        \"prediction\"
    ],
    \"thresholdUserParameters\": {},
    \"hyperParameters\": {},
    \"isOotb\": true,
    \"supportSlidingWindow\" : false
}"

curl --location --request POST 'http://hedge-ml-management:48095/api/v3/ml_management/algorithm?generateDigest=true' \
--data "{
   \"name\": \"Multivariate-DeepVar-TimeSeries\",
    \"description\": \"Helix Edge Multivariate-DeepVar-TimeSeries based Timeseries Algorithm\",
    \"type\": \"Timeseries\",
    \"enabled\": true,
    \"supervised\": true,
    \"outputFeaturesPredefined\": false,
    \"trainingComputeProvider\": \"AIF\",
    \"isTrainingExternal\": false,
    \"isDeploymentExternal\": false,
    \"allowDeploymentAtEdge\": true,
	  \"predictionName\": \"timeseries_forecast\",
    \"publishPredictionsRequired\": true,
    \"autoEventGenerationRequired\": true,
    \"trainerImagePath\": \"hedge-ml-trg-timeseries-multivariate-deepvar:$DOCKER_TAG\",
    \"predictionImagePath\": \"hedge-ml-pred-timeseries-multivariate-deepvar:$DOCKER_TAG\",
    \"defaultPredictionEndpointUrl\": \"/api/v3/predict\",
    \"defaultPredictionPort\": 48930,
    \"predictionOutputKeys\": [
        \"prediction\"
    ],
    \"thresholdUserParameters\": {},
    \"hyperParameters\": {
        \"confidence_min\": 0.5,
        \"confidence_max\": 0.9
    },
    \"isOotb\": true,
    \"supportSlidingWindow\" : false
}"

curl --location --request POST 'http://hedge-ml-management:48095/api/v3/ml_management/algorithm?generateDigest=true' \
--data "{
   \"name\": \"LightGBM-Regression\",
    \"description\": \"Helix Edge LightGBM-Regression based Regression Algorithm\",
    \"type\": \"Regression\",
    \"enabled\": true,
    \"supervised\": true,
    \"outputFeaturesPredefined\": false,
    \"trainingComputeProvider\": \"AIF\",
    \"isTrainingExternal\": false,
    \"isDeploymentExternal\": false,
    \"allowDeploymentAtEdge\": true,
	  \"predictionName\": \"prediction_current\",
    \"publishPredictionsRequired\": true,
    \"autoEventGenerationRequired\": true,
    \"trainerImagePath\": \"hedge-ml-trg-regression-lightgbm:$DOCKER_TAG\",
    \"predictionImagePath\": \"hedge-ml-pred-regression-lightgbm:$DOCKER_TAG\",
    \"defaultPredictionEndpointUrl\": \"/api/v3/predict\",
    \"defaultPredictionPort\": 48940,
    \"predictionOutputKeys\": [
        \"prediction\"
    ],
    \"thresholdUserParameters\": {},
    \"hyperParameters\": {},
    \"isOotb\": true,
    \"supportSlidingWindow\" : false
}"
