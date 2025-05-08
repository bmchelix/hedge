# (c) Copyright 2020-2025 BMC Software, Inc.
#
# Contributors: BMC Software, Inc. - BMC Helix Edge

#build stage
# Default values: placeholders to pull latest version for dev, will be overridden from Makefile
ARG GO_BASE=golang:latest
ARG ALPINE_BASE=alpine:latest

FROM ${GO_BASE} AS builder

# Switch to root user for installation
USER root

ARG ALPINE_PKG_BASE="make git"
ARG ALPINE_PKG_EXTRA=""

RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

WORKDIR /hedge
COPY go.mod .
RUN go mod download all
COPY . .

#create binary file in the container
RUN GO111MODULE=on go build -ldflags "-X hedge.Version=latest" -o nats-proxy ./app-services/nats-proxy/main.go

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
COPY --from=builder --chown=edgex:edgex /hedge/app-services/nats-proxy/res /res
COPY --from=builder --chown=edgex:edgex /hedge/nats-proxy /nats-proxy

# Change ownership of the application to the non-root user
RUN chown -R edgex:edgex /res /nats-proxy /etc/ssl/certs

RUN mkdir -p /res/edge/models
RUN chmod -R u+rwx /res
RUN chmod u+x /nats-proxy

# Switch to the non-root 'edgex' user
USER 2002

ENTRYPOINT ["./nats-proxy"]
CMD ["-cp=consul.http://edgex-core-consul:8500", "--registry", "--configDir=./res"]
