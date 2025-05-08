# (c) Copyright 2020-2025 BMC Software, Inc.
#
# Contributors: BMC Software, Inc. - BMC Helix Edge

ARG KUIPER_VERSION
FROM lfedge/ekuiper:${KUIPER_VERSION}

# Remove the packaged sample private/public keys
RUN rm -f /kuiper/etc/mgmt/sample_key /kuiper/etc/mgmt/sample_key.pub
