# /*******************************************************************************
#  * Copyright 2021 Intel
#  *
#  * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
#  * in compliance with the License. You may obtain a copy of the License at
#  *
#  * http://www.apache.org/licenses/LICENSE-2.0
#  *
#  * Unless required by applicable law or agreed to in writing, software distributed under the License
#  * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
#  * or implied. See the License for the specific language governing permissions and limitations under
#  * the License.
#  *
#  *******************************************************************************/

.PHONY: help run down pull force-clean get-node-secrets update-secrets get-consul-token get-vault-token get-consul-bootstrap-token get-version
.DEFAULT: help

#Comment out for debugging
.SILENT:

help:
	printf "\nCommands: make run / make down / make pull / make force-clean / make get-node-secrets / make update-secrets / make get-consul-token \
		 \n\nUsage (make run): make run [ENV=CORE,FLEET,PUMP] [PROFILE=all,virtual,demo,es,vm,biz] [<service-name>] \
		 \n\t<service-name> (default: all) - specific service name to start \
		 \n\tPROFILE=all,virtual,demo,es,vm,biz (default: <empty>) - optional services to deploy. Can be one or more comma separated profiles: all,virtual,demo,es,vm,biz \
		 \n\tENV=<demo env> (default: none) - <demo env> can be one of FLEET or PUMP or CORE or its combination eg FLEET,PUMP to start Fleet Management and/or Petrol Pump monitoring services \
		 \n\n\tSample Usages (make run): \
		 \n\t\tmake run - (most commonly used) to start the complete Helix Edge stack \
		 \n\t\tmake run PROFILE=virtual,demo - to deploy virtual-device-service and other demo related optional services \
		 \n\t\tmake run hedge-export data-enrichment - (for restarting selective services) to only start hedge-export and data-enrichment services \
		 \n\t\tmake run ENV=FLEET - (for fleet demos) to start the complete Helix Edge stack along with Fleet Management services \
		 \n\t\tmake run ENV=CORE - overrides .env params with .env-core and setup a CORE instance \
		 \n\tOther Usages: \
		 \n\t\tmake down - (most commonly used) to stop the complete Helix Edge stack \
		 \n\t\tmake pull - to pull the latest images \
		 \n\t\tmake force-clean - (use with caution) to delete the complete Helix Edge stack and all associated data \
		 \n\t\tmake get-node-secrets - (commonly used) to get important NODE setup instructions on the CORE \
		 \n\t\tmake update-secrets - to change secrets/passwords on the CORE \
		 \n\t\tmake get-consul-token - to fetch secret token to be used for login to Consul (for Service Health and Service Configuration) \
		 \n"
		 # make get-vault-token

### INITIALIZE ###
ENV-FILE:=/tmp/.env-generated
$(shell cat scripts/envheader .env > $(ENV-FILE))
include $(ENV-FILE)

REQUIRED_PACKAGES := docker whiptail jq

#Supports both env name/value in upper & lower case. This converts ENV value to uppercase and PROFILE values to lowercase.
ENV:=$(shell echo $(ENV)$(env) | tr [:lower:] [:upper:])# ENV=CORE
PROFILE:=$(shell echo $(PROFILE)$(profile) | tr [:upper:] [:lower:] | tr -d [:space:])# PROFILE=demo,optional,es...

# include .env-core environment file if ENV=CORE parameter is passed
ifneq (,$(findstring CORE,$(ENV)))
$(shell cat scripts/envheader .env-core >> $(ENV-FILE))
include $(ENV-FILE)
$(info ENV=CORE passed. Setting up environment overrides from .env-core for $(CURRENT_HEDGE_NODE_TYPE) setup)
endif

ifneq (,$(PROFILE))
$(info PROFILE=$(PROFILE) passed. Will be installing additional services for this profile)
OPT-PARAM:=COMPOSE_PROFILES=$(PROFILE)
$(shell echo "" >> $(ENV-FILE))
$(shell echo "" >> $(ENV-FILE))
$(shell cat ./.es-cred-env >> $(ENV-FILE) || echo notfound)
endif

