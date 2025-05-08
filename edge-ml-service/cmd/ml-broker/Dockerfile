# Copyright 2021 The TensorFlow Authors. All Rights Reserved.
# (c) Copyright 2020-2025 BMC Software, Inc.
#
# Contributors: BMC Software, Inc. - BMC Helix Edge
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# ============================================================================

#build stage
# Default values: placeholders to pull latest version for dev, will be overridden from Makefile
ARG GO_BASE=golang:latest
ARG ALPINE_BASE=alpine:latest

FROM ${GO_BASE} AS builder

# Switch to root user for installation
USER root

ARG ALPINE_PKG_BASE="make git"
ARG ALPINE_PKG_EXTRA=""

# RUN sed -e 's/dl-cdn[.]alpinelinux.org/nl.alpinelinux.org/g' -i~ /etc/apk/repositories
RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

# add git for go modules
WORKDIR /hedge
COPY go.mod .

# Not sure what is the purpose here
RUN go mod download all
COPY edge-ml-service edge-ml-service
COPY common common
# Check if the below is reqired or not
#RUN apk info -a zeromq-dev

#create binary file in the container
RUN GO111MODULE=on go build -ldflags "-X hedge.Version=latest" -o hedge-ml-broker ./edge-ml-service/cmd/ml-broker/main.go

#---------------------------------

#final stage
FROM ${ALPINE_BASE}

# Switch to root user for installation
USER root

# dumb-init is required as security-bootstrapper uses it in the entrypoint script
RUN apk add --update --no-cache ca-certificates dumb-init
#RUN apk --no-cache add zeromq

# Turn off secure mode for examples. Not recommended for production
#ENV EDGEX_SECURITY_SECRET_STORE=false

#COPY --from=builder /app/Attribution.txt /Attribution.txt
#COPY --from=builder /app/LICENSE /LICENSE

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

WORKDIR /
# Copy application files and configuration with 'edgex' user and group ownership
COPY --from=builder /hedge/edge-ml-service/cmd/ml-broker/res /res
COPY --from=builder /hedge/hedge-ml-broker /hedge-ml-broker

RUN chown -R edgex:edgex /res /hedge-ml-broker

RUN mkdir -p /res/edge/models
RUN chmod -R u+rwx /res
RUN chmod u+x /hedge-ml-broker

EXPOSE 48120

# Switch to the non-root 'edgex' user
USER 2002

ENTRYPOINT ["./hedge-ml-broker"]
CMD ["-cp=consul.http://edgex-core-consul:8500", "--registry", "--configDir=./res"]
