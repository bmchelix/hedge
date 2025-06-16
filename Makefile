# ==============================================================================
# Makefile for building Docker images with service directory-only hash detection
# ==============================================================================

# Ensure infra env vars

DOCKER_REGISTRY ?= docker.io/bmchelix
DOCKER_IMG_VERSION=$(shell cat ./VERSION 2>/dev/null || echo latest)
DOCKER_TAG ?= $(DOCKER_IMG_VERSION)
GIT_SHA := $(shell git rev-parse --short HEAD)
DOCKER_VERSION := $(shell docker version --format '{{.Server.Version}}')
BUILDX_VERSION := $(shell docker buildx version)
TODAY := $(shell date -I)
DOCKER_LABELS = \
				--label "git_sha=$(GIT_SHA)" \
				--label "copyright=(c) as per Apache 2.0." \
				--label "contributors=BMC Helix, Inc." \
				--label "hedge-version=$(DOCKER_TAG)" \
				--label "hedge-base-docker-version=$(DOCKER_VERSION)" \
				--label "hedge-base-buildx-version=$(BUILDX_VERSION)" \
				--label "hedge-base-build-date=$(TODAY)" \
				--label "license='Apache 2.0'"

# --- Base Versions and Docker Options ---

GO_BASE=golang:1.24.2-alpine3.21
NODE_BASE=node:23.11.0-alpine3.21
ALPINE_BASE=alpine:3.21.3
PYTHON_BASE := $(DOCKER_REGISTRY)/hedge-ml-python-base:$(DOCKER_TAG)
DOCKER_BASE=docker:dind-rootless
_DOCKER_BUILD_FLAGS_RAW := --platform linux/amd64
DOCKER_BUILD_FLAGS := $(strip $(_DOCKER_BUILD_FLAGS_RAW))
EDGEX_VERSION=3.1.0

# --- Hash-based Change Detection Configuration ---
HASHCACHE_DIR := .hashcache
IGNORE_PATTERNS := -name .hashcache -prune -o -name .git -prune -o -name node_modules -prune -o -name dist -prune -o -name build -prune -o -name vendor -prune

# --- User Configurable Options ---
IMAGE_PUSH ?= false
FORCE_DOCKER_BUILD ?= false

# --- External Service Versions ---
ANGULAR_VERSION=18.2.6
#NATS_VERSION=2.10.26-alpine3.21
#CONSUL_VERSION=1.20.2
#VAULT_VERSION=1.18.5
REDIS_VERSION=7.4.0-r0
#MOSQUITTO_VERSION=2.0.21
NGINX_VERSION=1.27.3-alpine
#GRAFANA_VERSION=11.6.1
NODERED_VERSION=4.0.7-22-minimal
#KUIPER_VERSION=2.0.7-alpine
#ELASTIC_SEARCH_VERSION=3
#VICTORIA_METRICS_VERSION=v1.115.0
#POSTGRES_VERSION=17.4-alpine3.21

# --- Consolidated Docker Build Arguments ---
DOCKER_BUILDARGS = \
	--build-arg http_proxy \
	--build-arg https_proxy \
	--build-arg GO_BASE=$(GO_BASE) \
	--build-arg NODE_BASE=$(NODE_BASE) \
	--build-arg ALPINE_BASE=$(ALPINE_BASE) \
	--build-arg PYTHON_BASE=$(PYTHON_BASE) \
	--build-arg REDIS_VERSION=$(REDIS_VERSION) \
	--build-arg NGINX_VERSION=$(NGINX_VERSION) \
	--build-arg NODERED_VERSION=$(NODERED_VERSION) \
	$(DOCKER_BUILD_FLAGS)

# --- Service Definitions and Grouping ---
_APP_SERVICES_PATHS_RAW := \
	app-services/hedge-admin \
	app-services/hedge-data-enrichment \
	app-services/hedge-event \
	app-services/hedge-device-extensions \
	app-services/hedge-event-publisher \
	app-services/hedge-export \
	app-services/hedge-remediate \
	app-services/hedge-meta-sync \
	app-services/hedge-metadata-notifier \
	app-services/hedge-nats-proxy \
	app-services/hedge-user-app-mgmt

