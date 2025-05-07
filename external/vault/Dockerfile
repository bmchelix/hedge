ARG VAULT_VERSION
FROM hashicorp/vault:${VAULT_VERSION} AS base

# Rebuild Vault with updated dependencies
FROM golang:1.23-alpine AS builder

# Install necessary build tools
RUN apk add --no-cache git make bash

# Install enumer tool required by Vault's build process
RUN go install github.com/dmarkham/enumer@latest

WORKDIR /build

ARG VAULT_VERSION
# Clone the Vault repository using the specified version
RUN export VERSION=$(echo ${VAULT_VERSION} | sed 's/-alpine//') && \
    git clone --branch "v${VERSION}" --depth 1 https://github.com/hashicorp/vault.git /build

# Upgrade github.com/hashicorp/go-slug to v0.16.3
RUN cd /build && \
    go mod edit -require=github.com/hashicorp/go-slug@v0.16.3 && \
    go mod tidy

# Build the Vault binary
RUN cd /build && make dev

FROM base

COPY --from=builder /build/bin/vault /bin/vault

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

# Switch to the non-root 'edgex' user
USER 2002
