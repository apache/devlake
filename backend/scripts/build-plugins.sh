#!/bin/sh
#
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

# compile all plugins and fire up api server:
#   make dev
#
# compile specific plugin and fire up api server:
#   PLUGIN=<PLUGIN_NAME[,PLUGIN_NAME2]> make dev
#
# compile all plugins and fire up api server in DEBUG MODE with `delve`:
#   make debug
#
# compile specific plugin and fire up api server in DEBUG MODE with `delve`:
#   PLUGIN=<PLUGIN_NAME[,PLUGIN_NAME2]> make debug

set -e

echo "Usage: "
echo "  build all plugins:              $0 [golang build flags...]"
echo "  build and keep specified plugins only: DEVLAKE_PLUGINS=github,jira $0 [golang build flags...]"

ROOT_DIR=$(dirname $(dirname "$0"))
EXTRA=""
PLUGIN_SRC_DIR=$ROOT_DIR/plugins
PLUGIN_OUTPUT_DIR=${PLUGIN_DIR:-$ROOT_DIR/bin/plugins}


rm -rf $PLUGIN_OUTPUT_DIR/*
if [ "$DEVLAKE_PLUGINS" = "none" ]; then
    echo "skip building plugins" >&2
    exit 0
fi

if [ -n "$DEVLAKE_DEBUG" ]; then
    GCFLAGS=all=-N\ -l
fi

if [ -z "$DEVLAKE_PLUGINS" ]; then
    echo "Building all plugins"
    # schema_e2e is not a plugin, it only holds the cross-plugin schema-drift e2e test
    PLUGINS=$(find $PLUGIN_SRC_DIR/* -maxdepth 0 -type d -not -name core -not -name helper -not -name logs -not -name schema_e2e -not -empty)
else
    echo "Building the following plugins: $PLUGIN"
    PLUGINS=
    for p in $(echo "$DEVLAKE_PLUGINS" | tr "," "\n"); do
        PLUGINS="$PLUGINS $PLUGIN_SRC_DIR/$p"
    done
fi



PIDS=""
# Cap concurrent CGO plugin builds. Unbounded nproc OOMs Docker Desktop / small VMs.
# Override with DEVLAKE_PLUGIN_PARALLELISM=N (default: min(4, nproc)).
if [ -n "$DEVLAKE_PLUGIN_PARALLELISM" ]; then
    PARALLELISM="$DEVLAKE_PLUGIN_PARALLELISM"
else
    PARALLELISM=4
    if command -v nproc >/dev/null 2>&1; then
        PARALLELISM=$(nproc)
    elif command -v sysctl >/dev/null 2>&1; then
        PARALLELISM=$(sysctl -n hw.ncpu)
    fi
    if [ "$PARALLELISM" -gt 4 ]; then
        PARALLELISM=4
    fi
fi
echo "Plugin build parallelism: $PARALLELISM"
for PLUG in $PLUGINS; do
    NAME=$(basename $PLUG)
    ROOT_GO_FILES=$(find "$PLUG" -maxdepth 1 -name '*.go' -print)
    if [ -z "$ROOT_GO_FILES" ]; then
        echo "Skipping plugin $NAME: no root-level *.go entrypoint in $PLUG" >&2
        continue
    fi
    echo "Building plugin $NAME to bin/plugins/$NAME/$NAME.so with args: $*  --gcflags="$GCFLAGS""
    go build -buildmode=plugin --gcflags="$GCFLAGS" -o $PLUGIN_OUTPUT_DIR/$NAME/$NAME.so $PLUG/*.go &
    PIDS="$PIDS $!"
    # avoid too many processes causing signal killed
    COUNT=$(echo "$PIDS" | wc -w)
    if [ "$COUNT" -ge "$PARALLELISM" ]; then
        for PID in $PIDS; do
            wait $PID
        done
        PIDS=""
    fi
done

for PID in $PIDS; do
    wait $PID
done