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

set -e

echo build-server.sh VERSION: $VERSION

ROOT_DIR=$(dirname $(dirname "$0"))
VERSION=${VERSION:-$(git describe --tags --always --dirty || true)}
EXTRA=""
# Strip the symbol table and DWARF from release builds: they roughly halve the
# binary and nothing needs them at runtime. A debug build keeps them for delve.
LDFLAGS="-s -w"

if [ -n "$DEVLAKE_DEBUG" ]; then
    EXTRA="-gcflags='all=-N -l'"
    LDFLAGS=""
fi

go build $EXTRA -trimpath -ldflags "$LDFLAGS -X 'github.com/apache/devlake/core/version.Version=$VERSION'" -o $ROOT_DIR/bin/lake $ROOT_DIR/server/