# define the service dependency directories im here, follow the convention
hedge-admin-deps := app-services/hedge-admin common
hedge-data-enrichment-deps := app-services/hedge-data-enrichment common
hedge-event-deps := app-services/hedge-event common
hedge-device-extensions-deps := app-services/hedge-device-extensions common
hedge-event-publisher-deps := app-services/hedge-event-publisher common
hedge-export-deps := app-services/hedge-export common
hedge-remediate-deps := app-services/hedge-remediate common
hedge-meta-sync-deps := app-services/hedge-meta-sync common
hedge-metadata-notifier-deps := app-services/hedge-metadata-notifier common
hedge-nats-proxy-deps := app-services/hedge-nats-proxy
hedge-user-app-mgmt-deps := app-services/hedge-user-app-mgmt common

_DEVICE_SERVICES_PATHS_RAW := \
	device-services/hedge-device-virtual

# define the service dependency directories im here, follow the convention
hedge-device-virtual-deps := device-services/hedge-device-virtual common

_ML_SERVICES_PATHS_RAW := \
	edge-ml-service/cmd/hedge-ml-management \
	edge-ml-service/cmd/hedge-ml-sandbox \
	edge-ml-service/cmd/hedge-ml-broker \
	edge-ml-service/cmd/hedge-ml-edge-agent

hedge-ml-management-deps :=	edge-ml-service/cmd/hedge-ml-management edge-ml-service/internal edge-ml-service/pkg common
hedge-ml-sandbox-deps := edge-ml-service/cmd/hedge-ml-sandbox edge-ml-service/internal edge-ml-service/pkg common
hedge-ml-broker-deps := edge-ml-service/cmd/hedge-ml-broker edge-ml-service/internal edge-ml-service/pkg common
hedge-ml-edge-agent-deps := edge-ml-service/cmd/hedge-ml-edge-agent edge-ml-service/internal edge-ml-service/pkg common

_INIT_SERVICES_PATHS_RAW := \
	hedge-init \
	hedge-swagger-ui

hedge-init-deps := hedge-init
hedge-swagger-ui-deps := hedge-swagger-ui

# --- service name to Dockerfile path mapping for python code, ensure python base is always first---
ML_PYTHON_SERVICE_MAP = \
    hedge-ml-python-base:base-image:base-image/Dockerfile \
	hedge-ml-trg-anomaly-autoencoder:anomaly/autoencoder/train:anomaly/autoencoder/train/Dockerfile \
	hedge-ml-pred-anomaly-autoencoder:anomaly/autoencoder/infer:anomaly/autoencoder/infer/Dockerfile \
	hedge-ml-trg-classification-randomforest:classification/random_forest/train:classification/random_forest/train/Dockerfile \
	hedge-ml-pred-classification-randomforest:classification/random_forest/infer:classification/random_forest/infer/Dockerfile \
	hedge-ml-trg-timeseries-multivariate-deepvar:timeseries/multivariate_deepVAR/train:timeseries/multivariate_deepVAR/train/Dockerfile \
	hedge-ml-pred-timeseries-multivariate-deepvar:timeseries/multivariate_deepVAR/infer:timeseries/multivariate_deepVAR/infer/Dockerfile \
	hedge-ml-trg-regression-lightgbm:regression/lightgbm/train:regression/lightgbm/train/Dockerfile \
	hedge-ml-pred-regression-lightgbm:regression/lightgbm/infer:regression/lightgbm/infer/Dockerfile \
	hedge-ml-pred-regression-lightgbm:regression/lightgbm/infer:regression/lightgbm/infer/Dockerfile

_ml_python_service_ids := $(foreach def,$(ML_PYTHON_SERVICE_MAP),$(firstword $(subst :, ,$(def))))


