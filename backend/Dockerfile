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

# uv (Python package manager) is copied from its official image instead of
# being downloaded by the install script at build time, so the version is
# pinned and Dependabot keeps it current through this FROM line.
FROM ghcr.io/astral-sh/uv:0.13.7 as uv

FROM --platform=$BUILDPLATFORM golang:1.27-trixie as builder

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


FROM python:3.14-slim-trixie as base

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

# trixie's login.defs sets HOME_MODE 0700; keep /app traversable for a random UID.
RUN useradd -ms /bin/bash -d /app -K HOME_MODE=0755 devlake -u 1010 -G 0
RUN chown -R devlake:devlake /etc/ssl/certs
USER devlake
ENV PYTHONUNBUFFERED=1

WORKDIR /app

RUN mkdir logs && chgrp 0 logs && chmod g=u logs
VOLUME /app/logs

# Ownership for arbitrary-UID runtimes (OpenShift runs containers with a random
# UID and GID 0) is applied in the same RUN that creates each path. Doing it
# there costs nothing: the layer was being written anyway. A recursive
# `chgrp -R 0 /app && chmod -R g=u /app` at the end of the image instead
# rewrites every file that earlier layers created, so Docker stores a second
# copy of the whole tree - around 190 MB on the published image.
#
# The devlake user is added to group 0 so it is allowed to chgrp the files it
# owns; its primary group stays devlake, so the runtime GID does not change.

# Setup Python
COPY --chown=devlake:0 python/ /app/python/
RUN python3 -m pip install --no-cache --upgrade pip setuptools && \
    python3 -m pip install --no-cache -r python/requirements.txt && \
    python3 -m pip install --upgrade pip && \
    chgrp -R 0 /app/.local && chmod -R g=u /app/.local
# Copied after the chgrp above so that it does not rewrite the binaries in a
# second layer; they are already world-executable.
COPY --from=uv --chown=devlake:0 /uv /uvx /app/.local/bin/
ENV PATH="/app/.local/bin:${PATH}"

# Build Python plugins, make sure the scripts has execute permission
# RUN find /app/python/ -name "*.sh" | xargs -I{} chmod +x {}
RUN /app/python/build.sh && \
    chgrp -R 0 /app/python && chmod -R g=u /app/python

FROM base as devlake-base
ARG DEBUG=

# apps
# The single binary already contains every plugin; the .so files that
# `make build-plugin` produces stay in the builder stage and are never copied.
# The binary and resources are read-only at runtime, so root:0 with the modes
# they already have (755 for the binary, 644 for the swagger docs) is enough for
# a random UID in group 0. No recursive chmod afterwards.
COPY --from=build --chown=0:0 /app/bin/lake /app/bin/lake
COPY --from=build --chown=0:0 /app/resources /app/resources

ENV PATH="/app/bin:${PATH}"
ENV DEBUG="$DEBUG"

USER root
# Empty mount point for sideloaded plugins: dropping a .so here registers it on
# top of the compiled-in set at startup; see PLUGIN_DIR.
RUN mkdir -p -m 0775 /app/bin/plugins
USER devlake

#add tini, prevent zombie process
ENTRYPOINT ["/usr/bin/tini", "--"]

CMD ["lake"]
