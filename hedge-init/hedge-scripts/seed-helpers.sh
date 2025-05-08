#!/bin/sh

#set -x   # Uncomment for debugging
set -e    # Exit immediately if any command returns a non-zero status

## env variable MQTT_USERNAME should be set - holds the mqtt username
## env variable MQTT_ENCPWD_INP_FILE should be set - holds the path to encrypted password to-be used for mqtt client setup on NODE
##
## Core generates a new password and populates the secrets file at MQTT_ENCPWD_INP_FILE + also creates .mqcredhash for mqtt broker setup
## Core/Node seeds encrypted password from MQTT_ENCPWD_INP_FILE to generate hedge_secrets.json (from template hedge_secrets_template.json)
## hedge-bootstrap-done - existence of this file indicates bootstrapping was already done

FILE="seed-helpers"
HEDGE_SECRETS_PATH='/tmp/hedge-secrets'
HEDGE_BOOTSTRAP_STATUS_FILE='/tmp/hedge-secrets/hedge-bootstrap-status'           #Bootstrap status indicator file - indicates bootstrapping was already done
HEDGE_SECRETS_JSONTEMPLATE='./hedge-scripts/secrets/hedge_secrets_template.json'  #Template secrets file
BTSP_HEDGEDB_PWD_FILE='/tmp/hedge-secrets/.pgpassword'                            #HedgeDB pg password file to be used for server bootstrapping and client connections
BTSP_VAULT_SECRETS_JSONFILE='/tmp/hedge-secrets/hedge_secrets.json'               #Final secrets file to be bootstrapped into vault
BTSP_CONSUL_TOKEN_FILE='/tmp/hedge-secrets/.consultoken'                          #holds the consul token to be used for consul bootstrapping on node (decoded content from CNSL_ENCTKN_INP_FILE)
BTSP_MQTT_ENCCREDS_FILE='/tmp/hedge-secrets/.mqcredhash'                          #holds the encrypted username:<hashed password> to be used for mqtt client/server setup
MQTT_BROKER_CONF_FILE='/tmp/hedge-secrets/hedge_mosquitto.conf'                   #Mqtt broker config file

### coming from environment - MQTT_USERNAME, MQTT_ENCPWD_INP_FILE, CONSUL_ENVTOKEN_INP_FILE
MQTT_USERNAME=$MQTT_USERNAME                  #from .env - hedgemquser
MQTT_ENCPWD_INP_FILE=$MQTT_ENCPWD_INP_FILE    #from .env - /tmp/.mqttencpwd
CNSL_ENCTKN_INP_FILE=$CNSL_ENCTKN_INP_FILE    #from .env - /tmp/.cnslenctok (encoded content from BTSP_CONSUL_TOKEN_FILE)
NATS_TLS_CRT=$NATS_TLS_CRT                    #from .env - /tmp/nats.crt.pem
NATS_TLS_KEY=$NATS_TLS_KEY                    #from .env - /tmp/nats.key.pem

HEDGE_BOOTSTRAP_STATUS_JSON={}
HEDGE_SECRETNAME_MQTT='mbconnection'
HEDGE_SECRETNAME_DB='dbconnection'
HEDGE_SECRETNAME_CONSUL='consulconnection'
HEDGE_SECRETNAME_VAULT='vaultconnection'
HEDGE_SECRETNAME_NATS='natsconnection'
HEDGE_BOOTSTRAP_COMPLETE='bootstrapdone'

NODE_HEDGE_SECRETS_FILES="hedge_admin_secrets.json
                          hedge_remediate_secrets.json
                          hedge_device_extensions_secrets.json
                          hedge_meta_sync_secrets.json
                          hedge_data_enrichment_secrets.json
                          hedge_event_publisher_secrets.json
                          hedge_ml_broker_secrets.json
                          hedge_ml_edge_agent_secrets.json"

CORE_HEDGE_SECRETS_FILES="$NODE_HEDGE_SECRETS_FILES
                          hedge_user_app_mgmt_secrets.json
                          hedge_ml_management_secrets.json
                          hedge_ml_sandbox_secrets.json
                          hedge_export_biz_data_secrets.json
                          hedge_export_secrets.json
                          hedge_event_secrets.json"

