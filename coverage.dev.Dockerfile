#
# Copyright (c) 2023 Intel Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

FROM golang:1.23-alpine3.20 AS stage2_gocoverage

ARG ALPINE_PKG_BASE="make git"
ARG ALPINE_PKG_EXTRA=""

RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

WORKDIR /hedge

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

# Delete the old report if it exists
RUN rm -f sonarqube/coverage.go_dev.out

# Generate coverage report
RUN go test ./... -coverprofile=sonarqube/coverage.go_dev.out -coverpkg=./...;

# Run sonarqube scanner
FROM sonarsource/sonar-scanner-cli:5.0.1

WORKDIR /hedge

# Copy the UI files from the previous stages
COPY --from=stage2_gocoverage /hedge/sonarqube/coverage.go_dev.out ./sonarqube/coverage.go_dev.out

COPY . .
ADD sonarqube/run.sh .
ADD sonarqube/run-dev.sh .

# Run UI sonarqube Coverage
CMD ["./run-dev.sh"]
