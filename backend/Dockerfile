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
#Apache DevLake is an effort undergoing incubation at The Apache Software
#Foundation (ASF), sponsored by the Apache Incubator PMC.
#
#Incubation is required of all newly accepted projects until a further review
#indicates that the infrastructure, communications, and decision making process
#have stabilized in a manner consistent with other successful ASF projects.
#
#While incubation status is not necessarily a reflection of the completeness or stability of the code,
#it does indicate that the project has yet to be fully endorsed by the ASF.

FROM --platform=linux/amd64 debian:bookworm as debian-amd64
RUN apt-get update
RUN apt-get install -y libssh2-1-dev libssl-dev zlib1g-dev

FROM --platform=linux/arm64 debian:bookworm as debian-arm64
RUN apt-get update
RUN apt-get install -y libssh2-1-dev libssl-dev zlib1g-dev

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm as builder

# docker build --build-arg GOPROXY=https://goproxy.io,direct -t mericodev/lake .
ARG GOPROXY=
# docker build --build-arg HTTPS_PROXY=http://localhost:4780 -t mericodev/lake .
ARG HTTP_PROXY=
ARG HTTPS_PROXY=

RUN apt-get update
RUN apt-get install -y gcc binutils libfindbin-libs-perl cmake libssh2-1-dev libssl-dev zlib1g-dev

RUN if [ "$(arch)" != "aarch64" ] ; then \
        apt-get install -y gcc-aarch64-linux-gnu binutils-aarch64-linux-gnu ; \
    fi
RUN if [ "$(arch)" != "x86_64" ] ; then \
        apt-get install -y gcc-x86-64-linux-gnu binutils-x86-64-linux-gnu ; \
    fi

RUN go install github.com/vektra/mockery/v2@v2.53.6
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.1

COPY --from=debian-amd64 /usr/include /rootfs-amd64/usr/include
COPY --from=debian-amd64 /usr/lib/x86_64-linux-gnu /rootfs-amd64/usr/lib/x86_64-linux-gnu
COPY --from=debian-amd64 /lib/x86_64-linux-gnu /rootfs-amd64/lib/x86_64-linux-gnu

COPY --from=debian-arm64 /usr/include /rootfs-arm64/usr/include
COPY --from=debian-arm64 /usr/lib/aarch64-linux-gnu /rootfs-arm64/usr/lib/aarch64-linux-gnu
COPY --from=debian-arm64 /lib/aarch64-linux-gnu /rootfs-arm64/lib/aarch64-linux-gnu


RUN for arch in aarch64 x86_64 ; do \
        mkdir -p /tmp/build/${arch} && cd /tmp/build/${arch} && \
        wget https://github.com/libgit2/libgit2/archive/refs/tags/v1.3.0.tar.gz -O - | tar -xz && \
        cd libgit2-1.3.0 && \
        mkdir build && cd build && \
        if [ "$arch" = "aarch64" ] ; then \
            cmake .. -DCMAKE_C_COMPILER=aarch64-linux-gnu-gcc \
                -DBUILD_SHARED_LIBS=ON -DCMAKE_SYSROOT=/rootfs-arm64 \
                -DCMAKE_INSTALL_PREFIX=/usr/local/deps/${arch} ; \
        elif [ "$arch" = "x86_64" ] ; then \
            cmake .. -DCMAKE_C_COMPILER=x86_64-linux-gnu-gcc \
                -DBUILD_SHARED_LIBS=ON -DCMAKE_SYSROOT=/rootfs-amd64 \
                -DCMAKE_INSTALL_PREFIX=/usr/local/deps/${arch} ; \
        fi && \
        make -j install ; \
    done


FROM builder as build

WORKDIR /app
COPY . /app
ENV GOBIN=/app/bin

ARG TARGETPLATFORM
ARG TAG=
ARG SHA=
ARG GO_PLUGINS=

RUN --mount=type=cache,target=/root/.cache/go-build \
    if [ "$TARGETPLATFORM" = "linux/arm64" ] ; then \
        ln -s /usr/local/deps/aarch64 /usr/local/deps/target && \
        export CC=aarch64-linux-gnu-gcc && \
        export GOARCH=arm64 ; \
    else \
        ln -s /usr/local/deps/x86_64 /usr/local/deps/target && \
        export CC=x86_64-linux-gnu-gcc && \
        export GOARCH=amd64 ; \
    fi && \
    export PKG_CONFIG_PATH=/usr/local/deps/target/lib/pkgconfig && \
    export CGO_ENABLED=1 &&\
    DEVLAKE_PLUGINS="$GO_PLUGINS" make all

# remove symlink in lib, we will recreate in final image
RUN cd /usr/local/deps/target/lib && \
    for file in *.so* ; do \
        if [ -L $file ] ; then \
            unlink $file ; \
        fi \
    done


FROM python:3.9-slim-bookworm as base

RUN apt-get update && \
    apt-get install -y python3-dev python3-pip tar pkg-config curl libssh2-1 zlib1g libffi-dev default-libmysqlclient-dev libpq-dev tini git openssh-client corkscrew && \
    apt-get clean && \
    rm -fr /usr/share/doc/* \
           /usr/share/info/* \
           /usr/share/linda/* \
           /usr/share/lintian/overrides/* \
           /usr/share/locale/* \
           /usr/share/man/* \
           /usr/share/doc/kde/HTML/* \
           /usr/share/gnome/help/* \
           /usr/share/locale/* \
           /usr/share/omf/*/*-*.emf \
           /var/lib/apt/lists/*

EXPOSE 8080

RUN useradd -ms /bin/bash -d /app devlake -u 1010
RUN chown -R devlake:devlake /etc/ssl/certs
USER devlake
ENV PYTHONUNBUFFERED=1

WORKDIR /app

RUN mkdir logs
VOLUME /app/logs

# Setup Python
COPY --chown=devlake:devlake python/ /app/python/
RUN python3 -m pip install --no-cache --upgrade pip setuptools && \
    python3 -m pip install --no-cache -r python/requirements.txt && \
    python3 -m pip install --upgrade pip

# Setup Python package manager
RUN curl -LsSf https://astral.sh/uv/install.sh | env UV_UNMANAGED_INSTALL=/app/.local/bin sh
ENV PATH="/app/.local/bin:${PATH}"

# Build Python plugins, make sure the scripts has execute permission
# RUN find /app/python/ -name "*.sh" | xargs -I{} chmod +x {}
RUN /app/python/build.sh

FROM base as devlake-base
ARG DEBUG=

# libraries
ENV LD_LIBRARY_PATH=/app/libs
RUN mkdir -p /app/libs
COPY --from=build /usr/local/deps/target/lib/*.so* /app/libs
COPY --from=build /go/bin /usr/bin
RUN ldconfig -vn /app/libs

# apps
COPY --from=build /app/bin /app/bin
COPY --from=build /app/resources /app/resources

ENV PATH="/app/bin:${PATH}"
ENV DEBUG="$DEBUG"

USER root
RUN chgrp -R 0 /app && \
    chmod -R g=u /app
USER devlake

#add tini, prevent zombie process
ENTRYPOINT ["/usr/bin/tini", "--"]

CMD ["lake"]