# Logging function
# Usage: log <current line> <filename> <log message>.
# Eg.log "${LINENO}" "${FILE}" "my log"
log() {
    lineno="$1"
    filename="$2"
    message="$3"
    printf "[%s:%03d] %s\n" "$filename" "$lineno" "$message"
}
# Error logging function
# Usage: errlog <current line> <filename> <error message>.
# Eg.errlog "${LINENO}" "${FILE}" "my error log"
errlog() {
    lineno="$1"
    filename="$2"
    message="$3"
    printf "[%s:%03d] ERROR: %s\n" "$filename" "$lineno" "$message"
}

### Check Hedge NODE or CORE environment
isHedgeCORE() {
  # search *CORE*
  case "$NODE_TYPE" in *CORE*)
    return 0 ;;
  esac
  return 1
}
isHedgeNODE() {
  case "$NODE_TYPE" in NODE)
    return 0 ;;
  esac
  return 1
}


### Hedge bootstrapping
# Create the status file, if it does not exist
init_bootstrap_status() {
  if [ -f "$HEDGE_BOOTSTRAP_STATUS_FILE" ]; then
    HEDGE_BOOTSTRAP_STATUS_JSON=$(cat "$HEDGE_BOOTSTRAP_STATUS_FILE") || \
                                { errlog "${LINENO}" "${FILE}" "Failed to read file: $HEDGE_BOOTSTRAP_STATUS_FILE" >&2; exit 1; }
    export HEDGE_BOOTSTRAP_STATUS_JSON
  else
    HEDGE_BOOTSTRAP_STATUS_JSON="{
      \"$HEDGE_SECRETNAME_MQTT\":false,
      \"$HEDGE_SECRETNAME_DB\":false,
      \"$HEDGE_SECRETNAME_CONSUL\":false,
      \"$HEDGE_SECRETNAME_NATS\":false,
      \"$HEDGE_BOOTSTRAP_COMPLETE\":false
    }"
    echo "$HEDGE_BOOTSTRAP_STATUS_JSON" > "$HEDGE_BOOTSTRAP_STATUS_FILE"
  fi
}

### Update bootstrap status file to add the
update_bootstrap_step() {
  stepname=$1
  stepval=$2
  # Update the step value in bootstrap status file, and reset bootstrap done status to false
  updated_json=$(echo "$HEDGE_BOOTSTRAP_STATUS_JSON" | \
          jq --arg key "$stepname" --argjson value "$stepval" '.[$key] = $value' | \
          jq --arg HEDGE_BOOTSTRAP_COMPLETE "$HEDGE_BOOTSTRAP_COMPLETE" '.[$HEDGE_BOOTSTRAP_COMPLETE] = false')
  echo "$updated_json" > "$HEDGE_BOOTSTRAP_STATUS_FILE"
  export HEDGE_BOOTSTRAP_STATUS_JSON="$updated_json"
}

### Check if a specific bootstrapping step is done
is_bootstrap_step_done() {
  stepname=$1
  echo "$HEDGE_BOOTSTRAP_STATUS_JSON" | jq --exit-status ".$stepname == true" > /dev/null
}

update_bootstrap_complete() {
  updated_json=$(echo "$HEDGE_BOOTSTRAP_STATUS_JSON" | \
          jq --arg HEDGE_BOOTSTRAP_COMPLETE "$HEDGE_BOOTSTRAP_COMPLETE" '.[$HEDGE_BOOTSTRAP_COMPLETE] = true')
  echo "$updated_json" > "$HEDGE_BOOTSTRAP_STATUS_FILE"
  export HEDGE_BOOTSTRAP_STATUS_JSON="$updated_json"
}

is_bootstrap_complete() {
  echo "$HEDGE_BOOTSTRAP_STATUS_JSON" | jq --exit-status ".$HEDGE_BOOTSTRAP_COMPLETE == true" > /dev/null
}


