.PHONY: build tidy package clean clean-docker docker codecoverage push-myimage push push-edgex-images push-ext-images push-opt-images push-ml-python-base push-python-test-coverage-base build-python-test-coverage-base generate-swagger-spec generate-control-swagger-spec
#.SILENT: #disabling by default since it becomes difficult to search/debug in Jenkins logs

##############################################
### ALL PRODUCT REGISTRY & VERSIONS IN USE ###
##############################################

# BMC Internal Registries
DOCKER_REGISTRY=pun-harbor-reg1.bmc.com
RSSO_REGISTRY=pun-harbor-reg1.bmc.com/rsso
UX_REGISTRY=cdn.bmc.com:4873/repository/bmc-ux/
SECURITY_REGISTRY=psg-hrbr-aus.bmc.com
SECURITY_TPS_REGISTRY=tps-jobs.psg.bmc.com:8443
##
REGISTRY ?= ${DOCKER_REGISTRY}/iot/
SECURITY_PSG_REGISTRY ?= ${SECURITY_REGISTRY}/psg/
SECURITY_IOT_REGISTRY ?= ${SECURITY_REGISTRY}/iot/

# BMC Internal Image Versions
#AUTH_PROXY_VER=HSSO_Auth_Proxy_RC_225
#RSSO_VER=HSSO_25.1.02-RC
ADE_LIBCHART_VERSION=3.0.17

# EDGEX Version
EDGEX_VERSION=3.1.0

# EXTERNAL App Versions
ANGULAR_VERSION=18.2.6
NATS_VERSION=2.10.26-alpine3.21
CONSUL_VERSION=1.20.2
VAULT_VERSION=1.18.5
REDIS_VER=7.2.5-r2
MOSQUITTO_VERSION=2.0.21
NGINX_VER=1.27.3-alpine
GRAFANA_VERSION=11.6.1
NODERED_VERSION=4.0.7-22-minimal
KUIPER_VERSION=2.0.7-alpine
ELASTIC_SEARCH_VERSION=3
VICTORIA_METRICS_VERSION=v1.115.0
POSTGRES_VER=17.4-alpine3.21

# Set another ENV value instead of default 'jenkins' - for creating images suitable on Mac M1 Silicon (will pull images from the default source - Docker Hub):
ENV ?= jenkins
# Jenkins Environment Variables (linux/amd64)
ifeq ($(ENV), jenkins)
	#GO_BASE := ${SECURITY_PSG_REGISTRY}golang:latest-1.23-alpine
	GO_BASE := golang:1.24.2-alpine3.21
	#NODE_BASE := ${SECURITY_PSG_REGISTRY}node:latest-20
	NODE_BASE := node:23.11.0-alpine3.21
	#ALPINE_BASE := ${SECURITY_PSG_REGISTRY}alpine:latest-base
	ALPINE_BASE := alpine:3.21.3
	DOCKER_BASE := docker:dind-rootless
	DOCKER_BUILD_FLAGS := --platform linux/amd64
else
# Local Environment Variables (MacOS M1 Silicon, linux/arm64)
	GO_BASE := golang:1.24.2-alpine3.21
	NODE_BASE := node:23.11.0-alpine3.21
	ALPINE_BASE := alpine:3.21.3
	DOCKER_BASE := docker:dind-rootless
	DOCKER_BUILD_FLAGS := --platform linux/arm64
endif

###################################

UNAME_S := $(shell uname)

ifeq ($(UNAME_S), Darwin)
	SED_OPTIONS = -i ''
else
	SED_OPTIONS = -i
endif

# Not used right now, still keeping it for later updates when we fix K8 deployments
helm-init:
	cp -f hedge-deployment/k8s/helm/hedge/Chart-template.yaml hedge-deployment/k8s/helm/hedge/Chart.yaml
	
	# Apply sed replacements
	sed $(SED_OPTIONS) -e "s/__LIB_VER__/$(ADE_LIBCHART_VERSION)/g" hedge-deployment/k8s/helm/hedge/Chart.yaml
	sed $(SED_OPTIONS) -e "s/__VERSION__/1221/g" hedge-deployment/k8s/helm/hedge/Chart.yaml

	# Update version in VERSION.helmchart file
	echo $(ADE_LIBCHART_VERSION) > hedge-deployment/k8s/VERSION.helmchart

	# Helm operations
	helm dependency update hedge-deployment/k8s/helm/hedge/.
	mv hedge-deployment/k8s/helm/hedge/charts/*.tgz hedge-deployment/k8s/helm/hedge/.
	rm -rf hedge-deployment/k8s/helm/hedge/Chart.lock
	rm -rf hedge-deployment/k8s/helm/hedge/charts


package:
	rm -rf package/_contents package/*.tgz && echo "package/_contents cleaned up"
	mkdir package/_contents

	#Hedge NODE package (dcompose) - hedge-node.tgz
	mkdir package/_contents/hedge-node-docker
	cp -rf hedge-deployment/docker/.env \
		hedge-deployment/docker/docker-compose-common.yml \
		hedge-deployment/docker/docker-compose-node.yml \
		hedge-deployment/docker/Makefile \
		hedge-deployment/docker/scripts \
		hedge-deployment/docker/README.md \
		hedge-deployment/docker/hedge-certs \
		VERSION LICENSE \
		package/_contents/hedge-node-docker/
	sed $(SED_OPTIONS) -e '/es-cred-env/d' package/_contents/hedge-node-docker/Makefile
	rm -f package/_contents/hedge-node-docker/scripts/platcrd.sh
	[ ! -f package/hedge-node-docker.tgz ] && tar -czf package/hedge-node-docker.tgz -Cpackage/_contents hedge-node-docker && echo "Created package/hedge-node-docker.tgz" || echo "package/hedge-node-docker.tgz exists"

	#Hedge CORE package (dcompose) - hedge-core-docker.tgz
	mkdir package/_contents/hedge-core-docker
	cp -rf hedge-deployment/docker/.env \
		hedge-deployment/docker/.env-core \
		hedge-deployment/docker/.es-cred-env \
		hedge-deployment/docker/docker-compose-common.yml \
		hedge-deployment/docker/docker-compose-core.yml \
		hedge-deployment/docker/docker-compose-node.yml \
		hedge-deployment/docker/Makefile \
		hedge-deployment/docker/scripts \
		hedge-deployment/docker/README.md \
		VERSION LICENSE \
		package/_contents/hedge-core-docker/
	#mkdir package/_contents/hedge-core-docker/scripts/ade
	#cp -f hedge-deployment/ade_scripts/rsso/external.conf hedge-deployment/ade_scripts/rsso/hedge-reverse-proxy.conf package/_contents/hedge-core-dev/scripts/ade/
	[ ! -f package/hedge-core-docker.tgz ] && tar -czf package/hedge-core-docker.tgz -Cpackage/_contents hedge-core-docker && echo "Created package/hedge-core-docker.tgz" || echo "package/hedge-core-dev.tgz exists"

	#Hedge ADE package
	#mkdir -p package/_contents/hedge-ade/dashboard
	#cp -rf VERSION LICENSE package/_contents/hedge-ade/
	#rsync -av --exclude='*-DEV*' --exclude='*.conf' --exclude='out/' --exclude='_archive/'
	#cp -rf external/grafana/contents/resources/dashboards/HelixIoT/edge_devices.json	package/_contents/hedge-ade/dashboard
	#cp -rf external/grafana/contents/resources/dashboards/HelixIoT/edge_events.json	package/_contents/hedge-ade/dashboard
	#cp -rf external/grafana/contents/resources/dashboards/Metering/Helix\ Edge\ License\ Metering-ADE.json package/_contents/hedge-ade/dashboard
	#[ ! -f package/hedge-ade.tgz ] && tar -czf package/hedge-ade.tgz -Cpackage/_contents hedge-ade && echo "Created package/hedge-ade.tgz" || echo "package/hedge-ade.tgz exists"

	rm -rf package/_contents

###########################

GO=GO111MODULE=on go

BMC_MICROSERVICES= \
	app-services/data-enrichment/data-enrichment \
	app-services/hedge-admin/cmd/hedge-admin \
	app-services/hedge-device-extensions/cmd/hedge-device-extensions \
	app-services/hedge-event/hedge-event \
	app-services/hedge-event-publisher/hedge-event-publisher \
	app-services/hedge-export/cmd/hedge-export \
	app-services/hedge-remediate/hedge-remediate \
	app-services/user-app-mgmt/hedge-user-app-mgmt \
	app-services/export-biz-data/export-biz-data \
	app-services/metadata-notifier/cmd/metadata-notifier \
	app-services/meta-sync/meta-sync \
	app-services/nats-proxy/nats-proxy \
	edge-ml-service/cmd/ml-management/ml-management \
	edge-ml-service/cmd/ml-broker/hedge-ml-broker \
	ui/edge-portal/hedge-ui-server \
	device-services/device-virtual/cmd/device-virtual \
	swagger-ui/swagger-ui

.PHONY: $(BMC_MICROSERVICES)

build: $(BMC_MICROSERVICES)

DOCKER_IMG_VERSION=$(shell cat ./VERSION 2>/dev/null || echo latest)
HEDGE_RELEASE_VERSION=$(shell echo $(DOCKER_IMG_VERSION) | sed 's/_.*//')
DOCKER_TAG ?= $(DOCKER_IMG_VERSION)

