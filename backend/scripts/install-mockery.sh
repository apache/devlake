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

MOCKERY_VERSION=3.9.0
INSTALL_DIR=/opt/mockery/$MOCKERY_VERSION
MOCKERY_BIN=$INSTALL_DIR/mockery

get_version() {
    if version_output=$("$1" version 2>/dev/null); then
        :
    else
        version_output=$("$1" --version 2>&1 || true)
    fi
    printf '%s\n' "$version_output" |
        tr ' =' '\n' |
        sed -n 's/^v\{0,1\}\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)$/\1/p' |
        head -n 1
}

use_installed_mockery() {
    export PATH="$INSTALL_DIR:$PATH"
    if [ -n "${GITHUB_PATH:-}" ]; then
        echo "$INSTALL_DIR" >> "$GITHUB_PATH"
    fi
}

current_mockery=$(command -v mockery 2>/dev/null || true)
if [ -n "$current_mockery" ]; then
    current_version=$(get_version "$current_mockery" || true)
    if [ "$current_version" = "$MOCKERY_VERSION" ]; then
        echo "mockery $current_version is already installed"
        exit 0
    fi
fi

if [ -x "$MOCKERY_BIN" ]; then
    installed_version=$(get_version "$MOCKERY_BIN" || true)
    if [ "$installed_version" = "$MOCKERY_VERSION" ]; then
        use_installed_mockery
        echo "mockery $installed_version is already installed in $INSTALL_DIR"
        exit 0
    fi
fi

case "$(uname -m)" in
    x86_64|amd64)
        archive_name=mockery_${MOCKERY_VERSION}_Linux_x86_64.tar.gz
        archive_sha256=30ca380381763e7c9e8afddeea09c260061aa1580b885a66677bb12ebd9f038d
        ;;
    aarch64|arm64)
        archive_name=mockery_${MOCKERY_VERSION}_Linux_arm64.tar.gz
        archive_sha256=de4951716cc25236d0ee8dd5f18d1c3a08ab10315028e4313a7aee7526760d13
        ;;
    *)
        echo "unsupported architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

if [ "$(id -u)" -ne 0 ]; then
    echo "install-mockery.sh must run as root" >&2
    exit 1
fi

download_url=https://github.com/vektra/mockery/releases/download/v${MOCKERY_VERSION}/${archive_name}
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

archive=$work_dir/$archive_name
curl --fail --location --retry 3 "$download_url" --output "$archive"
echo "$archive_sha256  $archive" | sha256sum --check --strict

tar -xzf "$archive" -C "$work_dir"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$work_dir/mockery" "$MOCKERY_BIN"

use_installed_mockery
installed_version=$(get_version "$MOCKERY_BIN")
if [ "$installed_version" != "$MOCKERY_VERSION" ]; then
    echo "expected mockery $MOCKERY_VERSION, found ${installed_version:-unknown}" >&2
    exit 1
fi

echo "installed mockery $installed_version in $INSTALL_DIR"
