# Copyright (c) 2020 IOTech Ltd
# (c) Copyright 2020-2025 BMC Software, Inc.
#
# Contributors: BMC Software, Inc. - BMC Helix Edge
#
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

ARG ALPINE_PKG_BASE="make git openssh-client gcc libc-dev zeromq-dev libsodium-dev"
ARG ALPINE_PKG_EXTRA=""
RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

WORKDIR /hedge
COPY go.mod .
#COPY device-services/device-virtual/_build ./edgexfoundry/
RUN go mod download all
COPY . .

RUN GO111MODULE=on go build -ldflags "-X hedge.Version=latest" -o device-virtual ./device-services/device-virtual/cmd/main.go

#==========================================================================================================================================================

FROM ${ALPINE_BASE}

# Switch to root user for installation
USER root

RUN apk add --update --no-cache ca-certificates dumb-init

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

ENV APP_PORT=49991
EXPOSE $APP_PORT

WORKDIR /
COPY --from=builder /hedge/device-services/device-virtual/cmd /cmd
COPY --from=builder /hedge/device-services/device-virtual/cmd/res /res
COPY --from=builder /hedge/device-virtual /device-virtual

# Change ownership of the application to the non-root user
RUN chown -R edgex:edgex /cmd /res /device-virtual /etc/ssl/certs

# Ensure the binary is executable by the owner
RUN chmod u+x /device-virtual

# Switch to the non-root 'edgex' user
USER 2002

ENTRYPOINT ["./device-virtual"]
CMD ["--cp=consul.http://edgex-core-consul:8500","--registry","--configDir=./res"]