hedge-ml-python-base-deps := edge-ml-service/python-code/base-image
hedge-ml-trg-anomaly-autoencoder-deps := edge-ml-service/python-code/anomaly/autoencoder/train edge-ml-service/python-code/common
hedge-ml-pred-anomaly-autoencoder-deps := edge-ml-service/python-code/anomaly/autoencoder/infer edge-ml-service/python-code/common
hedge-ml-trg-classification-randomforest-deps := edge-ml-service/python-code/classification/random_forest/train edge-ml-service/python-code/common
hedge-ml-pred-classification-randomforest-deps := edge-ml-service/python-code/classification/random_forest/infer edge-ml-service/python-code/common
hedge-ml-trg-timeseries-multivariate-deepvar-deps := edge-ml-service/python-code/timeseries/multivariate_deepVAR/train edge-ml-service/python-code/common
hedge-ml-pred-timeseries-multivariate-deepvar-deps := edge-ml-service/python-code/timeseries/multivariate_deepVAR/infer edge-ml-service/python-code/common
hedge-ml-trg-regression-lightgbm-deps := edge-ml-service/python-code/regression/lightgbm/train edge-ml-service/python-code/common
hedge-ml-pred-regression-lightgbm-deps := edge-ml-service/python-code/regression/lightgbm/infer edge-ml-service/python-code/common

# --- EdgeX Configuration ---
EDGEX_SUBMODULE_PATH = external/edgex-go-submodule
edgex-services-deps := external/edgex-go-submodule

# For external services, similar to what we did for ml-python-services
# --- service name to Dockerfile path mapping for python code, ensure python base is always first---

_EXTERNAL_SERVICES_PATHS_RAW := \
	external/nginx \
	external/node-red \
	external/redis

# dependencies for external services
hedge-ext-nginx-deps :=	external/nginx external/edgex-go-submodule
hedge-ext-node-red-deps := external/node-red
hedge-ext-redis-deps :=	external/redis external/edgex-go-submodule

# Cleaned service paths
APP_SERVICES_PATHS := $(foreach p,$(_APP_SERVICES_PATHS_RAW),$(strip $(p)))
DEVICE_SERVICES_PATHS := $(foreach p,$(_DEVICE_SERVICES_PATHS_RAW),$(strip $(p)))
EXTERNAL_SERVICES_PATHS := $(foreach p,$(_EXTERNAL_SERVICES_PATHS_RAW),$(strip $(p)))
ML_MGMT_SERVICES_PATHS := $(foreach p,$(_ML_SERVICES_PATHS_RAW),$(strip $(p)))
INIT_SERVICES_PATHS := $(foreach p,$(_INIT_SERVICES_PATHS_RAW),$(strip $(p)))

# Service IDs
APP_SERVICE_IDS := $(foreach p,$(APP_SERVICES_PATHS),$(notdir $(p)))
DEVICE_SERVICE_IDS := $(foreach p,$(DEVICE_SERVICES_PATHS),$(notdir $(p)))
# external service names are of the form hedge-ext-nginx etc
EXTERNAL_SERVICE_IDS := $(foreach p,$(EXTERNAL_SERVICES_PATHS),$(addprefix hedge-ext-,$(notdir $(p))))
ML_MGMT_SERVICE_IDS := $(foreach p,$(ML_MGMT_SERVICES_PATHS),$(notdir $(p)))
INIT_SERVICE_IDS := $(foreach p,$(INIT_SERVICES_PATHS),$(notdir $(p)))


# --- Core Build Rules with Service Directory Hash Detection ---
define BUILD_DOCKER_SERVICE
_SERVICE_ID := $(strip $(1))
_DOCKERFILE_PATH := $(strip $(2))
_SERVICE_DIR := $(strip $(dir $(_DOCKERFILE_PATH)))

