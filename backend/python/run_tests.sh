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

for test_dir in $(find "$SCRIPT_DIR" -path '*/.venv' -prune -o -type f -name "*_test.py" -print | xargs dirname | sort -u); do
  project_dir=$(dirname "$test_dir")
  printf "Running Python tests in $test_dir\n"
  sh "$SCRIPT_DIR/uv.sh" sync "$project_dir"
  sh "$SCRIPT_DIR/uv.sh" pytest "$project_dir" "$test_dir"
  if [ $? != 0 ]; then
    exit 1
  fi
done
