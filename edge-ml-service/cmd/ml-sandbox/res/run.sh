#!/bin/sh

# 1. Change the image name and tag to latest version - hedge-ml-sandbox:25.2.00_latest

# 2. Change the local volume mapping where the training file directory exists - LOCAL_VOLUME_MAPPING

# 3. Place the common_configuration_hedge.yaml file in the LOCAL_VOLUME_MAPPING directory

# 4. Run this script from root cd ~/git/edge-iot  and  ./edge-ml-service/cmd/ml-sandbox/res/run.sh


rm hedge_ml_sandbox

make hedge_ml_sandbox

docker tag hedge-ml-sandbox:25.2.00_latest pun-harbor-reg1.bmc.com/iot/hedge-ml-sandbox:25.2.00_latest

docker run \
  --name hedge-ml-sandbox \
  --hostname hedge-ml-sandbox \
  -e SERVICE_HOST=hedge-ml-sandbox \
  -e EDGEX_SECURITY_SECRET_STORE=false \
  -e SECRETSTORE_DISABLESCRUBSECRETSFILE=false \
  -e TRIGGER_EXTERNALMQTT_AUTHMODE=none  \
  -e SECRETSTORE_SECRETSFILE=/tmp/hedge-secrets/hedge_ml_sandbox_secrets.json \
  -e EDGEX_COMMON_CONFIG=/res/jobs/common_configuration_hedge.yaml \
  -e WRITABLE_INSECURESECRETS_REGISTRY_SECRETDATA_USERNAME=bilhedge \
  -e WRITABLE_INSECURESECRETS_REGISTRY_SECRETDATA_PASSWORD='7IeL8av#$[U12igKQ2Q85f^.:' \
  -e TRIGGER_EXTERNALMQTT_URL=tcp://host.docker.internal:1883 \
  -e APPLICATIONSETTINGS_JOBDIR=/tmp/jobs/ \
  -e APPLICATIONSETTINGS_IMAGEREGISTRY=pun-harbor-reg1.bmc.com/iot/ \
  -v edgex-init:/edgex-init \
  -v LOCAL_VOLUME_MAPPING:/res/jobs \
  -v hedge-secrets:/tmp/hedge-secrets \
  -v /tmp/edgex/secrets/app-hedge-ml-sandbox:/tmp/edgex/secrets/app-hedge-ml-sandbox:ro,z \
  --privileged \
  --user root:root \
  --rm \
  pun-harbor-reg1.bmc.com/iot/hedge-ml-sandbox:${HEDGE_ML_SANDBOX_DOCKER_TAG:-25.2.00_latest} \
  sh -c "dockerd && sleep 15 && /hedge-ml-sandbox --registry --configDir=/res"