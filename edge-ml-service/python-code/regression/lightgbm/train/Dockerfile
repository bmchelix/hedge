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

# Use the base image which contains all the necessary installations
ARG PYTHON_BASE=hedge-ml-python-base:internal
FROM ${PYTHON_BASE}

USER root

# Set working directory for your application
WORKDIR /edge-iot

# Define the root folder for the application files
ARG ROOT_FOLDER=regression/lightgbm

# Copy the application-specific files
COPY ${ROOT_FOLDER}/train/requirements.txt requirements.txt
COPY ${ROOT_FOLDER}/env.yaml regression/lightgbm/env.yaml

ENV LOCAL=False

# Create directories as needed
RUN mkdir -p /edge-iot/common/ && \
    mkdir -p /edge-iot/train/ && \
    mkdir -p /edge-iot/tmp/hedge

# Install Python packages
RUN pip install --no-cache-dir --upgrade pip && \
    pip install -r requirements.txt && \
    rm -rf /root/.cache /tmp/*

# Copy additional files
COPY ${ROOT_FOLDER}/../../common/ common/
COPY ${ROOT_FOLDER}/train/ train/

# Set the ownership and permissions for the application directory
RUN chown -R mluser:mluser /edge-iot && \
    chmod a+x train/src/main/task.py

# Use the non-root user created in the base image
USER mluser

# Set the PYTHONPATH environment variable
ENV PYTHONPATH="/edge-iot"

# Run the application
ENTRYPOINT ["python", "./train/src/main/task.py"]
