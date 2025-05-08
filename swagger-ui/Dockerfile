#FROM swaggerapi/swagger-ui:v5.18.2
#
#COPY swagger.json /swagger.json
#
#ENV SWAGGER_JSON=/swagger.json
#


#
# Copyright (c) 2019 Intel Corporation
# (c) Copyright 2020-2025 BMC Software, Inc.
#
# Contributors: BMC Software, Inc. - BMC Helix Edge
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Default values: placeholders to pull latest version for dev, will be overridden from Makefile
ARG GO_BASE=golang:latest
ARG ALPINE_BASE=alpine:latest

FROM ${GO_BASE} AS builder

# Switch to root user for installation
USER root

ARG ALPINE_PKG_BASE="make git gcc libc-dev libsodium-dev jq"
ARG ALPINE_PKG_EXTRA=""

RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

# add git for go modules
WORKDIR /hedge
COPY go.mod .
RUN go mod download all
COPY . .

RUN go get github.com/swaggo/swag/cmd/swag
RUN go run github.com/swaggo/swag/cmd/swag init --parseInternal=true --generalInfo=doc.go --pd=true --ot=json --output=/hedge/swagger-ui/res/swagger/.tmp/

RUN jq -S . /hedge/swagger-ui/res/swagger/swagger.json > /hedge/swagger-ui/res/swagger/.tmp/f1.json && jq -S . /hedge/swagger-ui/res/swagger/.tmp/swagger.json > /hedge/swagger-ui/res/swagger/.tmp/f2.json

RUN	if diff /hedge/swagger-ui/res/swagger/.tmp/f1.json /hedge/swagger-ui/res/swagger/.tmp/f2.json >/dev/null; then \
  		echo "Swagger specification is up-to-date."; \
	else \
  		echo "Discrepancy found between ./swagger-ui/res/swagger/swagger.json in Git and generated swagger specification. Please regenerate swagger using make generate-swagger-spec and execute Git commit and push. "; \
  		exit 1; \
  	fi

#create binary file in the container
RUN CGO_ENABLED=1 GO111MODULE=on go build -ldflags "-X hedge.Version=latest" -o swagger-ui ./swagger-ui/main.go

#==========================================================================================================================================================
FROM ${ALPINE_BASE}

# Switch to root user for installation
USER root

RUN apk add --update --no-cache ca-certificates dumb-init

# Turn off secure mode for examples. Not recommended for production
#ENV EDGEX_SECURITY_SECRET_STORE=false

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

WORKDIR /
# Copy application files and configuration with 'edgex' user and group ownership
COPY --from=builder --chown=edgex:edgex /hedge/swagger-ui /swagger-ui

# Ensure the main binary is executable
RUN chmod u+x /swagger-ui

# Switch to the non-root 'edgex' user
USER 2002

WORKDIR swagger-ui/
ENTRYPOINT ["./main"]
