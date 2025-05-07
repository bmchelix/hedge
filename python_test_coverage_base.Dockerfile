FROM --platform=linux/amd64 python:3.11 AS stage3_pycoverage

MAINTAINER sgritsan@bmc.com
LABEL description="Base image for coverage test of python code in edge-ml-service"

RUN apt-get -y update && \
    apt-get install vim python3-h5py -y

#
### Upgrade pip with no cache
RUN pip install --no-cache-dir -U pip
RUN pip install --upgrade pip setuptools wheel

COPY edge-ml-service/python-code/tests/requirements.txt requirements.txt

RUN pip install -r requirements.txt