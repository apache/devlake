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

set -eu

resolve_path() {
  CDPATH= cd -- "$1" && pwd
}

ensure_uv() {
  if command -v uv >/dev/null 2>&1; then
    return 0
  fi

  uv_install_dir=${DEVLAKE_UV_INSTALL_DIR:-${HOME:-$(pwd)}/.local/bin}
  if [ -x "$uv_install_dir/uv" ]; then
    PATH="$uv_install_dir:$PATH"
    export PATH
    return 0
  fi
  mkdir -p "$uv_install_dir"
  curl -LsSf https://astral.sh/uv/install.sh | env UV_UNMANAGED_INSTALL="$uv_install_dir" sh
  PATH="$uv_install_dir:$PATH"
  export PATH
}

sync_project() {
  project_dir=$(resolve_path "$1")
  ensure_uv
  cd "$project_dir"
  if [ -d .venv ] && [ ! -x .venv/bin/python ]; then
    rm -rf .venv
  fi
  if [ -x .venv/bin/python ] && ! .venv/bin/python -c "import sys" >/dev/null 2>&1; then
    rm -rf .venv
  fi
  if [ ! -x .venv/bin/python ]; then
    uv venv --python "${DEVLAKE_PYTHON_VERSION:-3.9}" .venv
  fi
  uv pip install --python .venv/bin/python -e .
}

ensure_project_python() {
  project_dir=$(resolve_path "$1")
  if [ ! -x "$project_dir/.venv/bin/python" ]; then
    sync_project "$project_dir"
  else
    ensure_uv
  fi
  printf '%s/.venv/bin/python\n' "$project_dir"
}

run_python() {
  project_dir=$(resolve_path "$1")
  shift
  python_bin=$(ensure_project_python "$project_dir")
  exec "$python_bin" "$@"
}

run_pytest() {
  project_dir=$(resolve_path "$1")
  shift
  ensure_uv
  python_bin=$(ensure_project_python "$project_dir")
  uv pip install --python "$python_bin" pytest
  exec "$python_bin" -m pytest "$@"
}

usage() {
  echo "Usage: $0 {sync|python|pytest} <project-dir> [args...]" >&2
  exit 1
}

command_name=${1:-}
[ -n "$command_name" ] || usage
shift

case "$command_name" in
  sync)
    [ $# -eq 1 ] || usage
    sync_project "$1"
    ;;
  python)
    [ $# -ge 2 ] || usage
    project_dir=$1
    shift
    run_python "$project_dir" "$@"
    ;;
  pytest)
    [ $# -ge 1 ] || usage
    project_dir=$1
    shift
    run_pytest "$project_dir" "$@"
    ;;
  *)
    usage
    ;;
esac