### HedgeDB database credentials
generate_db_secret(){
    if is_bootstrap_step_done "$HEDGE_SECRETNAME_DB"; then
      log "${LINENO}" "${FILE}" ">> $HEDGE_SECRETNAME_DB already bootstrapped. Continuing.."
      get_db_secret
      return 0
    fi
    log "${LINENO}" "${FILE}" "> Bootstrapping $HEDGE_SECRETNAME_DB.."

    # BTSP_HEDGEDB_PWD_FILE is used by Postgres and it is for the db password file
    log "${LINENO}" "${FILE}" "Generate database password, hedge-secrets volume/PV should be in place"
    log "${LINENO}" "${FILE}" "Set DB password file in ${BTSP_HEDGEDB_PWD_FILE}"
    # create password file for postgres to be used in the compose file
    mkdir -p "$(dirname "${BTSP_HEDGEDB_PWD_FILE}")"

    PG_DB_PASSWORD=$(openssl rand -base64 32) || { log "${LINENO}" "${FILE}" "failed to generate DB password"; exit 1; }
    echo "$PG_DB_PASSWORD" > "${BTSP_HEDGEDB_PWD_FILE}"
    #chmod 444 "${BTSP_HEDGEDB_PWD_FILE}"
    update_bootstrap_step "$HEDGE_SECRETNAME_DB" true

    export PG_DB_PASSWORD #export result
}

get_db_secret(){
  PG_DB_PASSWORD=""
  if isHedgeCORE; then
    PG_DB_PASSWORD=$(cat "$BTSP_HEDGEDB_PWD_FILE") || exit 1
    export PG_DB_PASSWORD #export result
  fi
}


### MQTT credentials
# pass MQTT username as param 1
generate_mqtt_secret() {
  MQTT_USERNAME=$1
  if is_bootstrap_step_done "$HEDGE_SECRETNAME_MQTT"; then
    log "${LINENO}" "${FILE}" ">> $HEDGE_SECRETNAME_MQTT already bootstrapped. Continuing.."
    validate_mqtt_secret_file
    return 0
  fi
  log "${LINENO}" "${FILE}" "> Bootstrapping $HEDGE_SECRETNAME_MQTT.."

  log "${LINENO}" "${FILE}" "Generating mqtt password on CORE"
  pwgenDone=0
  while [ $pwgenDone -eq 0 ]; do
    MQTT_PASSWORD=$(openssl rand -base64 15)
    #Make sure the password does not contain any non-ASCII characters - may be a result of decoding an invalid base64 string
    LC_CTYPE=C;
    case $MQTT_PASSWORD in *[![:cntrl:][:print:]]*)
      errlog "${LINENO}" "${FILE}" "Password contains invalid ascii characters. Retrying.."
      continue;;
    esac
    pwgenDone=1
  done

  echo "$MQTT_PASSWORD" > "$MQTT_ENCPWD_INP_FILE"
  log "${LINENO}" "${FILE}" "Mqtt password successfully generated at $MQTT_ENCPWD_INP_FILE"

  export MQTT_PASSWORD #export result

  #configure MQTT server connection
  encode_mqtt_creds_in_file
}

# Prepare secret to configure MQTT server
encode_mqtt_creds_in_file() {
  validate_mqtt_credentials || { errlog "${LINENO}" "${SEEDFILE}" "Failed during MQTT username/password validation"; exit 1; }

  if isHedgeCORE; then
    ## Write plaintext username:password to the file
    echo "$MQTT_USERNAME:$MQTT_PASSWORD" > "$BTSP_MQTT_ENCCREDS_FILE"
    #chmod 444 $BTSP_MQTT_ENCCREDS_FILE

    ## Hash the plaintext MQTT password and replace in file
    # Supress - Warning: File /tmp/hedge-secrets/.mqcredhash has world readable permissions. Future versions will refuse to load this file.
    #           To fix this, use `chmod 0700 /tmp/hedge-secrets/.mqcredhash`
    if ! /usr/bin/mosquitto_passwd -U "$BTSP_MQTT_ENCCREDS_FILE" 2>&1 | grep -v "Warning" | grep -v "chmod 0700"; then
      log "${LINENO}" "${FILE}" "Mqtt hashed password file successfully created at $BTSP_MQTT_ENCCREDS_FILE"
    else
      errlog "${LINENO}" "${FILE}" "Failed creating the mqtt password file $BTSP_MQTT_ENCCREDS_FILE. Exiting with failure.."
      return 1
    fi
  fi

  ## Make mosquitto config file available
  cp /hedge/hedge-scripts/secrets/hedge_mosquitto.conf $MQTT_BROKER_CONF_FILE
  update_bootstrap_step "$HEDGE_SECRETNAME_MQTT" true
}

