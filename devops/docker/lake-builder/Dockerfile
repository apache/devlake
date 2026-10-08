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

# Debian 11 (bullseye) reached the end of its LTS period on 2026-08-31 and its
# security suite is no longer served, so `apt upgrade` fails there. All stages
# use Debian 12 (bookworm), matching the runtime image in backend/Dockerfile.
FROM --platform=linux/amd64 debian:bookworm AS debian-amd64
RUN apt-get -y update && apt -y upgrade &&\
    apt-get install -y libssh2-1-dev libssl-dev zlib1g-dev

# This stage only cross-builds libgit2, it does not need a Go toolchain.
FROM --platform=linux/amd64 debian:bookworm AS builder

# Base dependencies
RUN apt-get -y update && apt -y upgrade &&\
    apt-get install -y ca-certificates wget gcc binutils libfindbin-libs-perl cmake libssh2-1-dev libssl-dev zlib1g-dev \
    gcc-x86-64-linux-gnu binutils-x86-64-linux-gnu

COPY --from=debian-amd64 /usr/include /rootfs-amd64/usr/include
COPY --from=debian-amd64 /usr/lib/x86_64-linux-gnu /rootfs-amd64/usr/lib/x86_64-linux-gnu
COPY --from=debian-amd64 /lib/x86_64-linux-gnu /rootfs-amd64/lib/x86_64-linux-gnu

## Build libs/headers needed for gitextractor plugin
RUN \
    mkdir -p /tmp/build/x86_64 && cd /tmp/build/x86_64 && \
    wget https://github.com/libgit2/libgit2/archive/refs/tags/v1.5.0.tar.gz -O - | tar -xz && \
    cd libgit2-1.5.0 && \
    mkdir build && cd build && \
    cmake .. -DCMAKE_C_COMPILER=x86_64-linux-gnu-gcc -DBUILD_SHARED_LIBS=ON -DBUILD_TESTS=OFF -DCMAKE_SYSROOT=/rootfs-amd64 -DCMAKE_INSTALL_PREFIX=/usr/local/deps/x86_64 &&\
    make -j install &&\
    mkdir -p /tmp/deps &&\
    cp *libgit2* /tmp/deps/ &&\
    cp -r ../include /tmp/deps/include

# Same Python line as the runtime image in backend/Dockerfile.
FROM python:3.11-slim-bookworm

RUN apt -y update && apt -y upgrade && apt -y install tzdata make tar curl gcc g++ pkg-config git \
    libssh2-1 zlib1g libffi-dev  \
    default-libmysqlclient-dev \
    libpq-dev

# Install Libs/Headers from previous stage
COPY --from=builder /tmp/deps/*.so* /usr/lib/
COPY --from=builder /tmp/deps/*.pc /usr/lib/x86_64-linux-gnu/pkgconfig/
COPY --from=builder /tmp/deps/include/ /usr/include/
ENV PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig

# Install Golang
# The previous installer was fetched from `git.io`, which GitHub shut down in
# 2022, so the image could no longer be rebuilt. Fetch the official archive
# from go.dev and verify it. Version and checksum match
# backend/scripts/install-go.sh.
ARG GO_VERSION=1.26.6
ARG GO_SHA256=708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89
RUN curl --fail --location --retry 3 "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" --output /tmp/go.tar.gz &&\
    echo "${GO_SHA256}  /tmp/go.tar.gz" | sha256sum --check --strict &&\
    tar -C /usr/local -xzf /tmp/go.tar.gz && rm /tmp/go.tar.gz &&\
    mkdir -p /go &&\
    ln -sf /usr/local/go/bin/* /usr/bin

# Install Golang Tools (versions match backend/Dockerfile and backend/Makefile)
RUN export GOPATH=/go &&  \
    go install github.com/vektra/mockery/v3@v3.7.4 &&  \
    go install github.com/swaggo/swag/cmd/swag@v1.16.6

# Golang Env
ENV GOPATH=/go
# Make sure GOROOT is unset
ENV GOROOT=
ENV PATH=${GOPATH}/bin:${PATH}

# Python package manager
RUN curl -LsSf https://astral.sh/uv/install.sh | env UV_UNMANAGED_INSTALL=/usr/local/bin sh
