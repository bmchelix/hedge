# Copyright (c) 2020 IOTech Ltd
# (c) Copyright 2020-2025 BMC Software, Inc.
#
# Contributors: BMC Software, Inc. - BMC Helix Edge
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
RUN apk add --update --no-cache pkgconfig build-base openssh-client ca-certificates curl

# Replicate the APK repository override.
# If it is no longer necessary to avoid the CDN mirros we should consider dropping this as it is brittle..

#RUN sed -e 's/dl-cdn[.]alpinelinux.org/nl.alpinelinux.org/g' -i~ /etc/apk/repositories
#RUN apk update && apk add zeromq-dev libsodium-dev pkgconfig build-base git

WORKDIR /hedge
COPY go.mod .
#COPY edge-ml-service/cmd/ml-management/_build ./edgexfoundry/
RUN go mod download all
COPY . .

RUN go test ./edge-ml-service/...

RUN GO111MODULE=on go build -ldflags "-X hedge.Version=latest" -o ml-management ./edge-ml-service/cmd/ml-management/main.go

#==========================================================================================================================================================

FROM ${ALPINE_BASE}

# Switch to root user for installation
USER root

#WORKDIR /edgex-iot/edge-ml-service/cmd/ml-management

#RUN sed -e 's/dl-cdn[.]alpinelinux.org/nl.alpinelinux.org/g' -i~ /etc/apk/repositories
#RUN apk --no-cache add zeromq

RUN apk add --update --no-cache ca-certificates dumb-init zip attr

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

EXPOSE 48095

# Turn off secure mode for examples. Not recommended for production

WORKDIR /
# Copy application files and configuration with 'edgex' user and group ownership
COPY --from=builder /hedge/edge-ml-service/cmd/ml-management/res /res
COPY --from=builder /hedge/ml-management /hedge-ml-management

# Copy and make the cleanup script executable
COPY --from=builder /hedge/edge-ml-service/cmd/ml-management/scripts/clean_zip.sh /usr/local/bin/clean_zip.sh
RUN chown edgex:edgex /usr/local/bin/clean_zip.sh && chmod +x /usr/local/bin/clean_zip.sh

# Change ownership to 'edgex' user and set broad permissions on /res
RUN chown -R edgex:edgex /res /hedge-ml-management

RUN mkdir -p /res/models
RUN chmod -R u+rwx /res
RUN chmod u+x /hedge-ml-management

# Switch to the non-root 'edgex' user
USER 2002

ENTRYPOINT ["./hedge-ml-management"]
CMD ["--cp=consul.http://edgex-core-consul:8500","--registry","--configDir=./res"]