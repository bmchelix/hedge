# Default value: placeholder to pull latest version for dev, will be overridden from Makefile
ARG ALPINE_BASE=alpine:latest
FROM ${ALPINE_BASE}

# Switch to root user for installation
USER root

ARG ALPINE_PKG_BASE="make git openssl jq rsync curl"
ARG ALPINE_PKG_EXTRA="mosquitto mosquitto-clients postgresql-client"

RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

WORKDIR /hedge

# Copy the scripts with 'edgex' user and group ownership, set permissions
COPY --chown=edgex:edgex hedge-scripts ./hedge-scripts
RUN chmod -R u+x ./hedge-scripts

# Copy the main run script with 'edgex' user and group ownership, set permissions
#COPY /usr/bin/mosquitto_passwd /
ADD --chown=edgex:edgex run.sh .
RUN chmod u+x run.sh

# Switch to the non-root 'edgex' user
USER 2002

CMD ["./run.sh"]
