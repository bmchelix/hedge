#!/usr/bin/dumb-init /bin/sh
#  ----------------------------------------------------------------------------------
#  Copyright (c) 2021 Intel Corporation
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
#
#  SPDX-License-Identifier: Apache-2.0
#  ----------------------------------------------------------------------------------

# This is customized entrypoint script for Consul and run on the consul's container
# In particular, it waits for Vault to be ready to roll

set -e

# function to check on Vault for readiness
vault_ready()
{
  vault_host=$1
  vault_port=$2
  resp_code=$(curl --write-out '%{http_code}' --silent --output /dev/null "${vault_host}":"${vault_port}"/v1/sys/health)
  if [ "$resp_code" -eq 200 ] ; then
    echo 1
  else
    echo 0
  fi
}

# env settings are populated from env files of docker-compose

# only in json format according to Consul's documentation
DEFAULT_CONSUL_LOCAL_CONFIG='
{
    "enable_local_script_checks": true,
    "disable_update_check": true,
    "ports": {
      "dns": -1
    }
}
'

# set the default value to environment var if not present
CONSUL_LOCAL_CONFIG=${CONSUL_LOCAL_CONFIG:-$DEFAULT_CONSUL_LOCAL_CONFIG}

export CONSUL_LOCAL_CONFIG

echo "$(date) CONSUL_LOCAL_CONFIG: ${CONSUL_LOCAL_CONFIG}"

echo "$(date) Starting edgex-core-consul with ACL enabled ..."

###### HEDGE changes starts : '-client 0.0.0.0 &' changed to '-client ${CURRENT_HEDGE_NODE_SERVER_IP} &'
docker-entrypoint.sh agent \
  -ui \
  -bootstrap \
  -server \
  -client ${CURRENT_HEDGE_NODE_SERVER_IP} &
###### HEDGE changes ends

# wait for the secretstore tokens ready as we need the token for bootstrapping
echo "$(date) Executing waitFor on Consul with waiting on TokensReadyPort \
  tcp://${STAGEGATE_SECRETSTORESETUP_HOST}:${STAGEGATE_SECRETSTORESETUP_TOKENS_READYPORT}"
/edgex-init/security-bootstrapper --configDir=/edgex-init/res waitFor \
  -uri tcp://"${STAGEGATE_SECRETSTORESETUP_HOST}":"${STAGEGATE_SECRETSTORESETUP_TOKENS_READYPORT}" \
  -timeout "${STAGEGATE_WAITFOR_TIMEOUT}"

# we don't want to exit out the whole Consul process when ACL bootstrapping failed, just that
# Consul won't have ACL to be used
set +e
# call setupRegistryACL bootstrapping command, containing both ACL bootstrapping and re-configure consul access steps
/edgex-init/security-bootstrapper --configDir=/edgex-init/res setupRegistryACL
setupACL_code=$?
if [ "${setupACL_code}" -ne 0 ]; then
  echo "$(date) failed to set up Consul ACL"

  ###### HEDGE changes starts : new block
  # Check if ACL was already set up 
  if test -f /consul/config/consul_acl_done; then
    echo "$(date) ACL was already setup"
  else
    echo "$(date) ACL was NOT setup earlier. To update /consul/data/acl-bootstrap-reset and retry"

    cmd=`curl --request PUT --header "X-Consul-Token: 39CD662F-76D5-4CA1-A980-7269923DD66A" http://${STAGEGATE_REGISTRY_HOST}:${STAGEGATE_REGISTRY_PORT}/v1/acl/bootstrap`
    echo "$(date) /v1/acl/bootstrap returned: $cmd"
    idx=`echo $cmd | sed  -e 's/^.*(reset index: \([0-9]*\))/\1/g'`

    #Sample response: Permission denied: ACL bootstrap no longer allowed (reset index: 5)
    echo "$(date) Forced Reset Index value in /consul/data/acl-bootstrap-reset = $idx"
    echo "${idx}" > /consul/data/acl-bootstrap-reset

    #Restart the container
    echo "$(date) Restarting the container..."
    pkill -f dumb-init
  fi
  ###### HEDGE changes ends
fi

# we need to grant the permission for proxy setup to read consul's token path so as to retrieve consul's token from it
echo "$(date) Changing ownership of consul token path to ${EDGEX_USER}:${EDGEX_GROUP}"

###### HEDGE changes starts: last param changed from STAGEGATE_REGISTRY_ACL_MANAGEMENTTOKENPATH to STAGEGATE_REGISTRY_ACL_BOOTSTRAPTOKENPATH
chown -Rh "${EDGEX_USER}":"${EDGEX_GROUP}" "${STAGEGATE_REGISTRY_ACL_BOOTSTRAPTOKENPATH}"
###### HEDGE changes ends

set -e
# no need to wait for Consul's port since it is in ready state after all ACL stuff