ARGS:=$(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
$(eval $(ARGS):;@:)

OPTIONS:=" node core fleet " # Must have spaces around words for `filter-out` function to work properly
DOCKER_IMG_VERSION=$(shell cat ./VERSION || echo notfound)
HEDGE_RELEASE_VERSION=$(shell echo $(DOCKER_IMG_VERSION) | sed 's/_.*//')
DOCKER_TAG ?= $(DOCKER_IMG_VERSION)

$(shell echo "" >> $(ENV-FILE))
$(shell echo "" >> $(ENV-FILE))
$(shell echo VERSION=$(DOCKER_IMG_VERSION) >> $(ENV-FILE))
$(shell echo DOCKER_TAG=$(DOCKER_TAG) >> $(ENV-FILE))
$(shell echo "" >> $(ENV-FILE))
SERVICES:=$(filter-out $(OPTIONS),$(ARGS))
PULL_ONLY_IMAGES:=hedge-ml-pred-anomaly-autoencoder hedge-ml-pred-classification-randomforest hedge-ml-pred-timeseries-multivariate-deepvar hedge-ml-pred-regression-lightgbm

svc_tag_overrides := HEDGE_INIT_DOCKER_TAG HEDGEXT_SECURITY_BOOTSTRAPPER_DOCKER_TAG HEDGEXT_SECURITY_SECRETSTORE_SETUP_DOCKER_TAG HEDGEXT_CORE_COMMON_CONFIG_BOOTSTRAPPER_DOCKER_TAG \
		HEDGEXT_CONSUL_DOCKER_TAG HEDGEXT_VAULT_DOCKER_TAG HEDGEXT_REDIS_DOCKER_TAG HEDGEXT_SUPPORT_NOTIFICATIONS_DOCKER_TAG HEDGEXT_CORE_METADATA_DOCKER_TAG \
		HEDGE_ADMIN_DOCKER_TAG HEDGE_DEVICE_EXTENSIONS_DOCKER_TAG HEDGE_META_SYNC_DOCKER_TAG HEDGE_EVENT_PUBLISHER_DOCKER_TAG HEDGE_REMEDIATE_DOCKER_TAG \
		HEDGE_NATS_PROXY_DOCKER_TAG HEDGEXT_MOSQUITTO_DOCKER_TAG HEDGEXT_SECURITY_PROXY_SETUP_DOCKER_TAG HEDGEXT_NATS_DOCKER_TAG HEDGE_RSSO_DOCKER_TAG HEDGE_AUTH_PROXY_DOCKER_TAG \
		HEDGEXT_NGINX_DOCKER_TAG HEDGEXT_OPENSEARCH_ES_DOCKER_TAG HEDGEXT_VICTORIA_METRICS_DOCKER_TAG HEDGEXT_POSTGRES_DOCKER_TAG HEDGE_USER_APP_MGMT_DOCKER_TAG \
		HEDGE_ML_MANAGEMENT_DOCKER_TAG HEDGE_ML_SANDBOX_DOCKER_TAG HEDGE_EXPORT_BIZ_DATA_DOCKER_TAG HEDGE_EXPORT_DOCKER_TAG HEDGE_EVENT_DOCKER_TAG HEDGE_UI_SERVER_DOCKER_TAG HEDGEXT_GRAFANA_DOCKER_TAG \
		HEDGEXT_NODE_RED_DOCKER_TAG HEDGEXT_EKUIPER_DOCKER_TAG HEDGEXT_CORE_COMMAND_DOCKER_TAG HEDGE_DATA_ENRICHMENT_DOCKER_TAG HEDGE_METADATA_NOTIFIER_DOCKER_TAG \
		HEDGE_ML_EDGE_AGENT_DOCKER_TAG HEDGE_ML_BROKER_DOCKER_TAG HEDGE_ML_PRED_ANOMALY_AUTOENCODER_DOCKER_TAG HEDGE_ML_PRED_CLASSIFICATION_RANDOMFOREST_DOCKER_TAG \
		HEDGE_ML_PRED_TIMESERIES_MULTIVARIATE_DEEPVAR_DOCKER_TAG HEDGE_ML_PRED_REGRESSION_LIGHTGBM_DOCKER_TAG HEDGE_DEVICE_VIRTUAL_DOCKER_TAG HEDGE_SWAGGER_UI_DOCKER_TAG \
		HEDGE_ML_SANDBOX_DOCKER_TAG HEDGE_DIGITAL_TWIN_DOCKER_TAG

define check_and_set_tag
	$(eval temp_tag := $(strip $($(1))))
	$(if $(temp_tag), \
		$(info $(1) overridden version: $(temp_tag)), \
		$(eval temp_tag := $(DOCKER_TAG)) \
	) \
	$(shell echo $(1)=$(temp_tag) >> $(ENV-FILE))
endef

### VALIDATE ###
# Checks if CURRENT_HEDGE_NODE_TYPE is a valid type
# If its a CORE setup then REMOTE_* parameters should be empty
# If its a NODE only setup (exact match) then REMOTE_* parameters should have valid values
# Validate CURRENT_* parameters in .env for both node and core
define VALIDATIONS
	$(info CURRENT_HEDGE_NODE_TYPE is $(CURRENT_HEDGE_NODE_TYPE))

	$(if $(or $(findstring CORE,$(CURRENT_HEDGE_NODE_TYPE)), $(findstring NODE,$(CURRENT_HEDGE_NODE_TYPE))),, \
		$(error CURRENT_HEDGE_NODE_TYPE should be either 'CORE', 'NODE', or 'CORE_NODE'))

	$(if $(findstring CORE,$(CURRENT_HEDGE_NODE_TYPE)), \
		$(if $(REMOTE_HEDGE_CORE_SERVER_NAME),$(error REMOTE_HEDGE_CORE_SERVER_NAME should be empty in .env-core for Hedge CORE setup),) \
		$(if $(REMOTE_HEDGE_CORE_DOMAIN_NAME),$(error REMOTE_HEDGE_CORE_DOMAIN_NAME should be empty in .env-core for Hedge CORE setup),) \
		$(if $(REMOTE_HEDGE_CORE_SERVER_IP),$(error REMOTE_HEDGE_CORE_SERVER_IP should be empty in .env-core for Hedge CORE setup),) \
		$(if $(NODE_TOPIC_PREFIX),$(error NODE_TOPIC_PREFIX should be empty in .env-core for Hedge CORE setup,)) \
		,)

	$(if $(filter NODE,$(CURRENT_HEDGE_NODE_TYPE)), \
		$(if $(filter clm-yyy-yyyyyy,$(REMOTE_HEDGE_CORE_SERVER_NAME)),$(error REMOTE_HEDGE_CORE_SERVER_NAME has placeholder value clm-yyy-yyyyyy in .env. This should be changed to remote Hedge CORE server's host name),) \
		$(if $(REMOTE_HEDGE_CORE_SERVER_NAME),,$(error REMOTE_HEDGE_CORE_SERVER_NAME is mandatory in .env for Hedge NODE setup)) \
		\
		$(if $(filter .mycoredomain.com,$(REMOTE_HEDGE_CORE_DOMAIN_NAME)),$(error REMOTE_HEDGE_CORE_DOMAIN_NAME has placeholder value .mycoredomain.com in .env. This should be changed to remote Hedge CORE server's domain name),) \
		$(if $(REMOTE_HEDGE_CORE_DOMAIN_NAME),,$(error REMOTE_HEDGE_CORE_DOMAIN_NAME is mandatory in .env for Hedge NODE setup)) \
		\
		$(if $(filter Y.Y.Y.Y,$(REMOTE_HEDGE_CORE_SERVER_IP)),$(error REMOTE_HEDGE_CORE_SERVER_IP has placeholder value Y.Y.Y.Y in .env. This should be changed to remote Hedge CORE server's IP address),) \
		$(if $(REMOTE_HEDGE_CORE_SERVER_IP),,$(error REMOTE_HEDGE_CORE_SERVER_IP is mandatory in .env for Hedge NODE setup)) \
		\
		$(if $(filter ${CURRENT_HEDGE_NODE_SERVER_NAME}/,$(NODE_TOPIC_PREFIX)),,$(error NODE_TOPIC_PREFIX in .env should be set to "$${CURRENT_HEDGE_NODE_SERVER_NAME}/" for Hedge NODE setup)) \
		$(if $(MQTT_ENCPWD_INP_FILE),,$(error MQTT_ENCPWD_INP_FILE env variable in .env should be set to the file containing MQTT encrypted password)) \
	)

	$(if $(filter clm-xxx-xxxxxx,$(CURRENT_HEDGE_NODE_SERVER_NAME)), \
		$(error CURRENT_HEDGE_NODE_SERVER_NAME has placeholder value clm-xxx-xxxxxx in .env. This should be changed to this host's name),)
	$(if $(CURRENT_HEDGE_NODE_SERVER_NAME),,$(error CURRENT_HEDGE_NODE_SERVER_NAME is mandatory in .env))

	$(if $(filter .mydomain.com,$(CURRENT_HEDGE_NODE_SERVER_DOMAIN_NAME)), \
		$(error CURRENT_HEDGE_NODE_SERVER_DOMAIN_NAME has placeholder value .mydomain.com in .env. This should be changed to this host's valid domain name),)
	$(if $(CURRENT_HEDGE_NODE_SERVER_DOMAIN_NAME),,$(error CURRENT_HEDGE_NODE_SERVER_DOMAIN_NAME is mandatory in .env))

	$(if $(filter X.X.X.X,$(CURRENT_HEDGE_NODE_SERVER_IP)), \
		$(error CURRENT_HEDGE_NODE_SERVER_IP has placeholder value X.X.X.X in .env. This should be changed to this host's IP address),)
	$(if $(CURRENT_HEDGE_NODE_SERVER_IP),,$(error CURRENT_HEDGE_NODE_SERVER_IP is mandatory in .env))
endef

#$(call VALIDATIONS)

### PREP FOR make run ###
DCOMPOSE-FILES:=docker-compose-common.yml
HEDGE-LOG:=

## if type contains CORE - ie CORE or CORE_NODE setup
ifneq (,$(findstring CORE,$(CURRENT_HEDGE_NODE_TYPE)))
DCOMPOSE-FILES:=$(DCOMPOSE-FILES) -f docker-compose-core.yml
HEDGE-LOG:=$(HEDGE-LOG) CORE
endif #ends if type=CORE

## if type contains NODE - ie NODE or CORE_NODE
ifneq (,$(findstring NODE,$(CURRENT_HEDGE_NODE_TYPE)))
DCOMPOSE-FILES:=$(DCOMPOSE-FILES) -f docker-compose-node.yml
HEDGE-LOG:=$(HEDGE-LOG) NODE
endif #ends if type contains NODE

## if ENV=FLEET
ifneq (,$(findstring FLEET,$(ENV)))
DCOMPOSE-FILES:=$(DCOMPOSE-FILES) -f fleet/docker-compose.fleet.yml
HEDGE-LOG:=$(HEDGE-LOG) FLEET
endif

## if ENV=PUMP
ifneq (,$(findstring PUMP,$(ENV)))
DCOMPOSE-FILES:=$(DCOMPOSE-FILES) -f petrolpumps/docker-compose.petrolpumps.yml
HEDGE-LOG:=$(HEDGE-LOG) PUMP
endif

# Add additional parameters to docker compose down, if it is a CORE package
# Add docker-compose-core.yml to docker compose cmd if not already added to the env DCOMPOSE-FILES ('make ENV=CORE ..' auto-adds this)
# When stopping services stop all irrespective of node/core/optional and other profiles
define COMPOSE_DOWN
	if [ -f ./.env-core ]; then \
		cat scripts/envheader .env-core >> $(ENV-FILE); \
	fi; \
	if [ -f ./docker-compose-core.yml ] && ! echo "$(DCOMPOSE-FILES)" | grep -q "docker-compose-core.yml"; then \
		printf "CMD: COMPOSE_PROFILES=all docker compose -p edgex -f $(DCOMPOSE-FILES) -f docker-compose-core.yml --env-file $(ENV-FILE) down $1\n"; \
		COMPOSE_PROFILES=all docker compose -p edgex -f $(DCOMPOSE-FILES) -f docker-compose-core.yml --env-file $(ENV-FILE) down $1 || true; \
	else \
		printf "CMD: COMPOSE_PROFILES=all docker compose -p edgex -f $(DCOMPOSE-FILES) --env-file $(ENV-FILE) down $1\n"; \
		COMPOSE_PROFILES=all docker compose -p edgex -f $(DCOMPOSE-FILES) --env-file $(ENV-FILE) down $1 || true; \
	fi; \
	printf "Stopping and removing containers in edgex_edgex-network which are not defined in docker-compose...\n"; \
	containers=$$(docker ps -aq --filter "network=edgex_edgex-network"); \
	if [ -n "$$containers" ]; then \
		echo "$$containers" | while read container_id; do \
			container_name=$$(docker inspect --format '{{.Name}}' $$container_id | sed 's|/||'); \
			docker stop $$container_id >/dev/null || true; \
			docker rm -f $$container_id >/dev/null || true; \
			printf "Removed container: $$container_name\n"; \
		done; \
	else \
		printf "No additional containers found in edgex_edgex-network.\n"; \
	fi; \
	if docker network ls | grep -q edgex_edgex-network; then \
		printf "Removing network: edgex_edgex-network...\n"; \
		docker network rm edgex_edgex-network || true; \
	else \
		printf "Network edgex_edgex-network does not exist.\n"; \
	fi
endef

# Define additional phony targets for all options to enable support for tab-completion in shell
# Note: This must be defined after the options are parsed otherwise it will interfere with them
.PHONY: $(OPTIONS)

pull:
	# Apply service tag overrides and update .env-generated
	$(foreach tag,$(svc_tag_overrides), \
		$(eval $(call check_and_set_tag,$(tag))))
	if [ -n "${SERVICES}" ]; then \
		if echo "$(PULL_ONLY_IMAGES)" | grep -qw "${SERVICES}"; then \
			$(eval tag_var := $(shell echo ${SERVICES} | tr '-' '_' | tr a-z A-Z)_DOCKER_TAG) \
			$(eval tag := $($(tag_var))) \
			$(if $(strip $(tag)), \
				tag=$(strip $(tag)), \
				tag=$(DOCKER_TAG)); \
			printf "Pulling out-of-box inference image: ${REGISTRY}${SERVICES}:$${tag}\n"; \
			docker pull ${REGISTRY}${SERVICES}:$${tag}; \
		else \
			printf "CMD: ${OPT-PARAM} docker compose -f ${DCOMPOSE-FILES} --env-file ${ENV-FILE} pull ${SERVICES}\n"; \
			${OPT-PARAM} docker compose -f ${DCOMPOSE-FILES} --env-file ${ENV-FILE} pull ${SERVICES}; \
		fi; \
	else \
		printf "Pulling all images.\n"; \
		${OPT-PARAM} docker compose -f ${DCOMPOSE-FILES} --env-file ${ENV-FILE} pull; \
		$(foreach image,$(PULL_ONLY_IMAGES), \
			$(eval tag_var := $(shell echo $(image) | tr '-' '_' | tr a-z A-Z)_DOCKER_TAG) \
			$(eval tag := $($(tag_var))) \
			$(if $(strip $(tag)), \
				tag=$(strip $(tag)), \
				tag=$(DOCKER_TAG)); \
			printf "Pulling out-of-box inference image: ${REGISTRY}$(image):$${tag}\n"; \
			docker pull ${REGISTRY}$(image):$${tag}; \
		) \
	fi

pre_requisites:
	@missing_pkgs="" ; \
	for pkg in $(REQUIRED_PACKAGES); do \
		if ! command -v $$pkg >/dev/null 2>&1; then \
			missing_pkgs="$$missing_pkgs $$pkg"; \
		fi; \
	done; \
	if [ -n "$$missing_pkgs" ]; then \
		printf "ERROR: Missing required packages: $$missing_pkgs\n"; \
		printf "Please install them before proceeding.\n\n"; \
		exit 1; \
	else \
		printf "All required packages are installed.\n"; \
	fi

user_interact:
	@./scripts/license_agrmt.sh || (printf "ERROR: LICENSE agreement NOT Accepted. Abort Installation\n"; exit 1)

platcrd:
		@if echo "${CURRENT_HEDGE_NODE_TYPE}" | grep -q "CORE"; then ./scripts/platcrd.sh "${IS_EXTERNAL_AUTH}"; fi

pre-run: pre_requisites user_interact platcrd
	$(call VALIDATIONS)
	$(info Deploying services with tag: $(DOCKER_TAG))
	# Apply service tag overrides and update .env-generated
	$(foreach tag,$(svc_tag_overrides), \
		$(eval $(call check_and_set_tag,$(tag))))
	if [ "${SERVICES}" = "" ]; then \
		printf "\nAbout to bootstrap hedge-secrets, make sure your environment vars have them\n"; \
		[ -d ${MQTT_ENCPWD_INP_FILE} ] && rm -rf ${MQTT_ENCPWD_INP_FILE} && printf "Directory with the name ${MQTT_ENCPWD_INP_FILE} found. Deleting..\n"; \
		[ ! -f ${MQTT_ENCPWD_INP_FILE} ] && touch ${MQTT_ENCPWD_INP_FILE} && printf "Touch file ${MQTT_ENCPWD_INP_FILE} created\n" || printf "Secret file ${MQTT_ENCPWD_INP_FILE} already exists..\n"; \
		[ -d ${CNSL_ENCTKN_INP_FILE} ] && rm -rf ${CNSL_ENCTKN_INP_FILE} && printf "Directory with the name ${CNSL_ENCTKN_INP_FILE} found. Deleting..\n"; \
		[ ! -f ${CNSL_ENCTKN_INP_FILE} ] && touch ${CNSL_ENCTKN_INP_FILE} && printf "Touch file ${CNSL_ENCTKN_INP_FILE} created\n" || printf "Secret file ${CNSL_ENCTKN_INP_FILE} already exists..\n"; \
		\
		printf "CMD: ${OPT-PARAM} docker compose -p edgex -f ${DCOMPOSE-FILES} --env-file ${ENV-FILE} up -d security-bootstrapper\n"; \
		${OPT-PARAM} docker compose -p edgex -f ${DCOMPOSE-FILES} --env-file ${ENV-FILE} up -d security-bootstrapper; \
		printf "CMD: EXECUTION_MODE=\"SEED_SECRETS\" docker compose -p edgex -f docker-compose-common.yml --env-file ${ENV-FILE} run --rm hedge-secret-bootstrapper\n"; \
		EXECUTION_MODE="SEED_SECRETS" docker compose -p edgex -f docker-compose-common.yml --env-file ${ENV-FILE} run --rm hedge-secret-bootstrapper; \
		ret=$$?; \
		if [ $$ret -eq 0 ]; then \
			printf "hedge-secrets bootstrapping completed successfully\n"; \
		else \
			printf "hedge-secrets bootstrapping completed with failure. Stopping..\n"; \
			exit $$ret; \
		fi; \
		printf "\nAbout to start common config bootstrapper\n"; \
		printf "CMD: docker compose -p edgex -f docker-compose-common.yml --env-file ${ENV-FILE} up -d core-common-config-bootstrapper redis\n"; \
		docker compose -p edgex -f docker-compose-common.yml --env-file ${ENV-FILE} up -d core-common-config-bootstrapper redis; \
		ret=$$?; \
		if [ $$ret -ne 0 ]; then \
			printf "pre-run completed with failure. Stopping..\n"; \
			exit $$ret; \
		fi; \
		printf "\nWait until Vault has been configured successfully\n"; \
		while [ "`docker exec edgex-vault vault status 2>/dev/null | grep Sealed | awk '{print $$2}'`" != "false" ]; do sleep 2; printf "Waiting for edgex-vault to start...\n"; done; \
		while [ "`docker exec edgex-security-secretstore-setup test -f /vault/config/assets/resp-init.json`" ]; do sleep 2; printf "Waiting for edgex-security-secretstore-setup to start..\n"; done; \
		vkeys=$$(docker exec edgex-security-secretstore-setup cat /vault/config/assets/resp-init.json | jq -r '.keys[:3]'); \
		key1=$$(echo $$vkeys | jq -r '.[0]'); key2=$$(echo $$vkeys | jq -r '.[1]'); key3=$$(echo $$vkeys | jq -r '.[2]'); \
		if [ -z "$$key1" ] || [ -z "$$key2" ] || [ -z "$$key3" ]; then \
			printf "ERROR: Failed fetching the Vault keys"; \
			exit 1; \
		fi; \
		docker exec edgex-redis sh -c \
			"sed -i 's/__VAULT_KEY1__/$$key1/' /tmp/hedge-secrets/hedge_admin_secrets.json; \
			 sed -i 's/__VAULT_KEY2__/$$key2/' /tmp/hedge-secrets/hedge_admin_secrets.json; \
			 sed -i 's/__VAULT_KEY3__/$$key3/' /tmp/hedge-secrets/hedge_admin_secrets.json"; \
		printf "Vault successfully started and initialized\n"; \
		printf "\nWait until common bootstrapping has completed successfully\n"; \
		while [ "`docker inspect -f {{.State.Status}} edgex-core-common-config-bootstrapper`" != "exited" ]; do sleep 2; printf "Waiting for edgex-core-common-config-bootstrapper to complete...\n"; done; \
		printf "pre-run completed successfully\n"; \
		printf "\n======\n"; \
		exit $$ret; \
	fi

run: pre-run
	printf "\n**************************************************"; \
	printf "\n Setting up Helix IoT Edge \"$(HEDGE-LOG) \""; \
	printf "\n Make sure the ${ENV-FILE} file is correctly updated"; \
	printf "\n**************************************************\n"; \
	printf "CMD: ${OPT-PARAM} docker compose -p edgex -f ${DCOMPOSE-FILES} --env-file ${ENV-FILE} up -d ${SERVICES}\n"; \
	${OPT-PARAM} docker compose -p edgex -f ${DCOMPOSE-FILES} --env-file ${ENV-FILE} up -d ${SERVICES}; \
	rm -f ${ENV-FILE}; \
	$(call HEDGE_DEPLOYMENT_COMPLETE, $(CURRENT_HEDGE_NODE_TYPE))

update-secrets:
	if [ "$(CURRENT_HEDGE_NODE_TYPE)" = "NODE" ]; then \
		printf "ERROR: update-secrets cannot be executed on a Helix Edge NODE machine\n\n"; \
		exit 1; \
	fi

	$(foreach tag,$(svc_tag_overrides), \
		$(eval $(call check_and_set_tag,$(tag))))

	printf "CMD: EXECUTION_MODE=\"UPDATE_SECRETS\" docker compose -p edgex -f docker-compose-common.yml --env-file ${ENV-FILE} run --rm hedge-secret-bootstrapper\n"
	EXECUTION_MODE="UPDATE_SECRETS" docker compose -p edgex -f docker-compose-common.yml --env-file ${ENV-FILE} run --rm hedge-secret-bootstrapper
	printf "\nHelix Edge services needs to be restarted for the changes to take effect. \nRestarting in 5 sec..\n\n"
	sleep 5
	printf "\n===========================================================================";
	printf "\n===========================================================================\n\n";
	printf "Helix Edge Services stopping...\n\n";
	$(MAKE) down;
	printf "\n===========================================================================";
	printf "\n===========================================================================\n\n";
	printf "Helix Edge Services stopped. Now starting again...\n\n";
	sleep 2;
	printf "CMD: make run ENV=${ENV} PROFILE=${PROFILE}\n";
	$(MAKE) run ENV=${ENV} PROFILE=${PROFILE};
	printf "Helix Edge Services started...\n";
	printf "Update Secrets operation complete. Please wait for services to stabilize\n\n";

define HEDGE_DEPLOYMENT_COMPLETE
		printf "\n\nSuccessfully deployed \"BMC Helix Edge\" - $(1).\n\n";
		@if [ "$(1)" = "NODE" ]; then \
				exit 0; \
		fi; \
		printf "===========================================================================\n"; \
		printf "===========================================================================\n\n"; \
		printf "  Make sure all the containers have stabilized before proceeding ahead..\n"; \
		printf "  ---\n"; \
		printf "  Next Steps:\n"; \
		printf "    1. Setup your HEDGE NODE(s) with these .env parameters:\n"; \
		printf "           REMOTE_HEDGE_CORE_SERVER_NAME=${CURRENT_HEDGE_NODE_SERVER_NAME}\n"; \
		printf "           REMOTE_HEDGE_CORE_DOMAIN_NAME=${CURRENT_HEDGE_NODE_SERVER_DOMAIN_NAME}\n"; \
		printf "           REMOTE_HEDGE_CORE_SERVER_IP=${CURRENT_HEDGE_NODE_SERVER_IP}\n\n"; \
		printf "    2. Download and copy these secret files to /tmp/ location of your NODE(s) before starting the Hedge NODE setup:\n"; \
		printf "           ${MQTT_ENCPWD_INP_FILE}\n"; \
		printf "           ${CNSL_ENCTKN_INP_FILE}\n"; \
		printf "           ${NATS_TLS_KEY}\n"; \
		printf "           ${NATS_TLS_CRT}\n\n"; \
		printf "===========================================================================\n"; \
		printf "===========================================================================\n\n"
endef

force-clean:
	printf "\n";
	@read -p "Are you sure you want to delete all volumes and stop containers in edgex_edgex-network (this cannot be reverted)? - [YES/NO]: " var; \
	printf ">>$$var\n"; \
	if [ "$$var" = "YES" ]; then \
		printf "Force Cleaning... containers + volumes + networks\n"; \
		$(call COMPOSE_DOWN,-v); \
		printf "Deleting named volumes prefixed with 'edgex_'\n"; \
		volumes=$$(docker volume ls -q | grep '^edgex_'); \
		if [ -n "$$volumes" ]; then \
			docker volume rm -f $$volumes; \
		else \
			printf "No named volumes prefixed with 'edgex_' to delete.\n"; \
		fi; \
		printf "Deleting secret files: ${MQTT_ENCPWD_INP_FILE} ${CNSL_ENCTKN_INP_FILE} ${NATS_TLS_CRT} ${NATS_TLS_KEY}\n\n"; \
		rm -f ${MQTT_ENCPWD_INP_FILE} ${CNSL_ENCTKN_INP_FILE} ${NATS_TLS_CRT} ${NATS_TLS_KEY}; \
		rm -f ${ENV-FILE}; \
		rm -f ".license_accepted"; \
		rm -f ".platcrd"; \
	else \
		printf "Cancelled...\n\n"; \
	fi

down:
	$(COMPOSE_DOWN)

get-vault-token:
	@./scripts/genvaulttok.sh || (printf "ERROR: Vault token generate/revoke failed\n"; exit 1)

#get-consul-bootstrap-token:
#	docker exec edgex-core-consul /bin/sh -c \
#		'cat "$$STAGEGATE_REGISTRY_ACL_BOOTSTRAPTOKENPATH" | jq -r '.SecretID' '

get-consul-token:
	@CONSUL_TOKEN=$$(docker exec edgex-core-consul /bin/sh -c 'cat /tmp/hedge-secrets/.consultoken'); \
	printf "\n===========================================================================\n\n"; \
	printf "\tCONSUL TOKEN = $$CONSUL_TOKEN\n"; \
	printf "\n===========================================================================\n\n"

get-node-secrets:
	printf "CMD: EXECUTION_MODE=\"GET_SECRETS\" CURRENT_HEDGE_NODE_TYPE=CORE_NODE docker compose -p edgex -f docker-compose-common.yml --env-file ${ENV-FILE} run --rm hedge-secret-bootstrapper\n"
	EXECUTION_MODE="GET_SECRETS" CURRENT_HEDGE_NODE_TYPE=CORE_NODE docker compose -p edgex -f docker-compose-common.yml --env-file ${ENV-FILE} run --rm hedge-secret-bootstrapper;
	rm -f ${ENV-FILE};
	$(call HEDGE_DEPLOYMENT_COMPLETE, CORE_NODE);

get-version:
	printf "\tHedge Release Version: \t${HEDGE_RELEASE_VERSION}\n"
	printf "\tHedge Image Tag: \t${DOCKER_IMG_VERSION}\n"