GOFLAGS=-ldflags "-X hedge.Version=$(DOCKER_IMG_VERSION)"
GOTESTFLAGS?=-race
ARCH=$(shell uname -m)
SAMBA_SERVER=clm-aus-w71olm

get_version:
	@echo "\tHedge Release Version: \t${HEDGE_RELEASE_VERSION}"
	@echo "\tHedge Image Tag: \t${DOCKER_IMG_VERSION}"

app-services/data-enrichment/data-enrichment:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/data-enrichment

app-services/hedge-admin/cmd/hedge-admin:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/hedge-admin/cmd

app-services/hedge-device-extensions/cmd/hedge-device-extensions:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/hedge-device-extensions/cmd

edge-ml-service/cmd/digital-twin:
	$(GO) build $(GOFLAGS) -o $@ ./edge-ml-service/cmd/digital-twin

app-services/hedge-event/hedge-event:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/hedge-event

app-services/hedge-event-publisher/hedge-event-publisher:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/hedge-event-publisher

app-services/hedge-export/cmd/hedge-export:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/hedge-export/cmd

app-services/hedge-remediate/hedge-remediate:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/hedge-remediate

app-services/user-app-mgmt/hedge-user-app-mgmt:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/user-app-mgmt

app-services/export-biz-data/export-biz-data:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/export-biz-data

app-services/meta-sync/meta-sync:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/meta-sync

app-services/metadata-notifier/cmd/metadata-notifier:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/metadata-notifier/cmd

app-services/nats-proxy/nats-proxy:
	$(GO) build $(GOFLAGS) -o $@ ./app-services/nats-proxy

edge-ml-service/cmd/ml-management/ml-management:
	$(GO) build $(GOFLAGS) -o $@ ./edge-ml-service/cmd/ml-management

edge-ml-service/cmd/ml-sandbox/ml-sandbox:
	$(GO) build $(GOFLAGS) -o $@ ./edge-ml-service/cmd/ml-sandbox

edge-ml-service/cmd/ml-broker/hedge-ml-broker:
	$(GO) build $(GOFLAGS) -o $@ ./edge-ml-service/cmd/ml-broker

ui/edge-portal/hedge-ui-server:
	$(GO) build $(GOFLAGS) -o $@ ./ui/edge-portal

device-services/device-virtual/cmd/device-virtual:
	$(GO) build $(GOFLAGS) -o $@ ./device-services/device-virtual/cmd

swagger-ui/swagger-ui:
	$(GO) build $(GOFLAGS) -o $@ ./swagger-ui/

tidy:
	go mod tidy

