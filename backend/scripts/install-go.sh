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

set -eu

GO_VERSION=1.27.3
INSTALL_DIR=/opt/go/$GO_VERSION
GO_BIN=$INSTALL_DIR/bin/go

get_version() {
    "$1" version 2>/dev/null |
        sed -n 's/^go version go\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\) .*/\1/p'
}

use_installed_go() {
    export PATH="$INSTALL_DIR/bin:$PATH"
    if [ -n "${GITHUB_PATH:-}" ]; then
        echo "$INSTALL_DIR/bin" >> "$GITHUB_PATH"
    fi
}

current_go=$(command -v go 2>/dev/null || true)
if [ -n "$current_go" ]; then
    current_version=$(get_version "$current_go" || true)
    if [ "$current_version" = "$GO_VERSION" ]; then
        echo "Go $current_version is already installed"
        exit 0
    fi
fi

if [ -x "$GO_BIN" ]; then
    installed_version=$(get_version "$GO_BIN" || true)
    if [ "$installed_version" = "$GO_VERSION" ]; then
        use_installed_go
        echo "Go $installed_version is already installed in $INSTALL_DIR"
        exit 0
    fi
fi

case "$(uname -m)" in
    x86_64|amd64)
        archive_name=go${GO_VERSION}.linux-amd64.tar.gz
        archive_sha256=65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40
        ;;
    aarch64|arm64)
        archive_name=go${GO_VERSION}.linux-arm64.tar.gz
        archive_sha256=a06e99309dbcbdca1a8ccd2282c3ee146e55930886e26b4f1193269587165f49
        ;;
    *)
        echo "unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

if [ "$(id -u)" -ne 0 ]; then
    echo "install-go.sh must run as root" >&2
    exit 1
fi

download_url=https://go.dev/dl/$archive_name
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

archive=$work_dir/$archive_name
curl --fail --location --retry 3 "$download_url" --output "$archive"
echo "$archive_sha256  $archive" | sha256sum --check --strict

tar -xzf "$archive" -C "$work_dir"
mkdir -p "$(dirname "$INSTALL_DIR")"
rm -rf "$INSTALL_DIR"
mv "$work_dir/go" "$INSTALL_DIR"

use_installed_go
installed_version=$(get_version "$GO_BIN")
if [ "$installed_version" != "$GO_VERSION" ]; then
    echo "expected Go $GO_VERSION, found ${installed_version:-unknown}" >&2
    exit 1
fi

echo "installed Go $installed_version in $INSTALL_DIR"