###### HEDGE changes starts: new block
echo -e "$(date) Important env vars: "\
      "\n    CURRENT_HEDGE_NODE_TYPE=${CURRENT_HEDGE_NODE_TYPE}"\
      "\n    CURRENT_HEDGE_NODE_SERVER_NAME=${CURRENT_HEDGE_NODE_SERVER_NAME}"\
      "\n    CURRENT_HEDGE_NODE_FRIENDLY_NAME=${CURRENT_HEDGE_NODE_FRIENDLY_NAME}"\
      "\n    CURRENT_HEDGE_NODE_SERVER_IP=${CURRENT_HEDGE_NODE_SERVER_IP}"\
      "\n    REMOTE_HEDGE_CORE_SERVER_NAME=${REMOTE_HEDGE_CORE_SERVER_NAME}"\
      "\n    REMOTE_HEDGE_CORE_SERVER_IP=${REMOTE_HEDGE_CORE_SERVER_IP}"

HEDGE_CONSUL_TOKEN=
HEDGE_CONSUL_TOKEN_FILE=/tmp/hedge-secrets/.consultoken
MASTER_CONSUL_TOKEN=`cat "${STAGEGATE_REGISTRY_ACL_BOOTSTRAPTOKENPATH}" | jq -r '.SecretID'`

# If the Hegde consul token was already created, fetch it.
if test -f ${HEDGE_CONSUL_TOKEN_FILE}; then
  # consul hedge token was already created
  echo -e "$(date) Hedge consul token already exists at ${HEDGE_CONSUL_TOKEN_FILE}"
  read -r HEDGE_CONSUL_TOKEN < "$HEDGE_CONSUL_TOKEN_FILE"
  HEDGE_CONSUL_TOKEN=$(echo "$HEDGE_CONSUL_TOKEN" | tr -d '[:space:]')  # Remove leading/trailing whitespace

  if [[ ${#HEDGE_CONSUL_TOKEN} -eq 0 ]]; then
    echo -e "[${LINENO}] ERROR: Empty consul token file $HEDGE_CONSUL_TOKEN_FILE. Deleting the file to regenerate a new token"
    rm -f $HEDGE_CONSUL_TOKEN_FILE
  fi
fi

case "$CURRENT_HEDGE_NODE_TYPE" in
  #### If CORE ####
  *CORE*)
    # Generate a new consul Hedge token if it was not already created
    if ! test -f ${HEDGE_CONSUL_TOKEN_FILE}; then
      # Token file not found. Create a new consul token
      HEDGE_CONSUL_TOKEN=`cat /proc/sys/kernel/random/uuid` ##`uuidgen`
      touch ${HEDGE_CONSUL_TOKEN_FILE} # if not, the next command considers this a directory
      echo ${HEDGE_CONSUL_TOKEN} > ${HEDGE_CONSUL_TOKEN_FILE}
      echo -e "$(date) New Hedge consul token created at ${HEDGE_CONSUL_TOKEN_FILE}"
    fi
    ;;

  #### If NODE ####
  NODE)
    # Error out if Hedge consul token file does not exist
    if ! test -f ${HEDGE_CONSUL_TOKEN_FILE}; then
      echo "$(date) Hedge Consul token not found on NODE ${HEDGE_CONSUL_TOKEN_FILE}. Error out.."
      return 1
    fi

    # Join WAN on startup
    wan=`consul join -http-addr ${CURRENT_HEDGE_NODE_SERVER_IP}:${STAGEGATE_REGISTRY_PORT} -token ${MASTER_CONSUL_TOKEN} -wan ${CURRENT_HEDGE_NODE_SERVER_IP} ${REMOTE_HEDGE_CORE_SERVER_IP}`
    echo "$(date) Join WAN returned: \"$wan\""
    ;;
esac

# Add the new hedge consul token
echo -e "$(date) Creating a Hedge consul ACL token with policy edgex-management-policy"
add_hedge_token=$(consul acl token create \
                    -secret ${HEDGE_CONSUL_TOKEN} \
                    -description "BMC Hedge Management Token" \
                    -policy-name edgex-management-policy \
                    -http-addr ${CURRENT_HEDGE_NODE_SERVER_IP}:${STAGEGATE_REGISTRY_PORT} \
                    -token ${MASTER_CONSUL_TOKEN} || true)
                    # always return success above, since this will fail if token already exists
echo -e "$(date) Add Hedge consul token returned: \n$add_hedge_token"

members=`consul members -wan -http-addr ${CURRENT_HEDGE_NODE_SERVER_IP}:${STAGEGATE_REGISTRY_PORT} -token ${MASTER_CONSUL_TOKEN}`
echo -e "$(date) WAN members:\n$members"
###### HEDGE changes ends

# Signal that Consul is ready for services blocked waiting on Consul
exec su-exec consul /edgex-init/security-bootstrapper --configDir=/edgex-init/res listenTcp \
  --port="${STAGEGATE_REGISTRY_READYPORT}" --host="${STAGEGATE_REGISTRY_HOST}"
if [ $? -ne 0 ]; then
    echo "$(date) failed to gating the consul ready port, exits"
fi