validate_mqtt_secret_file() {
  if [ ! -f "$MQTT_ENCPWD_INP_FILE" ]; then
    errlog "${LINENO}" "${FILE}" "Mqtt secret file missing at $MQTT_ENCPWD_INP_FILE. Exiting with failure.."
    return 1
  fi

  log "${LINENO}" "${FILE}" "Mqtt secret file found at $MQTT_ENCPWD_INP_FILE."

  read -r MQTT_ENCPWD_INP < "$MQTT_ENCPWD_INP_FILE"
  if [ -z "$MQTT_ENCPWD_INP" ]; then
    errlog "${LINENO}" "${FILE}" "Empty mqtt secret returned. Exiting with failure.."
    return 1
  fi

  MQTT_PASSWORD=$(echo "$MQTT_ENCPWD_INP" | base64 -d) || { \
    errlog "${LINENO}" "${FILE}" "Password could not be decoded. Exiting with failure.."; \
    return 1; }

  export MQTT_PASSWORD #export result

  #Make sure the password does not contain any non-ASCII characters - may be a result of decoding an invalid base64 string
  LC_CTYPE=C;
  case $MQTT_PASSWORD in *[![:cntrl:][:print:]]*)
    errlog "${LINENO}" "${FILE}" "Password contains invalid ascii characters. Exiting with failure.."
    return 1;;
  esac

  #mqtt connection check
  if isHedgeNODE; then
    check_mqtt_connection "$APPLICATIONSETTINGS_MQTTSERVER" "$MQTT_USERNAME" "$MQTT_PASSWORD" || { \
      errlog "${LINENO}" "${FILE}" "Mqtt connection could not be established with CORE @ $APPLICATIONSETTINGS_MQTTSERVER using encrypted password in $MQTT_ENCPWD_INP_FILE. \
                          \nMake sure you have copied the correct secret file from CORE"; \
      return 1; }
  fi

  log "${LINENO}" "${FILE}" "Successfully validated Mqtt secret"
  return 0
}

check_mqtt_connection() {
  mosquitto_pub -h "$APPLICATIONSETTINGS_MQTTSERVER" -u "$MQTT_USERNAME" -P "$MQTT_PASSWORD" -t pingcheck -m ping
}

#Verify both username & password is not empty
validate_mqtt_credentials() {
  if [ -z "$MQTT_USERNAME" ]; then
    errlog "${LINENO}" "${FILE}" "Empty mqtt user name received. Exiting with failure.."
    return 1
  fi
  if [ -z "$MQTT_PASSWORD" ]; then
    errlog "${LINENO}" "${FILE}" "mqtt password generation failed. Exiting with failure.."
    return 1
  fi
}

dump_mqtt_secret_file() {
  if isHedgeNODE; then
    validate_mqtt_secret_file && { log "${LINENO}" "${FILE}" "Valid mqtt secret file found on NODE at $MQTT_ENCPWD_INP_FILE";
      return 0; }

  elif isHedgeCORE; then
    if [ ! -f $BTSP_VAULT_SECRETS_JSONFILE ]; then
      errlog "${LINENO}" "${FILE}" "Mqtt secrets not bootstrapped yet. Make sure to setup this Hedge CORE stack successfully before retrying. Exiting with failure.."
      return 1
    fi

    cat "$BTSP_VAULT_SECRETS_JSONFILE" | \
        jq -r --arg HEDGE_SECRETNAME_MQTT "$HEDGE_SECRETNAME_MQTT" \
        '.secrets[] | select(.secretName == $HEDGE_SECRETNAME_MQTT) | .secretData[] | select(.key == "password") | .value' | base64 \
        > "$MQTT_ENCPWD_INP_FILE"

    validate_mqtt_secret_file && { \
      log "${LINENO}" "${FILE}" "Mqtt secret file successfully created on CORE."; \
      log "${LINENO}" "${FILE}" "SUCCESS! Download and copy $MQTT_ENCPWD_INP_FILE file to the NODE(s)."; \
      return 0; }
  fi

  errlog "${LINENO}" "${FILE}" "Exiting with failure.."
  return 1
}