build-$(_SERVICE_ID):
	@set -e; \
	mkdir -p $(HASHCACHE_DIR); \
	echo "Checking changes for $(_SERVICE_ID)..."; \
	echo " dependencies : $($(_SERVICE_ID)-deps)"; \
	( \
		LC_ALL=C find $($(_SERVICE_ID)-deps) \( $(IGNORE_PATTERNS) \) -o -type d -print | LC_ALL=C sort; \
		LC_ALL=C find $($(_SERVICE_ID)-deps) \( $(IGNORE_PATTERNS) \) -o -type f -exec sha256sum {} \; | LC_ALL=C sort -k2; \
	) | sha256sum | awk '{print $$1}' > $(HASHCACHE_DIR)/$(_SERVICE_ID).tmp; \
	if [ "$(FORCE_DOCKER_BUILD)" = "true" ] || \
	   ! [ -f $(HASHCACHE_DIR)/$(_SERVICE_ID).hash ] || \
	   ! cmp -s $(HASHCACHE_DIR)/$(_SERVICE_ID).hash $(HASHCACHE_DIR)/$(_SERVICE_ID).tmp; then \
		echo "Building $(_SERVICE_ID)..."; \
		if echo "$(_SERVICE_DIR)" | grep -q "\python\-"; then \
            echo "Service $(_SERVICE_ID) is ML-related. Pulling base image $(PYTHON_BASE)"; \
            docker pull $(PYTHON_BASE); \
        fi; \
        if echo "$(_SERVICE_ID)" | grep -q "edgex\-services"; then \
            echo "Service $(_SERVICE_ID) is edgex services, so calling build-edgex-services target"; \
            make build_edgex_services; \
            make push-edgex-images; \
        else \
            echo "building docker: $(_SERVICE_ID)"; \
            docker build \
                $(DOCKER_LABELS) \
                $(DOCKER_BUILDARGS) \
                --label "Name=$(_SERVICE_ID)" \
                -f "$(_DOCKERFILE_PATH)" \
                -t $(_SERVICE_ID):$(DOCKER_TAG) \
                -t $(DOCKER_REGISTRY)/$(_SERVICE_ID):$(GIT_SHA) \
                -t $(DOCKER_REGISTRY)/$(_SERVICE_ID):$(DOCKER_TAG) \
                . ; \
            if [ "$(IMAGE_PUSH)" = "true" ]; then \
                echo "Pushing $(DOCKER_REGISTRY)/$(_SERVICE_ID):$(DOCKER_TAG).." && \
                docker push $(DOCKER_REGISTRY)/$(_SERVICE_ID):$(GIT_SHA) && \
                docker push $(DOCKER_REGISTRY)/$(_SERVICE_ID):$(DOCKER_TAG); \
            fi; \
        fi; \
        if [ "$(IMAGE_PUSH)" = "true" ]; then \
            mv $(HASHCACHE_DIR)/$(_SERVICE_ID).tmp $(HASHCACHE_DIR)/$(_SERVICE_ID).hash; \
        fi; \
	else \
		echo "No changes detected for $(_SERVICE_ID)"; \
	fi; \
	rm -f $(HASHCACHE_DIR)/$(_SERVICE_ID).tmp
	
endef


## independent push to push specific service
push-$(_SERVICE_ID): build-$(_SERVICE_ID)
	@if [ -f $(HASHCACHE_DIR)/$(_SERVICE_ID).hash ]; then \
		echo "Pushing $(_SERVICE_ID)..."; \
		docker push $(DOCKER_REGISTRY)/$(_SERVICE_ID):$(GIT_SHA); \
		docker push $(DOCKER_REGISTRY)/$(_SERVICE_ID):$(DOCKER_TAG); \
	fi

.PHONY: build-$(_SERVICE_ID) push-$(_SERVICE_ID)

# --- Service Rule Instantiation ---
$(foreach svc_path,$(APP_SERVICES_PATHS),\
	$(eval $(call BUILD_DOCKER_SERVICE,$(notdir $(svc_path)),$(svc_path)/Dockerfile)))

$(foreach svc_path,$(DEVICE_SERVICES_PATHS),\
	$(eval $(call BUILD_DOCKER_SERVICE,$(notdir $(svc_path)),$(svc_path)/Dockerfile)))

$(foreach svc_path,$(EXTERNAL_SERVICES_PATHS),\
	$(eval $(call BUILD_DOCKER_SERVICE,$(notdir $(svc_path)),$(svc_path)/Dockerfile)))

