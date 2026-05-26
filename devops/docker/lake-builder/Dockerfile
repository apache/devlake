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

FROM --platform=linux/amd64 debian:bullseye as debian-amd64
RUN apt-get -y update && apt -y upgrade &&\
    apt-get install -y libssh2-1-dev libssl-dev zlib1g-dev

FROM golang:1.26-bullseye as builder

# Base dependencies
RUN apt-get -y update && apt -y upgrade &&\
    apt-get install -y gcc binutils libfindbin-libs-perl cmake libssh2-1-dev libssl-dev zlib1g-dev \
    gcc-x86-64-linux-gnu binutils-x86-64-linux-gnu

COPY --from=debian-amd64 /usr/include /rootfs-amd64/usr/include
COPY --from=debian-amd64 /usr/lib/x86_64-linux-gnu /rootfs-amd64/usr/lib/x86_64-linux-gnu
COPY --from=debian-amd64 /lib/x86_64-linux-gnu /rootfs-amd64/lib/x86_64-linux-gnu

## Build libs/headers needed for gitextractor plugin
RUN \
    mkdir -p /tmp/build/x86_64 && cd /tmp/build/x86_64 && \
    wget https://github.com/libgit2/libgit2/archive/refs/tags/v1.3.2.tar.gz -O - | tar -xz && \
    cd libgit2-1.3.2 && \
    mkdir build && cd build && \
    cmake .. -DCMAKE_C_COMPILER=x86_64-linux-gnu-gcc -DBUILD_SHARED_LIBS=ON -DCMAKE_SYSROOT=/rootfs-amd64 -DCMAKE_INSTALL_PREFIX=/usr/local/deps/x86_64 &&\
    make -j install &&\
    mkdir -p /tmp/deps &&\
    cp *libgit2* /tmp/deps/ &&\
    cp -r ../include /tmp/deps/include

FROM python:3.9-slim-bullseye

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
RUN curl -L https://git.io/vQhTU | bash -s -- --version 1.26.2
RUN mv /root/go /go &&\
    mv /root/.go /usr/local/go &&\
    ln -sf /usr/local/go/bin/* /usr/bin

# Install Golang Tools
RUN export GOPATH=/go &&  \
    go install github.com/vektra/mockery/v2@v2.53.6 &&  \
    go install github.com/swaggo/swag/cmd/swag@v1.16.1

# Golang Env
ENV GOPATH=/go
# Make sure GOROOT is unset
ENV GOROOT=
ENV PATH=${GOPATH}/bin:${PATH}

# Python package manager
RUN curl -LsSf https://astral.sh/uv/install.sh | env UV_UNMANAGED_INSTALL=/usr/local/bin sh
