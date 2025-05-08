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

ARG ALPINE_PKG_BASE="make git"
ARG ALPINE_PKG_EXTRA=""

RUN sed -e 's/dl-cdn[.]alpinelinux.org/nl.alpinelinux.org/g' -i~ /etc/apk/repositories
RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

# add git for go modules
WORKDIR /hedge
COPY go.mod .
RUN go mod download all
COPY . .

#create binary file in the container
RUN GO111MODULE=on go build -ldflags "-X hedge.Version=latest" -o hedge-ml-edge-agent ./edge-ml-service/cmd/ml-edge-agent/main.go

#==========================================================================================================================================================

FROM ${ALPINE_BASE}

# Switch to root user for installation
USER root

RUN apk add --update --no-cache ca-certificates dumb-init

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

WORKDIR /
COPY --from=builder /hedge/hedge-ml-edge-agent /hedge-ml-edge-agent
COPY --from=builder /hedge/edge-ml-service/cmd/ml-edge-agent/res /res

# Change ownership of the application to the non-root user
RUN chown -R edgex:edgex /res /hedge-ml-edge-agent /etc/ssl/certs

RUN mkdir -p /res/edge/models
RUN chmod -R u+rwx /res
RUN chmod u+x /hedge-ml-edge-agent

EXPOSE 49100

# Switch to the non-root 'edgex' user
USER 2002

ENTRYPOINT ["./hedge-ml-edge-agent"]
CMD ["-cp=consul.http://edgex-core-consul:8500", "--registry", "--configDir=./res"]