$(foreach svc_path,$(ML_MGMT_SERVICES_PATHS),\
	$(eval $(call BUILD_DOCKER_SERVICE,$(notdir $(svc_path)),$(svc_path)/Dockerfile)))

$(foreach svc_path,$(INIT_SERVICES_PATHS),\
	$(eval $(call BUILD_DOCKER_SERVICE,$(notdir $(svc_path)),$(svc_path)/Dockerfile)))

# to ensure we call build-edgex-services
$(eval $(call BUILD_DOCKER_SERVICE,edgex-services,external/edgex-go-submodule))

# for external services
$(foreach svc_path,$(EXTERNAL_SERVICES_PATHS),\
	$(eval $(call BUILD_DOCKER_SERVICE,$(addprefix hedge-ext-,$(notdir $(svc_path))),$(svc_path)/Dockerfile)))

# --- ML Model Processing ---
define PROCESS_ML_MODEL
$(eval _parts := $(subst :, ,$(1)))
$(eval _service_id := $(word 1,$(_parts)))
$(eval _src_subpath := $(word 2,$(_parts)))
$(eval _dockerfile_subpath := $(word 3,$(_parts)))

$(call BUILD_DOCKER_SERVICE,\
    $(_service_id),\
    edge-ml-service/python-code/$(_dockerfile_subpath))

endef

$(foreach model,$(ML_PYTHON_SERVICE_MAP),\
	$(eval $(call PROCESS_ML_MODEL,$(model))))

# --- EdgeX Build Rules ---
.PHONY: build-edgex ${PUSH_EDGEX_IMAGES} push-edgex-images
build_edgex_services:
	echo "Building EdgeX services..."
	git submodule update --init --recursive --force
	cp $(EDGEX_SUBMODULE_PATH)/Makefile_mod $(EDGEX_SUBMODULE_PATH)/edgex-go/Makefile
	cp $(EDGEX_SUBMODULE_PATH)/common_configuration_hedge.yaml $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/core-common-config-bootstrapper/res/configuration.yaml
	cp $(EDGEX_SUBMODULE_PATH)/secretstore_entrypoint_hedge.sh $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/security-secretstore-setup/entrypoint.sh
	cp $(EDGEX_SUBMODULE_PATH)/security_proxy_setup_hedge.sh $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/security-proxy-setup/entrypoint.sh
	cp $(EDGEX_SUBMODULE_PATH)/security_bootstrapper_entrypoint_hedge.sh $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/security-bootstrapper/entrypoint.sh
	cp $(EDGEX_SUBMODULE_PATH)/redis_wait_install_hedge.sh $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/security-bootstrapper/entrypoint-scripts/redis_wait_install.sh
	cp $(EDGEX_SUBMODULE_PATH)/core-metadata/device.go_hedge $(EDGEX_SUBMODULE_PATH)/edgex-go/internal/core/metadata/application/device.go
	cp $(EDGEX_SUBMODULE_PATH)/security_bootstrapper_redis_configuration_hedge.yaml $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/security-bootstrapper/res-bootstrap-redis/configuration.yaml

	# Handling Dockerfiles
	cp $(EDGEX_SUBMODULE_PATH)/Dockerfiles/Dockerfile_security_bootstrapper_mod $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/security-bootstrapper/Dockerfile
	cp $(EDGEX_SUBMODULE_PATH)/Dockerfiles/Dockerfile_security_secretstore_setup_mod $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/security-secretstore-setup/Dockerfile
	cp $(EDGEX_SUBMODULE_PATH)/Dockerfiles/Dockerfile_security_proxy_setup_mod $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/security-proxy-setup/Dockerfile
	cp $(EDGEX_SUBMODULE_PATH)/Dockerfiles/Dockerfile_core_metadata_mod $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/core-metadata/Dockerfile
	cp $(EDGEX_SUBMODULE_PATH)/Dockerfiles/Dockerfile_core_common_config_bootstrapper_mod $(EDGEX_SUBMODULE_PATH)/edgex-go/cmd/core-common-config-bootstrapper/Dockerfile

	# Handling go.mod
	cp $(EDGEX_SUBMODULE_PATH)/go.mod_mod $(EDGEX_SUBMODULE_PATH)/edgex-go/go.mod
	rm -f $(EDGEX_SUBMODULE_PATH)/edgex-go/go.sum
	cd $(EDGEX_SUBMODULE_PATH)/edgex-go && \
	make -j 2 -e DOCKER_TAG=$(EDGEX_VERSION) -e HEDGE_VERSION=$(DOCKER_TAG) -e GO_BASE=$(GO_BASE) -e ALPINE_BASE=$(ALPINE_BASE) -e NODE_BASE=$(NODE_BASE) \
		docker_security_bootstrapper \
		docker_security_secretstore_setup \
		docker_security_proxy_setup \
		docker_core_metadata \
		docker_core_common_config

	# Restore original files
	cd $(EDGEX_SUBMODULE_PATH)/edgex-go && git reset --hard

