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

# This is customized entrypoint script for other Edgex services.
# In particular, it waits for the ReadyToRunPort raised to be ready to roll
#
# Note:
#   Since the entrypoint script is overridden, user should also override the command
#   so that the $@ is set appropriately on the run-time.
#

set -e

# env settings are populated from env files of docker-compose

echo "Script for waiting on security bootstrapping ready-to-run"

# gating on the ready-to-run port
echo "$(date) Executing waitFor with $@ waiting on tcp://${STAGEGATE_BOOTSTRAPPER_HOST}:${STAGEGATE_READY_TORUNPORT}"
/edgex-init/security-bootstrapper --configDir=/edgex-init/res waitFor \
  -uri tcp://"${STAGEGATE_BOOTSTRAPPER_HOST}":"${STAGEGATE_READY_TORUNPORT}" \
  -timeout "${STAGEGATE_WAITFOR_TIMEOUT}"

### HEDGE changes starts: edgex's ready_to_run_wait_install.sh modified to add wait for meta-sync service

if [ "${CURRENT_HEDGE_NODE_TYPE}" == "NODE" ]; then
  echo "Waiting on services to be ready..."
  /edgex-init/security-bootstrapper --configDir=/edgex-init/res waitFor \
    -uri tcp://"${META_SYNC_HOST}":"${META_SYNC_PORT}" \
    -timeout "${STAGEGATE_WAITFOR_TIMEOUT}"
fi

# Installing custom CA Certificate
if command -v -- update-ca-certificates > /dev/null 2>&1; then
  update-ca-certificates
  echo "Successfully run update-ca-certificates."
else
  echo "Command update-ca-certificates not found, skipping..."
fi

### HEDGE changes ends

echo "$(date) Starting $@ ..."
exec "$@"
