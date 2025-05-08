ARG CONSUL_VERSION
FROM hashicorp/consul:${CONSUL_VERSION}

USER root 

# Upgrade according to the PSG-automation team recommendations
# https://github.bmc.com/secuprod/container-base-images/blob/main/Dockerfile/Dockerfile.alpine_base
RUN apk upgrade --no-cache

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

# Switch to the non-root 'edgex' user
USER 2002