### Consul token
generate_consul_token() {
  if is_bootstrap_step_done "$HEDGE_SECRETNAME_CONSUL"; then
    log "${LINENO}" "${FILE}" ">> $HEDGE_SECRETNAME_CONSUL already bootstrapped. Continuing.."
    validate_consul_token_file
    return 0
  fi
  log "${LINENO}" "${FILE}" "> Bootstrapping $HEDGE_SECRETNAME_CONSUL.."

  #generate consul token
  HEDGE_CONSUL_TOKEN=$(cat /proc/sys/kernel/random/uuid) ##`uuidgen`

  echo "${HEDGE_CONSUL_TOKEN}" > ${BTSP_CONSUL_TOKEN_FILE}
  #chmod 444 ${BTSP_CONSUL_TOKEN_FILE}
  update_bootstrap_step "$HEDGE_SECRETNAME_CONSUL" true

  log "${LINENO}" "${FILE}" "New Hedge consul token created at ${BTSP_CONSUL_TOKEN_FILE}"
  export HEDGE_CONSUL_TOKEN #export result
}

validate_consul_token_file() {
  if isHedgeCORE; then
    if [ -f $BTSP_CONSUL_TOKEN_FILE ]; then
      read -r HEDGE_CONSUL_TOKEN < $BTSP_CONSUL_TOKEN_FILE
      if [ -z "$HEDGE_CONSUL_TOKEN" ]; then
        errlog "${LINENO}" "${FILE}" "Empty consul token at $BTSP_CONSUL_TOKEN_FILE. Exiting with failure.."
        errlog "${LINENO}" "${FILE}" "Make sure to download and copy the correct $CNSL_ENCTKN_INP_FILE file from CORE!"
        return 1
      fi
    fi

    log "${LINENO}" "${FILE}" "Successfully validated consul token"
    export HEDGE_CONSUL_TOKEN   # export result
    return 0
  fi

  # If NODE, execute ahead..

  #Get consul token from the secret file at /tmp
  if [ ! -f "$CNSL_ENCTKN_INP_FILE" ]; then
    errlog "${LINENO}" "${FILE}" "Consul secret file missing at $CNSL_ENCTKN_INP_FILE. Exiting with failure.."
    return 1
  fi

  log "${LINENO}" "${FILE}" "Consul secret file found at $CNSL_ENCTKN_INP_FILE."

  read -r CONSUL_TOKEN_INP < "$CNSL_ENCTKN_INP_FILE"
  if [ -z "$CONSUL_TOKEN_INP" ]; then
    errlog "${LINENO}" "${FILE}" "Empty consul token returned. Exiting with failure.."
    return 1
  fi

  HEDGE_CONSUL_TOKEN=$(echo "$CONSUL_TOKEN_INP" | base64 -d) || { \
    errlog "${LINENO}" "${FILE}" "Consul token could not be decoded. Exiting with failure.."; \
    return 1; }

  if [ ! -f $BTSP_CONSUL_TOKEN_FILE ]; then
    echo "${HEDGE_CONSUL_TOKEN}" > ${BTSP_CONSUL_TOKEN_FILE}
    log "${LINENO}" "${FILE}" "Hedge consul token dumped at ${BTSP_CONSUL_TOKEN_FILE}"
  else
    read -r CONSUL_TOKEN < $BTSP_CONSUL_TOKEN_FILE
    if [ "$CONSUL_TOKEN" != "$HEDGE_CONSUL_TOKEN" ]; then
      errlog "${LINENO}" "${FILE}" "Mismatched consul tokens. One of them is invalid: $BTSP_CONSUL_TOKEN_FILE or $CNSL_ENCTKN_INP_FILE. Exiting with failure.."
      return 1
    fi
  fi

  log "${LINENO}" "${FILE}" "Successfully validated consul token"
  export HEDGE_CONSUL_TOKEN   # export result

  return 0
}

