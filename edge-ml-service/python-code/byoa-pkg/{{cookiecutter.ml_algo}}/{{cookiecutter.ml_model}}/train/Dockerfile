ARG PYTHON_BASE={{cookiecutter.base_image}}
FROM ${PYTHON_BASE}

USER root

# Set working directory for your application
WORKDIR /edge-iot

ARG ROOT_FOLDER={{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}

# set environment variables
ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

WORKDIR /edge-iot

RUN ls -altr 

RUN mkdir -p /edge-iot/${ROOT_FOLDER}

COPY ${ROOT_FOLDER}/train/requirements.txt requirements.txt
COPY ${ROOT_FOLDER}/env.yaml {{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/env.yaml

ENV LOCAL False

# Install Python packages
RUN pip install --no-cache-dir --upgrade pip && \
    pip install -r requirements.txt && \
    rm -rf /root/.cache /tmp/*

# Copy additional files
RUN mkdir -p /edge-iot/tmp/hedge

COPY ${ROOT_FOLDER}/train/ ${ROOT_FOLDER}/train/
COPY ${ROOT_FOLDER}/common/ ${ROOT_FOLDER}/common/

# Set the ownership and permissions for the application directory
RUN chown -R mluser:mluser /edge-iot && \
    chmod a+x /edge-iot/{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/train/src/task.py

# Use the non-root user created in the base image
USER mluser

# Set the PYTHONPATH environment variable
ENV PYTHONPATH="/edge-iot/{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}"

ENTRYPOINT ["python", "./{{cookiecutter.ml_algo}}/{{cookiecutter.ml_model}}/train/src/task.py"]