clean:
	rm -f $(BMC_MICROSERVICES)
	rm -rf package/_contents package/*.tgz
	rm -f hedge-deployment/k8s/helm/hedge/librarycharts-*.tgz
	rm -rf hedge-deployment/k8s/helm/hedge/*/charts
	rm -f hedge-deployment/k8s/helm/hedge/*/Chart.yaml\'\'
	find hedge-deployment/k8s/helm/hedge -name "Chart.yaml*" | grep -v "hedge_storage" | grep -v "_archive" | xargs rm -f

###########################

clean-docker:
	rm -f $(DOCKER_ENRICH) $(DOCKER_ADMN) $(DOCKER_DEXT) $(DOCKER_TWIN) $(DOCKER_EVE) $(DOCKER_EPUB) $(DOCKER_EXP) $(DOCKER_RMDT) $(DOCKER_USRMGT) \
		$(DOCKER_EXPBIZ) $(DOCKER_METNOTIFIER) $(DOCKER_METSYNC) $(DOCKER_NATSPRXY) $(DOCKER_MLMGMT) $(DOCKER_MLBRKR) $(DOCKER_MLAGNT) $(DOCKER_MLSANDBOX) $(DOCKER_MLPYBASE) $(DOCKER_MLAUTO) \
		$(DOCKER_MLINF) $(DOCKER_MLTRGCLASS) $(DOCKER_MLPREDCLASS) $(DOCKER_MLTRGTIMES) $(DOCKER_MLPREDTIMES) $(DOCKER_MLTRGREGR) $(DOCKER_MLPREDREGR) $(DOCKER_INIT) \
		$(DOCKER_NGINX) $(DOCKER_GRAFANA) hedge_grafana_contents $(DOCKER_NRED) $(DOCKER_KUIPR) $(DOCKER_DSVIRT) $(DOCKER_SWAGGER_UI)

# This list will be used during 'make docker' to build the docker images
DOCKER_ENRICH=hedge_data_enrichment
DOCKER_ADMN=hedge_admin
DOCKER_DEXT=hedge_device_extensions
DOCKER_TWIN=hedge_digital_twin
DOCKER_EVE=hedge_event
DOCKER_EPUB=hedge_event_publisher
DOCKER_EXP=hedge_export
DOCKER_RMDT=hedge_remediate
DOCKER_USRMGT=hedge_user_app_mgmt
DOCKER_EXPBIZ=hedge_export_biz_data
DOCKER_METSYNC=hedge_meta_sync
DOCKER_METNOTIFIER=hedge_metadata_notifier
DOCKER_NATSPRXY=hedge_nats_proxy
DOCKER_MLMGMT=hedge_ml_management
DOCKER_MLSANDBOX=hedge_ml_sandbox
DOCKER_MLBRKR=hedge_ml_broker
DOCKER_MLAGNT=hedge_ml_edge_agent
DOCKER_MLPYBASE=hedge_ml_python_base
DOCKER_MLAUTO=hedge_ml_trg_anomaly_autoencoder
DOCKER_MLINF=hedge_ml_pred_anomaly_autoencoder
DOCKER_MLTRGCLASS=hedge_ml_trg_classification_randomforest
DOCKER_MLPREDCLASS=hedge_ml_pred_classification_randomforest
DOCKER_MLTRGTIMES=hedge_ml_trg_timeseries_multivariate_deepvar
DOCKER_MLPREDTIMES=hedge_ml_pred_timeseries_multivariate_deepvar
DOCKER_MLTRGREGR=hedge_ml_trg_regression_lightgbm
DOCKER_MLPREDREGR=hedge_ml_pred_regression_lightgbm
DOCKER_INIT=hedge_init
DOCKER_NGINX=hedge_nginx
DOCKER_UI=hedge_ui_server
DOCKER_GRAFANA=hedge_grafana
DOCKER_NRED=hedge_node_red
DOCKER_KUIPR=hedge_kuiper
DOCKER_DSVIRT=device_virtual
DOCKER_EDGEX=edgex_docker_images
DOCKER_SWAGGER_UI=hedge_swagger_ui

# Docker may not rebuild an image because all layers are cached, so push will be skipped
push-ml-python-base: $(DOCKER_MLPYBASE)
	docker tag hedge-ml-python-base:$(DOCKER_TAG) $(REGISTRY)hedge-ml-python-base:$(DOCKER_TAG)
	docker push $(REGISTRY)hedge-ml-python-base:$(DOCKER_TAG)
	# Attempt to tag the image; if tagging fail (reason - the build was skipped because a cached version of the image, using the same Docker layers, already exists), skip push and exit gracefully
#	@if docker tag hedge-ml-python-base:$(DOCKER_TAG) $(PYTHON_BASE); then \
#		echo "Image tagged successfully."; \
#		docker push $(PYTHON_BASE); \
#	else \
#		echo "Tagging skipped because the image wasn't rebuilt (a cached version of the image, using the same Docker layers, already exists), skipping push as well."; \
#		exit 0; \
#	fi

build-python-test-coverage-base:
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-python-test-coverage-base" \
		-f python_test_coverage_base.Dockerfile \
		-t hedge-python-test-coverage-base:$(GIT_SHA) \
		-t hedge-python-test-coverage-base:$(DOCKER_TAG) \
		.
		touch $@

push-python-test-coverage-base: build-python-test-coverage-base
	# Attempt to tag the image; if tagging fail (reason - the build was skipped because a cached version of the image, using the same Docker layers, already exists), skip push and exit gracefully
	docker tag hedge-python-test-coverage-base:$(DOCKER_TAG) $(PYTHON_TEST_COVERAGE_BASE)
	docker push $(PYTHON_TEST_COVERAGE_BASE)

docker: $(DOCKER_EDGEX) $(DOCKER_ENRICH) $(DOCKER_ADMN) $(DOCKER_DEXT) $(DOCKER_TWIN) $(DOCKER_EVE) $(DOCKER_EPUB) $(DOCKER_EXP) $(DOCKER_RMDT) $(DOCKER_USRMGT) \
		$(DOCKER_EXPBIZ) $(DOCKER_METNOTIFIER) $(DOCKER_METSYNC) $(DOCKER_NATSPRXY) $(DOCKER_MLMGMT) $(DOCKER_MLBRKR) $(DOCKER_MLAGNT) $(DOCKER_MLSANDBOX) $(DOCKER_MLAUTO) \
		$(DOCKER_MLINF) $(DOCKER_MLTRGCLASS) $(DOCKER_MLPREDCLASS) $(DOCKER_MLTRGTIMES) $(DOCKER_MLPREDTIMES) $(DOCKER_MLTRGREGR) $(DOCKER_MLPREDREGR) $(DOCKER_INIT) \
		$(DOCKER_NGINX) $(DOCKER_GRAFANA) hedge_grafana_contents $(DOCKER_NRED) $(DOCKER_KUIPR) $(DOCKER_DSVIRT) $(DOCKER_SWAGGER_UI)

GIT_SHA=$(shell git rev-parse HEAD)
DOCKER_VERSION=$(shell docker version --format '{{.Server.Version}}')
BUILDX_VERSION=$(shell docker buildx version)
TODAY=$(shell date -I)
HOSTNAME=$(shell hostname)

SRC_COMMON := $(shell find common -type f | grep -v ' ')
SRC_COMMON_PYTHON := $(shell find edge-ml-service/python-code/common -type f | grep -v ' ')

DOCKER_LABELS = \
				--label "git_sha=$(GIT_SHA)" \
				--label "copyright=(c) as per Apache 2.0." \
				--label "contributors=BMC Software, Inc." \
				--label "hedge-version=$(DOCKER_TAG)" \
				--label "hedge-base-docker-version=$(DOCKER_VERSION)" \
				--label "hedge-base-buildx-version=$(BUILDX_VERSION)" \
				--label "hedge-base-build-date=$(TODAY)" \
				--label "hedge-base-build-server=$(HOSTNAME)" \
				--label "license='Apache 2.0'"

DOCKER_BUILDARGS = \
				--build-arg http_proxy \
				--build-arg https_proxy \
				--build-arg GO_BASE=$(GO_BASE) \
				--build-arg NODE_BASE=$(NODE_BASE) \
				--build-arg ALPINE_BASE=$(ALPINE_BASE) \
				--build-arg PYTHON_BASE=$(PYTHON_BASE) \
				$(DOCKER_BUILD_FLAGS)

#docker build commands start
SRC_ENRICH := $(shell find app-services/data-enrichment -type f | grep -v ' ')
hedge_data_enrichment: $(SRC_ENRICH) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-data-enrichment" \
		-f app-services/data-enrichment/Dockerfile \
		-t hedge-data-enrichment:$(GIT_SHA) \
		-t hedge-data-enrichment:$(DOCKER_TAG) \
		.
		touch $@

SRC_ADMIN := $(shell find app-services/hedge-admin -type f | grep -v ' ')
hedge_admin: $(SRC_ADMIN) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-admin" \
		-f app-services/hedge-admin/Dockerfile \
		-t hedge-admin:$(GIT_SHA) \
		-t hedge-admin:$(DOCKER_TAG) \
		.
		touch $@

SRC_DEVICE_EXTN := $(shell find app-services/hedge-device-extensions -type f | grep -v ' ')
hedge_device_extensions: $(SRC_DEVICE_EXTN) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-device-extensions" \
		-f app-services/hedge-device-extensions/Dockerfile \
		-t hedge-device-extensions:$(GIT_SHA) \
		-t hedge-device-extensions:$(DOCKER_TAG) \
		.
		touch $@

SRC_DIGITAL_TWIN := $(shell find ./edge-ml-service/cmd/digital-twin -type f | grep -v ' ')
hedge_digital_twin: $(SRC_DIGITAL_TWIN) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-digital-twin" \
		-f edge-ml-service/cmd/digital-twin/Dockerfile \
		-t hedge-digital-twin:$(GIT_SHA) \
		-t hedge-digital-twin:$(DOCKER_TAG) \
		.
		touch $@

SRC_EVENT := $(shell find app-services/hedge-event -type f | grep -v ' ')
hedge_event: $(SRC_EVENT) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-event" \
		-f app-services/hedge-event/Dockerfile \
		-t hedge-event:$(GIT_SHA) \
		-t hedge-event:$(DOCKER_TAG) \
		.
		touch $@

SRC_EVENT_PUBLISHER := $(shell find app-services/hedge-event-publisher -type f | grep -v ' ')
hedge_event_publisher: $(SRC_EVENT_PUBLISHER) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-event-publisher" \
		-f app-services/hedge-event-publisher/Dockerfile \
		-t hedge-event-publisher:$(GIT_SHA) \
		-t hedge-event-publisher:$(DOCKER_TAG) \
		.
		touch $@

SRC_EXPORT := $(shell find app-services/hedge-export -type f | grep -v ' ')
hedge_export: $(SRC_EXPORT) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-export" \
		-f app-services/hedge-export/Dockerfile \
		-t hedge-export:$(GIT_SHA) \
		-t hedge-export:$(DOCKER_TAG) \
		.
		touch $@

SRC_REMEDIATE := $(shell find app-services/hedge-remediate -type f | grep -v ' ')
hedge_remediate: $(SRC_REMEDIATE) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-remediate" \
		-f app-services/hedge-remediate/Dockerfile \
		-t hedge-remediate:$(GIT_SHA) \
		-t hedge-remediate:$(DOCKER_TAG) \
		.
		touch $@


SRC_USR_MGMT := $(shell find app-services/user-app-mgmt -type f | grep -v ' ')
hedge_user_app_mgmt: $(SRC_USR_MGMT) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-user-app-mgmt" \
		-f app-services/user-app-mgmt/Dockerfile \
		-t hedge-user-app-mgmt:$(GIT_SHA) \
		-t hedge-user-app-mgmt:$(DOCKER_TAG) \
		.
		touch $@

SRC_EXPORT_BIZ := $(shell find app-services/export-biz-data -type f | grep -v ' ')
hedge_export_biz_data: $(SRC_EXPORT_BIZ) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-export-biz-data" \
		-f app-services/export-biz-data/Dockerfile \
		-t hedge-export-biz-data:$(GIT_SHA) \
		-t hedge-export-biz-data:$(DOCKER_TAG) \
		.
		touch $@

SRC_META_SYNC := $(shell find app-services/meta-sync -type f | grep -v ' ')
hedge_meta_sync: $(SRC_META_SYNC) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-meta-sync" \
		-f app-services/meta-sync/Dockerfile \
		-t hedge-meta-sync:$(GIT_SHA) \
		-t hedge-meta-sync:$(DOCKER_TAG) \
		.
		touch $@

SRC_META_NOTIFIER := $(shell find app-services/metadata-notifier -type f | grep -v ' ')
hedge_metadata_notifier: $(SRC_META_NOTIFIER)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-metadata-notifier" \
		-f app-services/metadata-notifier/Dockerfile \
		-t hedge-metadata-notifier:$(GIT_SHA) \
		-t hedge-metadata-notifier:$(DOCKER_TAG) \
		.
		touch $@

SRC_NATS_PROXY := $(shell find app-services/nats-proxy -type f | grep -v ' ')
hedge_nats_proxy: $(SRC_NATS_PROXY) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-nats-proxy" \
		-f app-services/nats-proxy/Dockerfile \
		-t hedge-nats-proxy:$(GIT_SHA) \
		-t hedge-nats-proxy:$(DOCKER_TAG) \
		.
		touch $@

SRC_ML := $(shell find ./edge-ml-service -type f | grep -v ' ')
hedge_ml_management: $(SRC_ML) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		---target=builder \
		--label "Name=hedge-ml-management" \
		-f edge-ml-service/cmd/ml-management/Dockerfile \
		-t hedge-ml-management:$(GIT_SHA) \
		-t hedge-ml-management:$(DOCKER_TAG) \
		.
		touch $@

hedge_ml_sandbox: $(SRC_ML) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-sandbox" \
		-f edge-ml-service/cmd/ml-sandbox/Dockerfile \
		-t hedge-ml-sandbox:$(GIT_SHA) \
		-t hedge-ml-sandbox:$(DOCKER_TAG) \
		.
		touch $@

hedge_ml_broker: $(SRC_ML) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-broker" \
		-f edge-ml-service/cmd/ml-broker/Dockerfile \
		-t hedge-ml-broker:$(GIT_SHA) \
		-t hedge-ml-broker:$(DOCKER_TAG) \
		.
		touch $@

hedge_ml_edge_agent: $(SRC_ML) $(SRC_COMMON)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-edge-agent" \
		-f edge-ml-service/cmd/ml-edge-agent/Dockerfile \
		-t hedge-ml-edge-agent:$(GIT_SHA) \
		-t hedge-ml-edge-agent:$(DOCKER_TAG) \
		.
		touch $@

hedge_ml_python_base: edge-ml-service/python-code/base-image/Dockerfile
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-python-base" \
		-f base-image/Dockerfile \
		-t hedge-ml-python-base:$(GIT_SHA) \
		-t hedge-ml-python-base:$(DOCKER_TAG) \
		.
		touch $@
	docker tag hedge-ml-python-base:$(DOCKER_TAG) $(PYTHON_BASE)

hedge_ml_python_base_dev: base-image/Dockerfile.dev
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-python-base" \
		-f base-image/Dockerfile.dev \
		-t hedge-ml-python-base:$(GIT_SHA) \
		-t hedge-ml-python-base:$(DOCKER_TAG) \
		.
		touch $@


SRC_ML_TRG_ANOMALY_AUTOENCODER := $(shell find edge-ml-service/python-code/anomaly/autoencoder/train -type f | grep -v ' ')
hedge_ml_trg_anomaly_autoencoder: hedge_ml_python_base $(SRC_ML_TRG_ANOMALY_AUTOENCODER) $(SRC_COMMON_PYTHON)
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-trg-anomaly-autoencoder" \
		-f anomaly/autoencoder/train/Dockerfile \
		-t hedge-ml-trg-anomaly-autoencoder:$(GIT_SHA) \
		-t hedge-ml-trg-anomaly-autoencoder:$(DOCKER_TAG) \
		.
		touch $@

SRC_ML_PRED_ANOMALY_AUTOENCODER := $(shell find edge-ml-service/python-code/anomaly/autoencoder/infer -type f | grep -v ' ')
hedge_ml_pred_anomaly_autoencoder: hedge_ml_python_base $(SRC_ML_PRED_ANOMALY_AUTOENCODER) $(SRC_COMMON_PYTHON)
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-pred-anomaly-autoencoder" \
		-f anomaly/autoencoder/infer/Dockerfile \
		-t hedge-ml-pred-anomaly-autoencoder:$(GIT_SHA) \
		-t hedge-ml-pred-anomaly-autoencoder:$(DOCKER_TAG) \
		.
		touch $@

SRC_ML_TRG_CLASSIFICATION_RANDOMFOREST := $(shell find edge-ml-service/python-code/classification/random_forest/train -type f | grep -v ' ')
hedge_ml_trg_classification_randomforest: hedge_ml_python_base $(SRC_ML_TRG_CLASSIFICATION_RANDOMFOREST) $(SRC_COMMON_PYTHON)
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-trg-classification-randomforest" \
		-f classification/random_forest/train/Dockerfile \
		-t hedge-ml-trg-classification-randomforest:$(GIT_SHA) \
		-t hedge-ml-trg-classification-randomforest:$(DOCKER_TAG) \
		.
		touch $@

SRC_ML_PRED_CLASSIFICATION_RANDOMFOREST := $(shell find edge-ml-service/python-code/classification/random_forest/infer -type f | grep -v ' ')
hedge_ml_pred_classification_randomforest: hedge_ml_python_base $(SRC_ML_PRED_CLASSIFICATION_RANDOMFOREST) $(SRC_COMMON_PYTHON)
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-pred-classification-randomforest" \
		-f classification/random_forest/infer/Dockerfile \
		-t hedge-ml-pred-classification-randomforest:$(GIT_SHA) \
		-t hedge-ml-pred-classification-randomforest:$(DOCKER_TAG) \
		.
		touch $@

SRC_ML_TRG_TIMESERIES_MULTIVARIATE_DEEPVAR := $(shell find edge-ml-service/python-code/timeseries/multivariate_deepVAR/train -type f | grep -v ' ')
hedge_ml_trg_timeseries_multivariate_deepvar: hedge_ml_python_base $(SRC_ML_TRG_TIMESERIES_MULTIVARIATE_DEEPVAR) $(SRC_COMMON_PYTHON)
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-trg-timeseries-multivariate-deepvar" \
		-f timeseries/multivariate_deepVAR/train/Dockerfile \
		-t hedge-ml-trg-timeseries-multivariate-deepvar:$(GIT_SHA) \
		-t hedge-ml-trg-timeseries-multivariate-deepvar:$(DOCKER_TAG) \
		.
		touch $@

SRC_ML_PRED_TIMESERIES_MULTIVARIATE_DEEPVAR := $(shell find edge-ml-service/python-code/timeseries/multivariate_deepVAR/infer -type f | grep -v ' ')
hedge_ml_pred_timeseries_multivariate_deepvar: hedge_ml_python_base $(SRC_ML_PRED_TIMESERIES_MULTIVARIATE_DEEPVAR) $(SRC_COMMON_PYTHON)
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-pred-timeseries-multivariate-deepvar" \
		-f timeseries/multivariate_deepVAR/infer/Dockerfile \
		-t hedge-ml-pred-timeseries-multivariate-deepvar:$(GIT_SHA) \
		-t hedge-ml-pred-timeseries-multivariate-deepvar:$(DOCKER_TAG) \
		.
		touch $@

SRC_ML_TRG_REGRESSION_LIGHTGBM := $(shell find edge-ml-service/python-code/regression/lightgbm/train -type f | grep -v ' ')
hedge_ml_trg_regression_lightgbm: hedge_ml_python_base $(SRC_ML_TRG_REGRESSION_LIGHTGBM) $(SRC_COMMON_PYTHON)
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-trg-regression-lightgbm" \
		-f regression/lightgbm/train/Dockerfile \
		-t hedge-ml-trg-regression-lightgbm:$(GIT_SHA) \
		-t hedge-ml-trg-regression-lightgbm:$(DOCKER_TAG) \
		.
		touch $@

SRC_ML_PRED_REGRESSION_LIGHTGBM := $(shell find edge-ml-service/python-code/regression/lightgbm/infer -type f | grep -v ' ')
hedge_ml_pred_regression_lightgbm: hedge_ml_python_base $(SRC_ML_PRED_REGRESSION_LIGHTGBM) $(SRC_COMMON_PYTHON)
	cd edge-ml-service/python-code && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-ml-pred-regression-lightgbm" \
		-f regression/lightgbm/infer/Dockerfile \
		-t hedge-ml-pred-regression-lightgbm:$(GIT_SHA) \
		-t hedge-ml-pred-regression-lightgbm:$(DOCKER_TAG) \
		.
		touch $@

SRC_INIT := $(shell find hedge-init -type f | grep -v ' ')
hedge_init: $(SRC_INIT)
	cd hedge-init/ && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-init" \
		-f Dockerfile \
		-t hedge-init:$(GIT_SHA) \
		-t hedge-init:$(DOCKER_TAG) \
		.
		touch $@

SRC_NGINX := $(shell find external/nginx -type f | grep -v ' ')
hedge_nginx: $(SRC_NGINX)
	cd external/nginx/ && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedgext-nginx" \
		--label "NGINX_VERSION=$(NGINX_VER)" \
		--build-arg NGINX_VERSION=$(NGINX_VER) \
		-t hedgext-nginx:$(GIT_SHA) \
		-t hedgext-nginx:$(DOCKER_TAG) \
		.
		touch $@

SRC_UI_PORTAL := $(shell find ui/edge-portal -type f | grep -v ' ')
#hedge_ui_server: $(SRC_UI_PORTAL)
#	docker build \
#		$(DOCKER_LABELS) \
#		$(DOCKER_BUILDARGS) \
#		--label "Name=hedge-ui-server" \
#		--build-arg VERSION=$(HEDGE_RELEASE_VERSION) \
#		--build-arg UX_REGISTRY=$(UX_REGISTRY) \
#		--build-arg ANGULAR_VERSION=$(ANGULAR_VERSION) \
#		-f ui/edge-portal/Dockerfile \
#		-t hedge-ui-server:$(GIT_SHA) \
#		-t hedge-ui-server:$(DOCKER_TAG) \
#		.
#		touch $@

PLATFORM=linux/amd64
SRC_GRAFANA := $(shell find external/grafana -type f | grep -v ' ')
hedge_grafana: $(SRC_GRAFANA) hedge_grafana_contents
	cd external/grafana/ && \
	docker build \
		$(DOCKER_LABELS) \
		--label "Name=hedgext-grafana" \
		--label "GRAFANA_VERSION=$(GRAFANA_VERSION)" \
		--build-arg GRAFANA_VERSION=$(GRAFANA_VERSION) \
		--platform $(PLATFORM) \
		--tag hedgext-grafana:$(GIT_SHA) \
		--tag hedgext-grafana:$(DOCKER_TAG) \
		.
		touch $@

grf_content=./external/grafana/contents
grf=$(grf_content)/custom-plugins/_builds
hedge_grafana_contents: external/grafana/contents
	[ ! -d "$(grf)" ] && mkdir $(grf) || echo "$(grf) exists"
	[ ! -f $(grf)/panodata-map-panel.tgz ] && cd $(grf_content)/custom-plugins && tar -czf _builds/panodata-map-panel.tgz -Cmaps/panodata-map-panel . && echo "Created $(grf)/panodata-map-panel.tgz" || echo "$(grf)/panodata-map-panel.tgz exists"
	[ ! -f $(grf)/anomaly-custom-panel.tgz ] && cd $(grf_content)/custom-plugins && tar -czf _builds/anomaly-custom-panel.tgz -Canomaly-custom-panel . && echo "Created $(grf)/anomaly-custom-panel.tgz" || echo "$(grf)/anomaly-custom-panel.tgz exists"
	[ ! -f $(grf)/ticket-viewer-panel.tgz ] && cd $(grf_content)/custom-plugins && tar -czf _builds/ticket-viewer-panel.tgz -Cticket-viewer-panel . && echo "Created $(grf)/ticket-viewer-panel.tgz" || echo "$(grf)/ticket-viewer-panel.tgz exists"
	[ ! -f $(grf)/ade-panel-record-details.tgz ] && cd $(grf_content)/custom-plugins && sh HEDGE-BUILD.sh && echo "Created $(grf)/ade-panel-record-details.tgz" || echo "$(grf)/ade-panel-record-details.tgz exists"
	[ ! -f $(grf)/track-it.tgz ] && cd $(grf_content)/custom-plugins && tar -czf _builds/track-it.tgz -Ctrack-it . && echo "Created $(grf)/track-it.tgz" || echo "$(grf)/track-it.tgz exists"
	[ ! -f $(grf)/bmc-dtwin-datasource.tgz ] && cd $(grf_content)/custom-plugins && tar -czf _builds/bmc-dtwin-datasource.tgz -Cdatasources/bmc-hedgedt-datasource/dist . && echo "Created $(grf)/bmc-dtwin-datasource.tgz" || echo "$(grf)/bmc-dtwin-datasource.tgz exists"


SRC_NODE_RED := $(shell find external/node-red -type f | grep -v ' ')
hedge_node_red: $(SRC_NODE_RED)
	cd external/node-red/ && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedgext-node-red" \
		--label "NODERED_VERSION=$(NODERED_VERSION)" \
		--build-arg NODERED_VERSION=$(NODERED_VERSION) \
		-f Dockerfile \
		-t hedgext-node-red:$(GIT_SHA) \
		-t hedgext-node-red:$(DOCKER_TAG) \
		.
		touch $@

SRC_KUIPER := $(shell find external/kuiper -type f | grep -v ' ')
hedge_kuiper: $(SRC_KUIPER)
	cd external/kuiper/ && \
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedgext-ekuiper" \
		--label "KUIPER_VERSION=$(KUIPER_VERSION)" \
		--build-arg KUIPER_VERSION=$(KUIPER_VERSION) \
		-f Dockerfile \
		-t hedgext-ekuiper:$(GIT_SHA) \
		-t hedgext-ekuiper:$(DOCKER_TAG) \
		. && \
		docker-squash -t hedgext-ekuiper:$(DOCKER_TAG) hedgext-ekuiper:$(DOCKER_TAG)
		touch $@

install-swag:
	$(GO) get github.com/swaggo/swag/cmd/swag

generate-swagger-spec: install-swag
	$(GO) run github.com/swaggo/swag/cmd/swag init --parseInternal=true --generalInfo=doc.go --pd=true --ot=json --output=./swagger-ui/res/swagger/

generate-control-swagger-spec: install-swag
	$(GO) run github.com/swaggo/swag/cmd/swag init --parseInternal=true --generalInfo=doc.go --pd=true --ot=json --output=./swagger-ui/res/swagger/.tmp/

compare-swagger-spec: generate-control-swagger-spec
	jq -S . ./swagger-ui/res/swagger/swagger.json > ./swagger-ui/res/swagger/.tmp/f1.json
	jq -S . ./swagger-ui/res/swagger/.tmp/swagger.json > ./swagger-ui/res/swagger/.tmp/f2.json

	@if diff ./swagger-ui/res/swagger/.tmp/f1.json ./swagger-ui/res/swagger/.tmp/f2.json >/dev/null; then \
		echo "Swagger specification is up-to-date."; \
	else \
		echo "Discrepancy found between ./swagger-ui/res/swagger/swagger.json in Git and generated swagger specification. Please regenerate swagger using make generate-swagger-spec and execute Git commit and push. "; \
		exit 1; \
	fi

SRC_SWAGGER_UI := $(shell find swagger-ui -type f | grep -v ' ')
hedge_swagger_ui: $(SRC_SWAGGER_UI)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-swagger-ui" \
		-f swagger-ui/Dockerfile \
		-t hedge-swagger-ui:$(GIT_SHA) \
		-t hedge-swagger-ui:$(DOCKER_TAG) \
		.
		touch $@

SRC_DEVICE_VIRTUAL := $(shell find device-services/device-virtual -type f | grep -v ' ')
device_virtual: $(SRC_DEVICE_VIRTUAL)
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=hedge-device-virtual" \
		-f device-services/device-virtual/Dockerfile \
		-t hedge-device-virtual:$(GIT_SHA) \
		-t hedge-device-virtual:$(DOCKER_TAG) \
		.
		touch $@

###########################

# Build necessary Edgex images
#	1. fetch git submodule edgex-go from remote
#	2. copy modified entrypoint.sh
#		- entrypoint.sh to force secretstore regenerate security tokens every hour
#		- entrypoint.sh to setup nginx routing and rules
#	3. build docker images with the edgex version's tag = 3.1.0
#	4. revert files to the original

edgex_docker_images:
	git submodule update --init --recursive --force
	cp external/edgex-go-submodule/Makefile_mod external/edgex-go-submodule/edgex-go/Makefile
	cp external/edgex-go-submodule/common_configuration_hedge.yaml external/edgex-go-submodule/edgex-go/cmd/core-common-config-bootstrapper/res/configuration.yaml
	cp external/edgex-go-submodule/secretstore_entrypoint_hedge.sh external/edgex-go-submodule/edgex-go/cmd/security-secretstore-setup/entrypoint.sh
	cp external/edgex-go-submodule/security_proxy_setup_hedge.sh external/edgex-go-submodule/edgex-go/cmd/security-proxy-setup/entrypoint.sh
	cp external/edgex-go-submodule/security_bootstrapper_entrypoint_hedge.sh external/edgex-go-submodule/edgex-go/cmd/security-bootstrapper/entrypoint.sh
	cp external/edgex-go-submodule/redis_wait_install_hedge.sh external/edgex-go-submodule/edgex-go/cmd/security-bootstrapper/entrypoint-scripts/redis_wait_install.sh
	cp external/edgex-go-submodule/core-metadata/device.go_hedge external/edgex-go-submodule/edgex-go/internal/core/metadata/application/device.go
	cp external/edgex-go-submodule/security_bootstrapper_redis_configuration_hedge.yaml external/edgex-go-submodule/edgex-go/cmd/security-bootstrapper/res-bootstrap-redis/configuration.yaml
	# Handling Dockerfiles
	cp external/edgex-go-submodule/Dockerfiles/Dockerfile_security_bootstrapper_mod external/edgex-go-submodule/edgex-go/cmd/security-bootstrapper/Dockerfile
	cp external/edgex-go-submodule/Dockerfiles/Dockerfile_security_secretstore_setup_mod external/edgex-go-submodule/edgex-go/cmd/security-secretstore-setup/Dockerfile
	cp external/edgex-go-submodule/Dockerfiles/Dockerfile_support_notifications_mod external/edgex-go-submodule/edgex-go/cmd/support-notifications/Dockerfile
	cp external/edgex-go-submodule/Dockerfiles/Dockerfile_security_proxy_setup_mod external/edgex-go-submodule/edgex-go/cmd/security-proxy-setup/Dockerfile
	cp external/edgex-go-submodule/Dockerfiles/Dockerfile_core_metadata_mod external/edgex-go-submodule/edgex-go/cmd/core-metadata/Dockerfile
	cp external/edgex-go-submodule/Dockerfiles/Dockerfile_core_command_mod external/edgex-go-submodule/edgex-go/cmd/core-command/Dockerfile
	cp external/edgex-go-submodule/Dockerfiles/Dockerfile_core_common_config_bootstrapper_mod external/edgex-go-submodule/edgex-go/cmd/core-common-config-bootstrapper/Dockerfile
	# Handling go.mod and the old go.sum
	cp external/edgex-go-submodule/go.mod_mod external/edgex-go-submodule/edgex-go/go.mod
	rm -f external/edgex-go-submodule/edgex-go/go.sum
	cd external/edgex-go-submodule/edgex-go && \
	go mod tidy && \
	make -j 5 -e DOCKER_TAG=$(EDGEX_VERSION) -e HEDGE_VERSION=$(DOCKER_TAG) -e GO_BASE=$(GO_BASE) -e ALPINE_BASE=$(ALPINE_BASE) -e NODE_BASE=$(NODE_BASE) \
		docker_security_bootstrapper \
		docker_security_secretstore_setup \
		docker_support_notifications \
		docker_security_proxy_setup \
		docker_core_metadata \
		docker_core_command \
		docker_core_common_config
	# Restore original files
	cd external/edgex-go-submodule/edgex-go && git reset --hard

###########################

codecoverage: push-python-test-coverage-base
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--build-arg PYTHON_TEST_COVERAGE_BASE=$(PYTHON_TEST_COVERAGE_BASE) \
		--label "Name=hedge-test-coverage" \
		--no-cache \
		-f coverage.Dockerfile \
		-t hedge-test-coverage:$(GIT_SHA) \
		-t hedge-test-coverage:$(DOCKER_TAG) \
		.

codecoverage-dev:
	docker build \
		$(DOCKER_LABELS) \
		$(DOCKER_BUILDARGS) \
		--label "Name=dev-hedge-test-coverage" \
		-f coverage.dev.Dockerfile \
		-t dev-hedge-test-coverage:$(GIT_SHA) \
		-t dev-hedge-test-coverage:$(DOCKER_TAG) \
		.

###########################

# This list will be used during 'make push' to send the images to docker registry
PUSH_HEDGE_IMAGES= \
	hedge-data-enrichment \
	hedge-admin \
	hedge-device-extensions \
	hedge-event \
	hedge-event-publisher \
	hedge-export \
	hedge-remediate \
	hedge-user-app-mgmt \
	hedge-metadata-notifier \
	hedge-meta-sync \
	hedge-nats-proxy \
	hedge-ml-management \
	hedge-ml-sandbox \
	hedge-ml-broker \
	hedge-ml-edge-agent \
	hedge-ml-python-base \
	hedge-ml-trg-anomaly-autoencoder \
	hedge-ml-pred-anomaly-autoencoder \
	hedge-ml-trg-classification-randomforest \
	hedge-ml-pred-classification-randomforest \
	hedge-ml-trg-timeseries-multivariate-deepvar \
	hedge-ml-pred-timeseries-multivariate-deepvar \
	hedge-ml-trg-regression-lightgbm \
	hedge-ml-pred-regression-lightgbm \
	hedge-init \
	hedgext-nginx \
	hedge-ui-server \
	hedgext-node-red \
	hedgext-ekuiper \
	hedge-swagger-ui

## These optional services should not be pushed to PSG harbor or TPS share
PUSH_OPTIONAL_IMAGES= \
	hedge-export-biz-data \
	hedge-device-virtual \
	hedgext-grafana

PUSH_EDGEX_IMAGES= \
	edgexfoundry/security-bootstrapper \
	edgexfoundry/security-secretstore-setup \
	edgexfoundry/support-notifications \
	edgexfoundry/security-proxy-setup \
	edgexfoundry/core-metadata \
	edgexfoundry/core-command \
	edgexfoundry/core-common-config-bootstrapper

.PHONY: ${PUSH_HEDGE_IMAGES} ${PUSH_OPTIONAL_IMAGES} ${PUSH_EDGEX_IMAGES}


###########################


#override tag if needed, else default picked from VERSION file. eg. 'make push -e NEW_DOCKER_TAG=1.0.0'
NEW_DOCKER_TAG ?= $(DOCKER_TAG)
PYTHON_TEST_COVERAGE_VERSION := $(DOCKER_TAG)
PYTHON_BASE := ${REGISTRY}hedge-ml-python-base:$(DOCKER_TAG)
PYTHON_TEST_COVERAGE_BASE := ${REGISTRY}hedge-python-test-coverage-base:$(NEW_DOCKER_TAG)

myimage: hedge-xxxxx
push-myimage:
	docker tag ${myimage}:$(DOCKER_TAG) $(REGISTRY)${myimage}:$(NEW_DOCKER_TAG)
	docker push $(REGISTRY)${myimage}:$(NEW_DOCKER_TAG)

# This is to push the latest hedge images to PSG harbor repo (for Aqua scanning) & push .tar files to Nexus repo (for Sonatype scanning)
nexus_repo_pass:
nexus_repo_user:
push: $(PUSH_HEDGE_IMAGES)
$(PUSH_HEDGE_IMAGES):
	# Image build will be skipped if nothing changed for a service since last successful build. If so, skip push to registry gracefully.
	docker tag $@:$(DOCKER_TAG) $(REGISTRY)$@:$(NEW_DOCKER_TAG) || echo -e "Image not found. Skipped tagging: $@:$(DOCKER_TAG)"
	docker push $(REGISTRY)$@:$(NEW_DOCKER_TAG) || echo -e "Image not found. Skipping push to registry: $(REGISTRY)$@:$(NEW_DOCKER_TAG)\n"

	# Run with -e nexus_repo_pass=<password> and nexus_repo_user=<username> to use these credentials for pushing .tar files to Nexus
	if [ -z "${nexus_repo_pass}" ] || [ -z "${nexus_repo_user}" ]; then \
		echo -e "\nWARNING: \"nexus_repo_pass\" and/or \"nexus_repo_user\" env(s) are not set. This is required to push tar images to Nexus repository. Skipping..\n"; \
	elif [ "${NEW_DOCKER_TAG}" = "latest" ]; then \
		docker save --output /tmp/$@-$(NEW_DOCKER_TAG).tar $(REGISTRY)$@:$(NEW_DOCKER_TAG) || echo -e "Image not found. Save image as tar skipped:  $@:$(NEW_DOCKER_TAG)"; \
		if [ -f "/tmp/$@-$(NEW_DOCKER_TAG).tar" ]; then \
			if curl -k --fail -X POST -u "${nexus_repo_user}:${nexus_repo_pass}" \
				"https://${SECURITY_TPS_REGISTRY}/service/rest/v1/components?repository=IOT-EDGE" \
				-H 'accept: application/json' \
				-H 'Content-Type: multipart/form-data' \
				-F "raw.directory=/" \
				-F "raw.asset1=@/tmp/$@-$(NEW_DOCKER_TAG).tar;type=application/x-tar" \
				-F "raw.asset1.filename=$@-$(NEW_DOCKER_TAG).tar"; then \
				echo -e "Successfully uploaded /tmp/$@-$(NEW_DOCKER_TAG).tar to Nexus repository 'IOT-EDGE'\n"; \
			else \
				echo -e "Failed to upload /tmp/$@-$(NEW_DOCKER_TAG).tar to Nexus repository 'IOT-EDGE'. Please check the credentials or server availability and try again.\n"; \
			fi; \
		else \
			echo -e "Tar file not found. Skipped uploading to Nexus repository: /tmp/$@-$(NEW_DOCKER_TAG).tar\n"; \
		fi; \
		rm -f /tmp/$@-$(NEW_DOCKER_TAG).tar; \
	else \
		echo -e "Skipped sending tar for $(REGISTRY)$@:$(NEW_DOCKER_TAG) since NOT the latest tag\n"; \
	fi

push-opt-images:$(PUSH_OPTIONAL_IMAGES)
$(PUSH_OPTIONAL_IMAGES):
	# Image build will be skipped if nothing changed for a service since last successful build. If so, skip push to registry gracefully.
	docker tag $@:$(DOCKER_TAG) $(REGISTRY)$@:$(NEW_DOCKER_TAG) || echo -e "Image not found. Skipped tagging: $@:$(DOCKER_TAG)"
	docker push $(REGISTRY)$@:$(NEW_DOCKER_TAG) || echo -e "Image not found. Skipping push to registry: $(REGISTRY)$@:$(NEW_DOCKER_TAG)\n"

# This is to pull edgex and other external images from docker.io and push into harbor
push-edgex-images: ${PUSH_EDGEX_IMAGES} push-ext-images
$(PUSH_EDGEX_IMAGES):
	@$(eval ORIG_EDGEX_IMG := $@:${EDGEX_VERSION})
	@$(eval NEW_EDGEX_IMGNAME := $(shell echo $@ | sed -e "s/edgexfoundry\//hedgext-/g"))
	@$(eval TMP_DOCKERFILE := /tmp/Dockerfile.$(shell basename ${NEW_EDGEX_IMGNAME}))

	@echo "# Creating a temporary Dockerfile to set USER 2002"
	@echo "FROM ${ORIG_EDGEX_IMG}" > ${TMP_DOCKERFILE}
	@echo "USER 2002" >> ${TMP_DOCKERFILE}

	@echo "Building the new image with USER 2002 set"
	@docker build -t $(REGISTRY)${NEW_EDGEX_IMGNAME}:${EDGEX_VERSION} -f ${TMP_DOCKERFILE} .

	@echo "Pushing the new image"
	@docker push $(REGISTRY)${NEW_EDGEX_IMGNAME}:${EDGEX_VERSION}

	@echo "Tagging and pushing the image with the new Docker tag"
	@docker tag $(REGISTRY)${NEW_EDGEX_IMGNAME}:${EDGEX_VERSION} $(REGISTRY)${NEW_EDGEX_IMGNAME}:${NEW_DOCKER_TAG}
	@docker push $(REGISTRY)${NEW_EDGEX_IMGNAME}:${NEW_DOCKER_TAG}

	@echo "Cleaning up the temporary Dockerfile"
	@rm ${TMP_DOCKERFILE}

push-ext-images:
	@{ \
	echo "FROM nats:${NATS_VERSION}"; \
	echo "USER 2002"; \
	} | docker build $(DOCKER_LABELS) -t ${REGISTRY}hedgext-nats:${NATS_VERSION} -f- .
	docker push ${REGISTRY}hedgext-nats:${NATS_VERSION}
	docker tag ${REGISTRY}hedgext-nats:${NATS_VERSION} ${REGISTRY}hedgext-nats:${NEW_DOCKER_TAG}
	docker push ${REGISTRY}hedgext-nats:${NEW_DOCKER_TAG}

	docker build $(DOCKER_LABELS) --build-arg CONSUL_VERSION=$(CONSUL_VERSION) --build-arg ALPINE_BASE=$(ALPINE_BASE) \
			-t ${REGISTRY}hedgext-consul:${CONSUL_VERSION} -f external/consul/Dockerfile .
	docker push ${REGISTRY}hedgext-consul:${CONSUL_VERSION}
	docker tag ${REGISTRY}hedgext-consul:${CONSUL_VERSION} ${REGISTRY}hedgext-consul:${NEW_DOCKER_TAG}
	docker push ${REGISTRY}hedgext-consul:${NEW_DOCKER_TAG}

	docker build $(DOCKER_LABELS) --build-arg VAULT_VERSION=$(VAULT_VERSION) --build-arg ALPINE_BASE=$(ALPINE_BASE) \
			-t ${REGISTRY}hedgext-vault:${VAULT_VERSION} -f external/vault/Dockerfile .
	docker push ${REGISTRY}hedgext-vault:${VAULT_VERSION}
	docker tag ${REGISTRY}hedgext-vault:${VAULT_VERSION} ${REGISTRY}hedgext-vault:${NEW_DOCKER_TAG}
	docker push ${REGISTRY}hedgext-vault:${NEW_DOCKER_TAG}

	docker build $(DOCKER_LABELS) --build-arg POSTGRES_VER=$(POSTGRES_VER) \
				-t ${REGISTRY}hedgext-postgres:${POSTGRES_VER} -f external/postgres/Dockerfile .
	docker push ${REGISTRY}hedgext-postgres:${POSTGRES_VER}
	docker tag ${REGISTRY}hedgext-postgres:${POSTGRES_VER} ${REGISTRY}hedgext-postgres:${NEW_DOCKER_TAG}
	docker push ${REGISTRY}hedgext-postgres:${NEW_DOCKER_TAG}

	docker build $(DOCKER_LABELS) --build-arg REDIS_VER=$(REDIS_VER) \
				-t ${REGISTRY}hedgext-redis:${REDIS_VER} -f external/redis/Dockerfile .
	docker push ${REGISTRY}hedgext-redis:${REDIS_VER}
	docker tag ${REGISTRY}hedgext-redis:${REDIS_VER} ${REGISTRY}hedgext-redis:${NEW_DOCKER_TAG}
	docker push ${REGISTRY}hedgext-redis:${NEW_DOCKER_TAG}

	@{ \
	echo "FROM eclipse-mosquitto:${MOSQUITTO_VERSION}"; \
	echo "USER 2002"; \
	} | docker build $(DOCKER_LABELS) -t ${REGISTRY}hedgext-mosquitto:${MOSQUITTO_VERSION} -f- .
	docker push ${REGISTRY}hedgext-mosquitto:${MOSQUITTO_VERSION}
	docker tag ${REGISTRY}hedgext-mosquitto:${MOSQUITTO_VERSION} ${REGISTRY}hedgext-mosquitto:${NEW_DOCKER_TAG}
	docker push ${REGISTRY}hedgext-mosquitto:${NEW_DOCKER_TAG}

	## Should not be pushed to PSG harbor
	@{ \
	echo "FROM opensearchproject/opensearch:$(ELASTIC_SEARCH_VERSION)"; \
	} | docker build $(DOCKER_LABELS) -t ${REGISTRY}hedgext-opensearch-es:$(ELASTIC_SEARCH_VERSION) -f- .
	docker tag ${REGISTRY}hedgext-opensearch-es:$(ELASTIC_SEARCH_VERSION) ${REGISTRY}hedgext-opensearch-es:${NEW_DOCKER_TAG}
	@if [ "$(shell echo ${REGISTRY} | cut -c1-3)" != "psg" ]; then \
		docker push ${REGISTRY}hedgext-opensearch-es:$(ELASTIC_SEARCH_VERSION); \
		docker push ${REGISTRY}hedgext-opensearch-es:${NEW_DOCKER_TAG}; \
	else \
		echo "Skip pushing opensearch-elasticsearch image to ${REGISTRY} registry"; \
	fi

	## Should not be pushed to PSG harbor
	@{ \
	echo "FROM victoriametrics/victoria-metrics:$(VICTORIA_METRICS_VERSION)"; \
	echo "USER 2002"; \
	} | docker build $(DOCKER_LABELS) -t ${REGISTRY}hedgext-victoria-metrics:$(VICTORIA_METRICS_VERSION) -f- .
	docker tag ${REGISTRY}hedgext-victoria-metrics:$(VICTORIA_METRICS_VERSION) ${REGISTRY}hedgext-victoria-metrics:${NEW_DOCKER_TAG}
	@if [ "$(shell echo ${REGISTRY} | cut -c1-3)" != "psg" ]; then \
		docker push ${REGISTRY}hedgext-victoria-metrics:$(VICTORIA_METRICS_VERSION); \
		docker push ${REGISTRY}hedgext-victoria-metrics:${NEW_DOCKER_TAG}; \
	else \
		echo "Skip pushing victoria-metrics image to ${REGISTRY} registry"; \
	fi