dump_consul_token_file() {
  if isHedgeNODE; then
    validate_consul_token_file && { \
      log "${LINENO}" "${FILE}" "Valid consul token file found on NODE at $CNSL_ENCTKN_INP_FILE"; \
      return 0; }

  elif isHedgeCORE; then
    read -r CONSUL_TOKEN < $BTSP_CONSUL_TOKEN_FILE
    echo "$CONSUL_TOKEN" | base64 > "$CNSL_ENCTKN_INP_FILE"

    validate_consul_token_file && { \
      log "${LINENO}" "${FILE}" "Consul token file successfully created on CORE."; \
      log "${LINENO}" "${FILE}" "SUCCESS! Download and copy $CNSL_ENCTKN_INP_FILE file to the NODE(s)."; \
      return 0; }
  fi

  errlog "${LINENO}" "${FILE}" "Exiting with failure.."
  return 1
}


### NATS TLS certificates
generate_nats_tls() {
  if is_bootstrap_step_done "$HEDGE_SECRETNAME_NATS"; then
    log "${LINENO}" "${FILE}" ">> $HEDGE_SECRETNAME_NATS already bootstrapped. Continuing.."
    #validate_nats_tls_files
    return 0
  fi
  log "${LINENO}" "${FILE}" "> Bootstrapping $HEDGE_SECRETNAME_NATS.."

  SAN_IP=""
  if [ -n "$NATS_SERVER_IP" ]; then
    SAN_IP="IP:${NATS_SERVER_IP},"
    log "${LINENO}" "${FILE}" "Set SubjectAlternateName as IP provided: $NATS_SERVER_IP"
  fi

  SAN_DNS=""
  if [ -n "$NATS_SERVER_NAME" ]; then
    SAN_DNS="DNS:${NATS_SERVER_NAME},"
    log "${LINENO}" "${FILE}" "Set SubjectAlternateName as DNS provided: $NATS_SERVER_NAME"
  fi

  keypem=/certs/nats.key.pem
  certpem=/certs/nats.crt.pem
  if test -d /certs ; then
      if test ! -f "${keypem}" ; then
          openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
          -subj "/C=GB/CN=${CURRENT_HEDGE_NODE_SERVER_NAME}" \
          -addext "subjectAltName = ${SAN_IP} ${SAN_DNS} DNS:${CURRENT_HEDGE_NODE_SERVER_NAME}, DNS:edgex-nats-server" \
          -keyout "${keypem}" \
          -out "${certpem}"
          log "${LINENO}" "${FILE}" "NATS TLS certificate successfully created on CORE."
          cp ${certpem} "${NATS_TLS_CRT}"
          cp ${keypem} "${NATS_TLS_KEY}"
      fi
  fi

  update_bootstrap_step "$HEDGE_SECRETNAME_NATS" true

  #export results
  HEDGE_NATS_TLS_CRT=$(cat "$NATS_TLS_CRT")
  HEDGE_NATS_TLS_KEY=$(cat "$NATS_TLS_KEY")
  export HEDGE_NATS_TLS_CRT HEDGE_NATS_TLS_KEY
}

validate_nats_tls_files() {
  if [ ! -f "$NATS_TLS_CRT" ]; then
    errlog "${LINENO}" "${FILE}" "NATS TLS cert missing. Make sure to download and copy the $NATS_TLS_CRT file from CORE. Exiting with failure.."
    return 1
  fi
  if [ ! -f "$NATS_TLS_KEY" ]; then
    errlog "${LINENO}" "${FILE}" "NATS TLS key missing. Make sure to download and copy the correct $NATS_TLS_KEY file from CORE. Exiting with failure.."
    return 1
  fi
  log "${LINENO}" "${FILE}" "Successfully validated NATS certificates"

  cp "$NATS_TLS_CRT" /certs
  cp "$NATS_TLS_KEY" /certs

  #export results
  HEDGE_NATS_TLS_CRT=$(cat "$NATS_TLS_CRT")
  HEDGE_NATS_TLS_KEY=$(cat "$NATS_TLS_KEY")
  export HEDGE_NATS_TLS_CRT HEDGE_NATS_TLS_KEY
  return 0
}

