# (c) Copyright 2020-2025 BMC Software, Inc.
#
# Contributors: BMC Software, Inc. - BMC Helix Edge

ARG NODERED_VERSION
FROM nodered/node-red:${NODERED_VERSION}

RUN npm install node-red-dashboard@3.6.5
RUN npm install node-red-contrib-pythonshell

COPY theme /data/theme
COPY settings.js /data/
COPY xmlHttpRequest_interceptor.js /tmp/xmlHttpRequest_interceptor.js
COPY insert_xmlHttpRequest_interceptor.sh /tmp/insert_xmlHttpRequest_interceptor.sh

USER root

# Temp fix: revice once node-red@v4.0.6 released
RUN npm install -g npm@10.9.1
# Upgrade according to the PSG-automation team recommendations
# https://github.bmc.com/secuprod/container-base-images/blob/main/Dockerfile/Dockerfile.alpine_base
RUN apk upgrade --no-cache

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

RUN chown -R root:root /tmp/insert_xmlHttpRequest_interceptor.sh && chmod u+x  /tmp/insert_xmlHttpRequest_interceptor.sh

# Execute the shell script to modify red.min.js
RUN /tmp/insert_xmlHttpRequest_interceptor.sh

# Switch to the non-root 'edgex' user
USER 2002
