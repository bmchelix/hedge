## Common base image for Python-based images in HEDGE

To align with security recommendations, we are considering Alpine as the core base image for our Python-based ML services and avoiding the use of Debian.

Some of our images require "heavy" dependencies like TensorFlow, which have prerequisites not included in standard Alpine or Python-Alpine images. 
To prevent duplication and reduce build times for these ML images, we recommend using our hedge-ml-python-base image as a base, either by building it locally or pulling it from the PSG Pune registry.

For more lightweight Python-based images, consider using the standard Python-Alpine image as the base.

### Why to use Miniconda?

The 'python-alpine' image uses **musl** libc instead of **glibc**. 
In our case TensorFlow (and some other dependencies), is compiled against glibc and doesn't work properly with musl. 
This leads to compatibility issues. With Miniconda, we are using pre-compiled conda packages that are known to work together, minimizing dependency conflicts.

### How to use the 'hedge-ml-python-base'

In Jenkins the base image is being re-built and pushed to PSG each time the job is triggered to have this image up-to-date.
If you want to trigger build or/and push the base image manually - you could run one of the commands below (from edge-iot/):

> make push-ml-python-base

(builds the 'hedge-ml-python-base' image, re-tags and pushes to PSG Pune Harbor registry)

> make hedge_ml_python_base

(builds the 'hedge-ml-python-base' image)

To use this image as a base you need to pull it in your Dockerfile: 
```from
FROM ${PYTHON_BASE}
```

The PYTHON_BASE var defined in edge-iot/Makefile. The image path by default: 
>  psg-hrbr-aus.bmc.com/iot/hedge-ml-python-base:internal

Please use existing Dockerfiles in edge-iot/edge-ml-service/python-code/ as example, or the Dockerfile attached below:

```dockerfile
# Use the base image which contains all the necessary installations
ARG PYTHON_BASE=hedge-ml-python-base:internal
FROM ${PYTHON_BASE}

USER root

# Set working directory for your application
WORKDIR /edge-iot

# Define the root folder for the application files
ARG ROOT_FOLDER=anomaly/autoencoder

# Copy the application-specific files
COPY ${ROOT_FOLDER}/infer/requirements.txt requirements.txt
COPY ${ROOT_FOLDER}/env.yaml anomaly/autoencoder/env.yaml

# Create directories as needed
RUN mkdir -p /edge-iot/common/ && \
    mkdir -p /edge-iot/infer/ && \
    mkdir -p /edge-iot/tmp/hedge

# Install Python packages
RUN pip install --no-cache-dir --upgrade pip && \
    pip install -r requirements.txt && \
    rm -rf /root/.cache /tmp/*

# Copy additional files
COPY ${ROOT_FOLDER}/../../common/ common/
COPY ${ROOT_FOLDER}/infer/ infer/

# Set the ownership and permissions for the application directory
RUN chown -R mluser:mluser /edge-iot && \
    chmod a+x infer/src/main/task.py

# Use the non-root user created in the base image
USER mluser

# Set the PYTHONPATH environment variable
ENV PYTHONPATH="/edge-iot"

# Run the application
ENTRYPOINT ["python", "./infer/src/main/task.py"]
```