dump_nats_tls_files() {
  if isHedgeNODE; then
    if [ -f "${NATS_TLS_CRT}" ] && [ -f "${NATS_TLS_KEY}" ]; then
      log "${LINENO}" "${FILE}" "Valid NATS tls files found on NODE at $NATS_TLS_CRT and $NATS_TLS_KEY"
      return 0
    else
      errlog "${LINENO}" "${FILE}" "One or more NATS tls file(s) missing at $NATS_TLS_CRT and $NATS_TLS_KEY. Exiting with failure.."
      return 1
    fi
  fi

  keypem=/certs/nats.key.pem
  certpem=/certs/nats.crt.pem
  if [ -f "${certpem}" ] && [ -f "${keypem}" ]; then
    cp ${certpem} "${NATS_TLS_CRT}"
    cp ${keypem} "${NATS_TLS_KEY}"
  else
    errlog "${LINENO}" "${FILE}" "NATS tls file\(s\) missing - ${certpem} or/and ${keypem}."
    return 1
  fi

  log "${LINENO}" "${FILE}" "SUCCESS! Download and copy $NATS_TLS_CRT and $NATS_TLS_KEY files to the NODE(s)."
  return 0
}

check_and_replace() {
  var_name="$1"
  var_value="$2"
  pattern="$3"
  if [ -z "$var_value" ]; then
    #errlog "${LINENO}" "${FILE}" "$var_name is empty."
    cat #let it pass through without any sed replacements
    return 0
  fi
  sed "s/$pattern/$var_value/"
}

### Vault secrets
update_secret_placeholders() {
  MQTT_PASSWORD_SAFE=$1   # param1
  PG_DB_PASSWORD_SAFE=$2  # param2
  HEDGE_CONSUL_TOKEN=$3   # param3

  HEDGE_SECRETS_JSON=$(cat "$HEDGE_SECRETS_JSONTEMPLATE" | \
    check_and_replace "MQTT_USERNAME"         "$MQTT_USERNAME"          "__mqtt_user_name__" | \
    check_and_replace "MQTT_PASSWORD_SAFE"    "$MQTT_PASSWORD_SAFE"     "__mqtt_password__" | \
    check_and_replace "PG_DB_PASSWORD_SAFE"   "$PG_DB_PASSWORD_SAFE"    "__PG_DB_PASSWORD__" | \
    check_and_replace "HEDGE_CONSUL_TOKEN"    "$HEDGE_CONSUL_TOKEN"     "__CONSUL_TOKEN__" | \
    check_and_replace "REGISTRY_USER"         "$REGISTRY_USER"          "__REGISTRY_USER__" | \
    check_and_replace "REGISTRY_PASSWORD"     "$REGISTRY_PASSWORD"      "__REGISTRY_PASSWORD__")
    # __VAULT_KEY?__ 1/2/3 will be substituted later in Makefile/deploy.sh, once Vault and secretstore-setup has stabilized

  # returning secrets_json..
  export HEDGE_SECRETS_JSON
}

## create_vault_secret_json(secrets_json) -- $secrets_json sent as function parameter
create_vault_secret_json() {
  secrets_json=$1
  if isHedgeNODE; then
    #remove ADE, DWP and Postgres connection json blocks on Node (these are only needed on CORE)
    node_secrets_json=$(echo "$secrets_json" | jq "del(.secrets[] | (select(.secretName == \"$HEDGE_SECRETNAME_DB\")))")

    set -- $NODE_HEDGE_SECRETS_FILES # NOTE: do not enclose in quotes, else it prevents word splitting
    for file in "$@"; do
      echo "$node_secrets_json" > "${HEDGE_SECRETS_PATH}/${file}"
      chmod a+rw "${HEDGE_SECRETS_PATH}/${file}"
    done
    log "${LINENO}" "${FILE}" "Created hedge secret files on NODE"
  else
    set -- $CORE_HEDGE_SECRETS_FILES # NOTE: do not enclose in quotes, else it prevents word splitting
    for file in "$@"; do
      echo "$secrets_json" > "${HEDGE_SECRETS_PATH}/${file}"
      chmod a+rw "${HEDGE_SECRETS_PATH}/${file}"
    done

    echo "$secrets_json" > $BTSP_VAULT_SECRETS_JSONFILE
    log "${LINENO}" "${FILE}" "Created hedge secret files on CORE"
  fi
}
