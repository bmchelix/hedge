ARG PYTHON_BASE=hedge-ml-python-base:internal
FROM ${PYTHON_BASE}

USER root

# Set working directory for your application
WORKDIR /edge-iot

ARG ROOT_FOLDER=/timeseries/multivariate_deepVAR

# set environment variables
ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

WORKDIR /edge-iot

RUN ls -altr 

RUN mkdir -p /edge-iot/${ROOT_FOLDER}

COPY ${ROOT_FOLDER}/infer/requirements.txt requirements.txt
COPY ${ROOT_FOLDER}/env.yaml timeseries/multivariate_deepVAR/env.yaml

# Install Python packages
RUN pip install --no-cache-dir --upgrade pip && \
    pip install -r requirements.txt && \
    rm -rf /root/.cache /tmp/*

# Copy additional files
RUN mkdir -p /edge-iot/common/
RUN mkdir -p /edge-iot/infer/
RUN mkdir -p /edge-iot/tmp/hedge

COPY ${ROOT_FOLDER}/../../common/ common/
COPY ${ROOT_FOLDER}/infer/ infer/

ENV PYTHONPATH="/edge-iot"

# Set the ownership and permissions for the application directory
RUN chown -R mluser:mluser /edge-iot && \
    chmod a+x infer/src/main/task.py

USER mluser

ENTRYPOINT ["python", "./infer/src/main/task.py"]
