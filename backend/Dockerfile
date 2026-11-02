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

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm as builder

# docker build --build-arg GOPROXY=https://goproxy.io,direct -t mericodev/lake .
ARG GOPROXY=
# docker build --build-arg HTTPS_PROXY=http://localhost:4780 -t mericodev/lake .
ARG HTTP_PROXY=
ARG HTTPS_PROXY=

RUN apt-get update
RUN apt-get install -y ca-certificates git

RUN go install github.com/vektra/mockery/v3@v3.9.0
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6


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
        export GOARCH=arm64 ; \
    else \
        export GOARCH=amd64 ; \
    fi && \
    export CGO_ENABLED=0 && \
    DEVLAKE_PLUGINS="$GO_PLUGINS" make all


FROM python:3.11-slim-bookworm as base

# apt-get upgrade picks up Debian security fixes that the base tag does not contain yet
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y python3-dev python3-pip tar pkg-config curl zlib1g libffi-dev default-libmysqlclient-dev libpq-dev tini git openssh-client corkscrew && \
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

# apps
# The single binary already contains every plugin; the .so files that
# `make build-plugin` produces stay in the builder stage and are never copied.
COPY --from=build /app/bin/lake /app/bin/lake
COPY --from=build /app/resources /app/resources

ENV PATH="/app/bin:${PATH}"
ENV DEBUG="$DEBUG"

USER root
# Empty mount point for sideloaded plugins: dropping a .so here registers it on
# top of the compiled-in set at startup; see PLUGIN_DIR.
RUN mkdir -p /app/bin/plugins && \
    chgrp -R 0 /app && \
    chmod -R g=u /app
USER devlake

#add tini, prevent zombie process
ENTRYPOINT ["/usr/bin/tini", "--"]

CMD ["lake"]