PUSH_EDGEX_IMAGES= \
	edgexfoundry/security-bootstrapper \
	edgexfoundry/security-secretstore-setup \
	edgexfoundry/security-proxy-setup \
	edgexfoundry/core-metadata \
	edgexfoundry/core-common-config-bootstrapper

# This is to pull edgex and other external images from docker.io and push into harbor
push-edgex-images: ${PUSH_EDGEX_IMAGES}
$(PUSH_EDGEX_IMAGES):
	@$(eval ORIG_EDGEX_IMG := $@:${EDGEX_VERSION})
	@$(eval NEW_EDGEX_IMGNAME := $(shell echo $@ | sed -e "s/edgexfoundry\//hedge-ext-/g"))
	@$(eval TMP_DOCKERFILE := /tmp/Dockerfile.$(shell basename ${NEW_EDGEX_IMGNAME}))

	@echo "# Creating a temporary Dockerfile to set USER 2002"
	@echo "FROM ${ORIG_EDGEX_IMG}" > ${TMP_DOCKERFILE}
	@echo "USER 2002" >> ${TMP_DOCKERFILE}

	@echo "Building the new image with USER 2002 set"
	@docker build -t $(DOCKER_REGISTRY)/${NEW_EDGEX_IMGNAME}:${EDGEX_VERSION} -f ${TMP_DOCKERFILE} .

	@echo "Pushing the new image"
	@docker push $(DOCKER_REGISTRY)/${NEW_EDGEX_IMGNAME}:${EDGEX_VERSION}

	@echo "Tagging and pushing the image with the new Docker tag"
	@docker tag $(DOCKER_REGISTRY)/${NEW_EDGEX_IMGNAME}:${EDGEX_VERSION} $(DOCKER_REGISTRY)/${NEW_EDGEX_IMGNAME}:${DOCKER_TAG}
	@docker push $(DOCKER_REGISTRY)/${NEW_EDGEX_IMGNAME}:${DOCKER_TAG}

	@echo "Cleaning up the temporary Dockerfile"
	@rm ${TMP_DOCKERFILE}

# --- direct utility docker build & push sub-targets --

# make target to build docker images for all hedge services
build-hedge-services: hedge-services-build-group
push-hedge-services: push-app-services push-device-services push-ml-models push-ml-services

# make target to build docker images for all hedge services as well as edgex and external images

build-all: hedge-services-build-group \
           build-edgex-services build-external-services

push-all: push-app-services push-device-services push-external-services \
          push-ml-python-services push-ci-python-base push-ml-services \
          push-init-services

hedge-services-build-group: build-app-services build-device-services \
                build-init-services build-ml-services \
                build-ml-python-services

build-app-services: $(addprefix build-,$(APP_SERVICE_IDS))
push-app-services: $(addprefix push-,$(APP_SERVICE_IDS))

build-device-services: $(addprefix build-,$(DEVICE_SERVICE_IDS))
push-device-services: $(addprefix push-,$(DEVICE_SERVICE_IDS))

build-external-services: $(addprefix build-,$(EXTERNAL_SERVICE_IDS))
push-external-services: $(addprefix push-,$(EXTERNAL_SERVICE_IDS))

build-ml-python-services: $(addprefix build-,$(_ml_python_service_ids))
push-ml-python-services: $(addprefix push-,$(_ml_python_service_ids))

