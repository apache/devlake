# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements.  See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License.  You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# uv version matches backend/Dockerfile; Dependabot updates both FROM lines.
FROM ghcr.io/astral-sh/uv:0.13.7 AS uv

# Same Python line as the runtime image in backend/Dockerfile.
FROM python:3.14-slim-trixie

RUN apt -y update && apt -y upgrade && apt -y install tzdata make tar curl gcc g++ pkg-config git \
    zlib1g libffi-dev  \
    default-libmysqlclient-dev \
    libpq-dev

# Install Golang
# The previous installer was fetched from `git.io`, which GitHub shut down in
# 2022, so the image could no longer be rebuilt. Fetch the official archive
# from go.dev and verify it. Version and checksum match
# backend/scripts/install-go.sh.
ARG GO_VERSION=1.27.3
ARG GO_SHA256=65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40
RUN curl --fail --location --retry 3 "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" --output /tmp/go.tar.gz &&\
    echo "${GO_SHA256}  /tmp/go.tar.gz" | sha256sum --check --strict &&\
    tar -C /usr/local -xzf /tmp/go.tar.gz && rm /tmp/go.tar.gz &&\
    mkdir -p /go &&\
    ln -sf /usr/local/go/bin/* /usr/bin

# Install Golang Tools (versions match backend/Dockerfile and backend/Makefile)
RUN export GOPATH=/go &&  \
    go install github.com/vektra/mockery/v3@v3.9.0 &&  \
    go install github.com/swaggo/swag/cmd/swag@v1.16.6

# Golang Env
ENV GOPATH=/go
# Make sure GOROOT is unset
ENV GOROOT=
ENV PATH=${GOPATH}/bin:${PATH}

# Python package manager
COPY --from=uv /uv /uvx /usr/local/bin/
