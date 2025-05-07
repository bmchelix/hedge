#!/bin/sh

#set -x

#Make sure following environment variables are correctly set (typically set during deployment):
  # CURRENT_HEDGE_NODE_TYPE
  # CURRENT_HEDGE_NODE_SERVER_NAME
  # CURRENT_HEDGE_NODE_ID
  # IS_EXTERNAL_AUTH
  # MQTT_USERNAME
  # For SSL : CERT_PATH, KEY_PATH, DOMAIN

echo -e "\n\n========================================"
echo -e     "========== hedge_init started =========="
echo -e "Start time: $(date)\n--"

export NODE_TYPE=$CURRENT_HEDGE_NODE_TYPE
export EXECUTION_MODE=$EXECUTION_MODE # Valid execution modes are: SEED_SECRETS, GET_SECRETS, UPDATE_SECRETS, POST_PROCESSING

echo "VERSION=$VERSION"
echo "NODE_TYPE=$CURRENT_HEDGE_NODE_TYPE"
echo "DATASTORE_PROVIDER=$DATASTORE_PROVIDER"
echo "EXECUTION_MODE=$EXECUTION_MODE"
echo "COMPOSE_PROFILES=$COMPOSE_PROFILES"
echo "MQTT_USERNAME=$MQTT_USERNAME"
echo "MQTT_ENCPWD_INP_FILE=$MQTT_ENCPWD_INP_FILE"
echo "CNSL_ENCTKN_INP_FILE=$CNSL_ENCTKN_INP_FILE"
echo "NATS_TLS_CRT=$NATS_TLS_CRT"
echo "NATS_TLS_KEY=$NATS_TLS_KEY"

# Content Deployment
deploy_contents() {
  echo -e ">>>>>>>>> About to deploy_contents(): <<<<<<<<< \n"
  ./hedge-scripts/content-deploy.sh
  echo -e "\n\n========================================\n"
}

# Creating Elastic Index
create_elasticindex() {
  echo -e ">>>>>>>>> About to create_elasticindex: <<<<<<<<< \n"
  ./hedge-scripts/create_elastic_index.sh
  echo -e "\n\n========================================\n"
}

# Register ML alogorithm as seed data, ml-management needs to have been deployed at core before this
register_ml_algo() {
  echo -e ">>>>>>>>> About to register_ml_algo: <<<<<<<<< \n"
  ./hedge-scripts/register-ml-algo.sh
  echo -e "\n\n========================================\n"
}

# Creating Elastic Index
create_default_nodes() {
  echo -e ">>>>>>>>> About to create_default_nodes: <<<<<<<<< \n"
  echo "CURRENT_HEDGE_NODE_SERVER_NAME=$CURRENT_HEDGE_NODE_SERVER_NAME"
  echo "CURRENT_HEDGE_NODE_ID=$CURRENT_HEDGE_NODE_ID"
  ./hedge-scripts/create-default-node.sh "$CURRENT_HEDGE_NODE_ID" "$CURRENT_HEDGE_NODE_SERVER_NAME"
  echo -e "\n\n========================================\n"
}

# Create Service Protocols
create_service_protocols() {
  echo -e ">>>>>>>>> About to create_service_protocols: <<<<<<<<< \n"
  ./hedge-scripts/create-service-protocols.sh
  echo -e "\n\n========================================\n"
}

bootstrap_hedge_secrets() {
  echo -e ">>>>>>>>> About to bootstrap_hedge_secrets: <<<<<<<<< \n"
  ./hedge-scripts/seed-hedge-secrets.sh
  return $?
}

echo -e "Running hedge-init in $EXECUTION_MODE mode"
if [ "$EXECUTION_MODE" = "SEED_SECRETS" ] || [ "$EXECUTION_MODE" = "GET_SECRETS" ] || [ "$EXECUTION_MODE" = "UPDATE_SECRETS" ]; then
  bootstrap_hedge_secrets
  ret=$?

  echo -e "Exiting hedge_init with return value: $ret\n"
  exit $ret
fi

case "$NODE_TYPE" in
  *CORE*|*NODE*)
    #CORE setup
    if [[ "$NODE_TYPE" == "*CORE*" ]]; then #CORE
      register_ml_algo
      create_default_nodes
      create_elasticindex
    fi

    #NODE setup
    if [[ "$NODE_TYPE" == "*NODE*" ]]; then #NODE
      create_service_protocols
      deploy_contents
    fi
    ;;

  *)
    echo -e "ERROR: Invalid NODE_TYPE: $NODE_TYPE"
    exit 1
    ;;
esac

echo -e "Init completed. Pausing..."

# pause execution to avoid container from exiting - useful for debugging
sleep infinity & wait

echo -e "Exiting with success"
exit 0