build-ml-services: $(addprefix build-,$(ML_MGMT_SERVICE_IDS))
push-ml-services: $(addprefix push-,$(ML_MGMT_SERVICE_IDS))

build-init-services: $(addprefix build-,$(INIT_SERVICE_IDS))
push-init-services: $(addprefix push-,$(INIT_SERVICE_IDS))

build-edgex: $(addprefix build-,edgex-services)


# --- Main Targets ---
ALL_GROUPS = app-services device-services external-services ml-python-services ml-services init-services edgex-services

ifeq ($(strip $(SERVICE)),)
    _MAIN_BUILD_TARGET := build-all
    _MAIN_PUSH_TARGET := push-all
else
    ifeq ($(filter $(SERVICE),$(ALL_GROUPS)),$(SERVICE))
        _MAIN_BUILD_TARGET := build-$(SERVICE)
        _MAIN_PUSH_TARGET := push-$(SERVICE)
    else
        _MAIN_BUILD_TARGET := build-$(SERVICE)
        _MAIN_PUSH_TARGET := push-$(SERVICE)
    endif
endif

.PHONY: docker build push all push-all clean
docker: $(_MAIN_BUILD_TARGET)
	@echo "Build completed for $(or $(SERVICE),all services)"

push: $(_MAIN_PUSH_TARGET)
	@echo "Push completed for $(or $(SERVICE),all services)"

#Make local build
GO := GO111MODULE=on go

# Standard services that follow the pattern
STD_SERVICES := \
	app-services/hedge-data-enrichment/hedge-data-enrichment \
	app-services/hedge-admin/cmd/hedge-admin \
	app-services/hedge-device-extensions/cmd/hedge-device-extensions \
	app-services/hedge-event/hedge-event \
	app-services/hedge-event-publisher/hedge-event-publisher \
	app-services/hedge-export/cmd/hedge-export \
	app-services/hedge-remediate/hedge-remediate \
	app-services/hedge-user-app-mgmt/hedge-user-app-mgmt \
	app-services/hedge-metadata-notifier/cmd/hedge-metadata-notifier \
	app-services/hedge-meta-sync/hedge-meta-sync \
	app-services/hedge-nats-proxy/hedge-nats-proxy \
	edge-ml-service/cmd/hedge-ml-management/hedge-ml-management \
	edge-ml-service/cmd/hedge-ml-broker/hedge-ml-broker \
	device-services/hedge-device-virtual/cmd/hedge-device-virtual

# Services that need special source paths
SPECIAL_SERVICES := \
	hedge-swagger-ui/hedge-swagger-ui \
	edge-ml-service/cmd/hedge-digital-twin \
	edge-ml-service/cmd/ml-sandbox/hedge-ml-sandbox

ALL_SERVICES := $(STD_SERVICES) $(SPECIAL_SERVICES)

.PHONY: build clean $(ALL_SERVICES)

build: $(ALL_SERVICES)

# Pattern rule for standard services
$(STD_SERVICES): %:
	$(GO) build $(GOFLAGS) -o $@ ./$(@D)

# Special services with custom source paths
hedge-swagger-ui/hedge-swagger-ui:
	$(GO) build $(GOFLAGS) -o $@ ./hedge-swagger-ui

edge-ml-service/cmd/hedge-digital-twin:
	$(GO) build $(GOFLAGS) -o $@ ./edge-ml-service/cmd/hedge-digital-twin

edge-ml-service/cmd/ml-sandbox/hedge-ml-sandbox:
	$(GO) build $(GOFLAGS) -o $@ ./edge-ml-service/cmd/hedge-ml-sandbox

clean:
	rm -f $(STD_SERVICES)
	rm -f hedge-swagger-ui/hedge-swagger-ui
	rm -f edge-ml-service/cmd/hedge-digital-twin/hedge-digital-twin
	rm -f edge-ml-service/cmd/ml-sandbox/hedge-ml-sandbox/hedge-ml-sandbox

