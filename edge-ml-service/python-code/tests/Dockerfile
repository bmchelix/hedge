ARG PYTHON_BASE=hedge-ml-python-base:internal
FROM ${PYTHON_BASE}

USER root

# Set working directory for your application
WORKDIR /edge-iot

ARG ROOT_FOLDER=/tests

# set environment variables
ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

WORKDIR /edge-iot

RUN ls -altr

RUN mkdir -p /edge-iot/${ROOT_FOLDER}

COPY ${ROOT_FOLDER}/requirements.txt requirements.txt

# Install Python packages
RUN pip install --no-cache-dir --upgrade pip && \
    pip install -r requirements.txt && \
    rm -rf /root/.cache /tmp/*

RUN pip install pytest-cov==6.0.0

# Copy additional files
RUN mkdir -p /edge-iot/anomaly/
RUN mkdir -p /edge-iot/classification/
RUN mkdir -p /edge-iot/clustering/
RUN mkdir -p /edge-iot/common/
RUN mkdir -p /edge-iot/regression/
RUN mkdir -p /edge-iot/timeseries/

COPY ${ROOT_FOLDER}/ tests/
COPY ${ROOT_FOLDER}/../anomaly/ anomaly/ 
COPY ${ROOT_FOLDER}/../classification/ classification/ 
COPY ${ROOT_FOLDER}/../clustering/ clustering/ 
COPY ${ROOT_FOLDER}/../common/ common/ 
COPY ${ROOT_FOLDER}/../regression/ regression/ 
COPY ${ROOT_FOLDER}/../timeseries/ timeseries/ 

COPY ${ROOT_FOLDER}/../.coveragerc .


# # Set the ownership and permissions for the application directory
# RUN chown -R mluser:mluser /edge-iot && \
#     chmod a+x /edge-iot/*

# # Use the non-root user created in the base image
# USER mluser

RUN ls -altr 

# Set the PYTHONPATH environment variable
ENV PYTHONPATH="/edge-iot"

ENTRYPOINT ["pytest", "--cov=.", "--cov-report=html:coverage_report", "--cov-report=term-missing" , "--cov-config=.coveragerc"]