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
SCRIPT_DIR=$(CDPATH= cd -- "${0%/*}" && pwd)

if [ -n "${COVERAGE_OUTPUT_DIR:-}" ]; then
  mkdir -p "$COVERAGE_OUTPUT_DIR"
  COV_ABS_DIR=$(cd "$COVERAGE_OUTPUT_DIR" && pwd)
fi

cov_index=0
for test_dir in $(find "$SCRIPT_DIR" -path '*/.venv' -prune -o -type f -name "*_test.py" -print | xargs dirname | sort -u); do
  project_dir=$(dirname "$test_dir")
  printf "Running Python tests in $test_dir\n"
  sh "$SCRIPT_DIR/uv.sh" sync "$project_dir"

  if [ -n "${COV_ABS_DIR:-}" ]; then
    cov_index=$((cov_index + 1))
    python_bin="$project_dir/.venv/bin/python"
    "$python_bin" -m pip install pytest pytest-cov
    "$python_bin" -m pytest \
      --cov="$project_dir" \
      --cov-report="xml:${COV_ABS_DIR}/coverage-${cov_index}.xml" \
      "$test_dir"
  else
    sh "$SCRIPT_DIR/uv.sh" pytest "$project_dir" "$test_dir"
  fi

  if [ $? != 0 ]; then
    exit 1
  fi
done