#package target
.PHONY: package
package:
	rm -rf package/_contents package/*.tgz && echo "package/_contents cleaned up"
	mkdir -p package/_contents

	#Hedge NODE package (dcompose) - hedge-node.tgz
	mkdir package/_contents/hedge-node-docker
	cp -rf hedge-deployment/docker/.env \
		hedge-deployment/docker/docker-compose-common.yml \
		hedge-deployment/docker/docker-compose-node.yml \
		hedge-deployment/docker/Makefile \
		hedge-deployment/docker/scripts \
		hedge-deployment/docker/README.md \
		hedge-deployment/docker/hedge-certs \
		VERSION ./LICENSE \
		package/_contents/hedge-node-docker/
	sed $(SED_OPTIONS) -e '/es-cred-env/d' package/_contents/hedge-node-docker/Makefile
	rm -f package/_contents/hedge-node-docker/scripts/platcrd.sh
	[ ! -f package/hedge-node-docker.tgz ] && tar -czf package/hedge-node-docker.tgz -Cpackage/_contents hedge-node-docker && echo "Created package/hedge-node-docker.tgz" || echo "package/hedge-node-docker.tgz exists"

	#Hedge CORE package (dcompose) - hedge-core-docker.tgz
	mkdir package/_contents/hedge-core-docker
	cp -rf hedge-deployment/docker/.env \
		hedge-deployment/docker/.env-core \
		hedge-deployment/docker/docker-compose-common.yml \
		hedge-deployment/docker/docker-compose-core.yml \
		hedge-deployment/docker/docker-compose-node.yml \
		hedge-deployment/docker/Makefile \
		hedge-deployment/docker/scripts \
		hedge-deployment/docker/README.md \
		VERSION ./LICENSE \
		package/_contents/hedge-core-docker/
	[ ! -f package/hedge-core-docker.tgz ] && tar -czf package/hedge-core-docker.tgz -Cpackage/_contents hedge-core-docker && echo "Created package/hedge-core-docker.tgz" || echo "package/hedge-core-docker.tgz exists"

	rm -rf package/_contents


# --- Help Documentation ---
.PHONY: help
help:
	@echo "Hedge Service Management System"
	@echo "==============================="
	@echo ""
	@echo "User configurable options for make docker :"
	@echo "  FORCE_DOCKER_BUILD=true/false    - Force build regardless of changes (default: false)"
	@echo "  IMGAGE_PUSH=true/false    - whether the new image that is built has to be pushed to docker registry (default: false)"
	@echo ""
	@echo "Docker build usage examples:"
	@echo "  # Normal docker build (only changed services)"
	@echo "  make docker IMAGE_PUSH=false"
	@echo ""
	@echo "  # Build specific service"
	@echo "  make docker SERVICE=hedge-admin"
	@echo "" 
	@echo "  # Force rebuild specific service"
	@echo "  make docker SERVICE=hedge-admin FORCE_DOCKER_BUILD=true"
	@echo ""
	@echo "  # Build individual local service "
	@echo "  make app-services/hedge-admin/cmd/hedge-admin"
	@echo ""
	@echo "Main Targets: docker push package build"
	@echo "  docker                   - Build all service groups"
	@echo "  docker [SERVICE=<name>]  - Build specific service or group"
	@echo "  docker [SERVICE=<name>]  - Build specific service or group"
	@echo "  push [SERVICE=<name>]    - Push specific service or group"
	@echo "  push-all                 - Push all service groups"
	@echo "  package                  - Create docker-compose package that can be distributed for installations"
	@echo "  build                    - Build all local services"
	@echo "  clean                    - Remove all local built binaries"
	@echo ""
	@echo "Service Groups Targets:"
	@echo "  hedge-services           - Build all hedge services"
	@echo "  app-services       - Core application services"
	@echo "  device-services    - Device management services"
	@echo "  external-services  - Build External images"
	@echo "  ml-python-services  - Machine learning models"
	@echo "  hedge-ml-python-base   - ML Python base image"
	@echo "  ml-services         - ML services"
	@echo "  init-services      - hedge initialization services"
	@echo "  edgex-services     - EdgeX core services"
