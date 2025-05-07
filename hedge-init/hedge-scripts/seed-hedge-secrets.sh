#!/bin/sh

#set -x   # Uncomment for debugging
set -e    # Exit immediately if any command returns a non-zero status

### import seed-helpers.sh file with function definitions
source /hedge/hedge-scripts/seed-helpers.sh

CHANGE_SECRETS_SCRIPT='/hedge/hedge-scripts/change_hedge_secrets.sh'
MYFILE=$(basename "$0" .sh)

# Create the status file, if it does not exist
init_bootstrap_status

### if running in GET_SECRETS mode, fetch secrets and stop
if [ "$EXECUTION_MODE" = "GET_SECRETS" ]; then
  dump_mqtt_secret_file && dump_consul_token_file && dump_nats_tls_files
  return $?
fi

### if running in UPDATE_SECRETS mode, change secrets
if [ "$EXECUTION_MODE" = "UPDATE_SECRETS" ]; then
  echo CONSUL_ACL_BOOTSTRAPTOKENPATH="$CONSUL_ACL_BOOTSTRAPTOKENPATH"
  if [ ! -f "$CONSUL_ACL_BOOTSTRAPTOKENPATH" ]; then
    errlog "${LINENO}" "${MYFILE}" "Consul bootstrap token file missing: $CONSUL_ACL_BOOTSTRAPTOKENPATH"
    exit 1
  fi

  # Run change_hedge_secrets.sh script to update secrets with user prompts
  sh "$CHANGE_SECRETS_SCRIPT" || { errlog "${LINENO}" "${MYFILE}" "Failed changing secrets"; return 1; }

  # Generate service specific hedge-secrets files for persisting in to Vault
  hedge_secrets_json=$(cat "$BTSP_VAULT_SECRETS_JSONFILE")
  create_vault_secret_json "$hedge_secrets_json"
fi

# when setting up a NODE, .mqttencpwd file should be created in /tmp containing base64 encoded password as first line
if isHedgeNODE; then
  log "${LINENO}" "${MYFILE}" "Seeding mqtt secret on NODE from file: $MQTT_ENCPWD_INP_FILE"
  # Validate MQTT credentials and prep mqtt secrets config
  validate_mqtt_secret_file || { errlog "${LINENO}" "${MYFILE}" "Failed during reading of mqtt secret file"; return 1; } # exports env MQTT_PASSWORD
  validate_mqtt_credentials || { errlog "${LINENO}" "${MYFILE}" "Failed during MQTT username/password validation"; return 1; } # validate MQTT username/password
  encode_mqtt_creds_in_file || { errlog "${LINENO}" "${MYFILE}" "Failed while configuring MQTT server connection"; return 1; } # encodes MQTT credentials
  # Validate Consul token in file and export env HEDGE_CONSUL_TOKEN
  log "${LINENO}" "${MYFILE}" "Seeding consul token on NODE from file: $CNSL_ENCTKN_INP_FILE"
  validate_consul_token_file || { errlog "${LINENO}" "${MYFILE}" "Failed during reading of consul token file"; return 1; }
  # Validate NATS certificates in file and export env HEDGE_NATS_TLS_CRT, HEDGE_NATS_TLS_KEY
  validate_nats_tls_files || { errlog "${LINENO}" "${MYFILE}" "Failed during reading of nats tls files"; return 1; }

# when setting up a CORE, mqtt password will be auto-generated
elif isHedgeCORE; then
  generate_mqtt_secret "$MQTT_USERNAME"   # sets in file and env MQTT_PASSWORD
  generate_consul_token                   # sets in file and env HEDGE_CONSUL_TOKEN
  generate_nats_tls                       # sets in file
  generate_db_secret                      # sets in file and env PG_DB_PASSWORD

else
  errlog "${LINENO}" "${MYFILE}" "Invalid NODE_TYPE $NODE_TYPE \(should be NODE, CORE or CORE_NODE\). Exiting with failure.."
  return 1
fi

# Dont proceed further if nothing changed - bootstrap status should be complete
if is_bootstrap_complete; then
  log "${LINENO}" "${MYFILE}" "Hedge security bootstrapping already in place. Exiting with success.."
  return 0
fi

##### Below is needed for configuring MQTT clients and creating an ADE connection #####
log "${LINENO}" "${MYFILE}" "Seed hedge secrets from environment, hedge-secrets volume/PV should be in place"

if [ -f "$HEDGE_SECRETS_JSONTEMPLATE" ]; then \
  log "${LINENO}" "${MYFILE}" "Found the secrets template $HEDGE_SECRETS_JSONTEMPLATE, will update secrets based on env var"

  ## escape '/' special character if used in password
  PG_DB_PASSWORD_SAFE=$(echo "$PG_DB_PASSWORD" | sed 's/\//\\\//g')
  # Generate service specific hedge-secrets files for persisting in to Vault
  MQTT_PASSWORD_SAFE=$(echo "$MQTT_PASSWORD" | sed 's/\//\\\//g') # escape special character '/' if used in password

  update_secret_placeholders "$MQTT_PASSWORD_SAFE" "$PG_DB_PASSWORD_SAFE" "$HEDGE_CONSUL_TOKEN" # exports env HEDGE_SECRETS_JSON
  create_vault_secret_json "$HEDGE_SECRETS_JSON"

  #update status file to mark complete
  update_bootstrap_complete

  log "${LINENO}" "${MYFILE}" "Successfully created secrets json files to be used by Vault"

  #extract and create the secret file to be used during Node setup
  dump_mqtt_secret_file && dump_consul_token_file && dump_nats_tls_files
  return $?
else
  errlog "${LINENO}" "${MYFILE}" "Template secret seeding file not found: $HEDGE_SECRETS_JSONTEMPLATE. Exiting with failure.."
  return 1
fi