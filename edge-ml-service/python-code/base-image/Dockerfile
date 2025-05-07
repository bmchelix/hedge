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

# Default value: placeholder to pull latest version for dev, will be overridden from Makefile
ARG ALPINE_BASE=alpine:latest
FROM ${ALPINE_BASE}

# Switch to root user for installation
USER root

# Update and install basic required packages
RUN apk update && \
    apk upgrade && \
    apk add --no-cache libstdc++ hdf5-dev

# Set locale and environment variables
ENV LANG=C.UTF-8
ENV LC_ALL=en_US.utf8
ENV PYTHONPATH=/opt/bmc
ENV PATH=/opt/bmc/python/bin:$PATH

WORKDIR /opt/bmc

# Install glibc without overwriting gcompat files
RUN apk add --no-cache libc6-compat libc-dev && \
    wget --quiet \
        http://onebuild.bmc.com/onepatrol_universe/maven/python-alpine-image-support/2.38-r1-bmc/glibc-2.38-r1-bmc.apk \
        http://onebuild.bmc.com/onepatrol_universe/maven/python-alpine-image-support/2.38-r1-bmc/glibc-bin-2.38-r1-bmc.apk \
        http://onebuild.bmc.com/onepatrol_universe/maven/python-alpine-image-support/2.38-r1-bmc/glibc-i18n-2.38-r1-bmc.apk && \
    apk add --no-cache --allow-untrusted --force-overwrite \
        glibc-2.38-r1-bmc.apk glibc-bin-2.38-r1-bmc.apk glibc-i18n-2.38-r1-bmc.apk && \
    /usr/glibc-compat/bin/localedef --force --inputfile POSIX --charmap UTF-8 $LANG || true && \
    echo "export LANG=$LANG" > /etc/profile.d/locale.sh

# Download and install Miniconda
ARG SHA256_HASH=4da8dde69eca0d9bc31420349a204851bfa2a1c87aeb87fe0c05517797edaac4
RUN CONDA_VERSION="py311_24.3.0-0" && \
    wget https://repo.anaconda.com/miniconda/Miniconda3-${CONDA_VERSION}-Linux-x86_64.sh -O miniconda.sh && \
    echo "${SHA256_HASH}  miniconda.sh" | sha256sum -c - || (echo "Checksum failed!" >&2; exit 1) && \
    sh miniconda.sh -b -p /opt/bmc/python && \
    rm miniconda.sh && \
    /opt/bmc/python/bin/python -V && \
    chmod 755 /opt/bmc && \
    rm -rf /root/.wget-hsts glibc-2.38-r1-bmc.apk glibc-bin-2.38-r1-bmc.apk \
        glibc-i18n-2.38-r1-bmc.apk miniconda.sh /root/.cache/pip /opt/bmc/python/pkgs /opt/java && \
    find /opt/bmc/python/ -type f -name '*.a' -delete && \
    find /opt/bmc/python/ -type f -name '*.js.map' -delete && \
    rm -rf /opt/bmc/python/lib/python3.11/site-packages/Pillow-*-py*.egg-info && \
    rm -rf /opt/bmc/python/lib/python3.11/site-packages/future/backports/test/*.pem && \
    rm -rf /opt/bmc/python/bin/openssl \
            /opt/bmc/python/lib/python3.11/site-packages/tests/conda_env/support/requirements.txt && \
    pip uninstall cryptography -y && \
    conda clean -afy && \
    rm /opt/bmc/python/bin/xml2-config && \
    rm -rf /root/.cache /tmp/* /opt/bmc/python/share /opt/bmc/python/include /opt/bmc/python/doc

RUN pip install --upgrade pip && \
    pip install --upgrade idna tqdm certifi urllib3 watchdog flatdict

# Create a non-root user
# RUN addgroup -S mluser && adduser -S mluser -G mluser
# Create a user with specific UID and GID, need to ensure same UID and GID since we will use this to set
# right permission in ml-management and ml-sandbox
RUN addgroup -S -g 2001 mluser && adduser -S -u 2002 -G mluser mluser

# Set the appropriate ownership for the directory
RUN chown -R mluser:mluser /opt/bmc

# Set user for the image
USER mluser

# Set environment variables for Python
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

# Final command, to be overridden by the derived image
ENTRYPOINT ["python", "-V"]
