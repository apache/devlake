#!/usr/bin/env bash
# Licensed to the Apache Software Foundation (ASF) under one or more
# contributor license agreements. See the NOTICE file distributed with
# this work for additional information regarding copyright ownership.
# The ASF licenses this file to You under the Apache License, Version 2.0
# (the "License"); you may not use this file except in compliance with
# the License. You may obtain a copy of the License at
# http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Run from any directory: bash backend/test/graphql-resume-crash.sh
# Requires Docker; generates missing mocks with the pinned tool version.
# Uses only newly created containers; no existing DB or published port.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
artifacts=$(mktemp -d "${TMPDIR:-/tmp}/devlake-graphql-crash.XXXXXX")
prefix="devlake-graphql-crash-$$"
driver=${DEVLAKE_CRASH_DRIVER:-mysql}
mysql_name="$prefix-$driver"
containers=()
cleanup() {
 for name in "${containers[@]}"; do docker rm -f "$name" >/dev/null 2>&1 || true; done
 echo "Logs and binaries: $artifacts"
}
trap cleanup EXIT
if [[ ! -f "$repo/backend/mocks/core/dal/Dal.go" ]]; then
 for config in .mockery.core.yml .mockery.helpers.yml; do
  docker run --rm -v "$repo:/src" -v devlake-9142-go-cache:/go \
   -v devlake-9142-build-cache:/root/.cache/go-build -w /src/backend golang:1.26 \
   go run github.com/vektra/mockery/v3@v3.7.4 --config "$config"
 done
fi
build() {
 docker run --rm -v "$repo:/src:ro" -v "$artifacts:/out" \
  -v devlake-9142-go-cache:/go -v devlake-9142-build-cache:/root/.cache/go-build \
  -w /src/backend golang:1.26 go test -race -c -o "/out/$1.test" "$2"
}
build collector ./helpers/pluginhelper/api
build server ./server/api
containers+=("$mysql_name")
if [[ "$driver" == postgres ]]; then
 docker run -d --name "$mysql_name" --tmpfs /var/lib/postgresql \
  -e POSTGRES_PASSWORD=crash-test-only -e POSTGRES_DB=crash_test postgres:18.4 >/dev/null
elif [[ "$driver" == mysql ]]; then
 docker run -d --name "$mysql_name" --tmpfs /var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD=crash-test-only -e MYSQL_DATABASE=crash_test mysql:8.4.11 >/dev/null
else
 echo "Unsupported driver: $driver"; exit 1
fi
sql() {
 if [[ "$driver" == postgres ]]; then docker exec "$mysql_name" psql -U postgres -d crash_test -c "$1";
 else docker exec -e MYSQL_PWD=crash-test-only "$mysql_name" mysql -uroot -e "$1"; fi
}
ready=0
for ((attempt=0; attempt<120; attempt++)); do
 if sql 'SELECT 1' >/dev/null 2>&1; then ready=1; break; fi
 sleep 1
done
if [[ "$ready" != 1 ]]; then docker logs "$mysql_name"; exit 1; fi
set_database() {
 local database="$1"
 common=(--network "container:$mysql_name" -v "$artifacts:/out:ro" -e "DEVLAKE_CRASH_DRIVER=$driver")
 if [[ "$driver" == postgres ]]; then
  common+=(-e "DEVLAKE_CRASH_DSN=host=127.0.0.1 user=postgres password=crash-test-only dbname=$database port=5432 sslmode=disable"
   -e "DEVLAKE_SERVER_CRASH_DB_URL=postgres://postgres:crash-test-only@127.0.0.1:5432/$database?sslmode=disable")
 else
  common+=(-e "DEVLAKE_CRASH_DSN=root:crash-test-only@tcp(127.0.0.1:3306)/$database?parseTime=true"
   -e "DEVLAKE_SERVER_CRASH_DB_URL=mysql://root:crash-test-only@127.0.0.1:3306/$database?parseTime=true")
 fi
}
set_database crash_test
crash() {
 local name="$1" binary="$2" test_name="$3" scenario="$4" phase="${5:-crash}"
 containers+=("$name")
 docker run -d --name "$name" "${common[@]}" -e "DEVLAKE_CRASH_PHASE=$phase" \
  -e "DEVLAKE_CRASH_SCENARIO=$scenario" golang:1.26 "/out/$binary.test" \
  -test.run "^$test_name$" -test.v -test.timeout 3m >/dev/null
 local ready=0
 for ((attempt=0; attempt<600; attempt++)); do
  docker logs "$name" > "$artifacts/$scenario-crash.log" 2>&1
  if grep -q CRASH_READY "$artifacts/$scenario-crash.log"; then ready=1; break; fi
  if [[ $(docker inspect -f '{{.State.Running}}' "$name") != true ]]; then break; fi
  sleep 0.2
 done
 if [[ "$ready" != 1 ]]; then cat "$artifacts/$scenario-crash.log"; return 1; fi
 docker kill --signal KILL "$name" >/dev/null
 docker logs "$name" > "$artifacts/$scenario-crash.log" 2>&1
 if grep -q 'WARNING: DATA RACE' "$artifacts/$scenario-crash.log"; then
  cat "$artifacts/$scenario-crash.log"; return 1
 fi
 docker inspect -f '{{.State.ExitCode}} {{.State.OOMKilled}}' "$name" > "$artifacts/$scenario-exit.log"
 grep -qx '137 false' "$artifacts/$scenario-exit.log"
}
for scenario in single multi transaction batch incremental initialize final-page complete; do
 crash "$prefix-$scenario" collector TestGraphqlCollectorMySQLCrash "$scenario"
 for phase in resume repeat fresh; do
  docker run --rm "${common[@]}" -e "DEVLAKE_CRASH_PHASE=$phase" \
   -e "DEVLAKE_CRASH_SCENARIO=$scenario" golang:1.26 /out/collector.test \
   -test.run '^TestGraphqlCollectorMySQLCrash$' -test.v -test.timeout 1m \
   > "$artifacts/$scenario-$phase.log" 2>&1 || { cat "$artifacts/$scenario-$phase.log"; exit 1; }
  echo "PASS $scenario/$phase"
 done
done
for test_name in TestGraphqlCollectorMySQLLifecycleRecovery TestGraphqlCollectorMySQLScopeIsolation TestGraphqlDatabaseGuard TestGraphqlDatabasePageRollback TestGraphqlDatabaseNestedResume TestGraphqlDatabaseTasklessAndEmpty; do
 docker run --rm "${common[@]}" golang:1.26 /out/collector.test \
  -test.run "^$test_name$" -test.v -test.timeout 1m \
  > "$artifacts/$test_name.log" 2>&1 || { cat "$artifacts/$test_name.log"; exit 1; }
 echo "PASS $test_name"
done
crash "$prefix-server" server TestGraphqlServerResumeCrash server
docker run --rm "${common[@]}" -e DEVLAKE_CRASH_PHASE=resume golang:1.26 \
 /out/server.test -test.run '^TestGraphqlServerResumeCrash$' -test.v -test.timeout 2m \
 > "$artifacts/server-resume.log" 2>&1 || { cat "$artifacts/server-resume.log"; exit 1; }
echo "PASS real server RESUME_PIPELINES"
sql "CREATE DATABASE server_rerun" >/dev/null
set_database server_rerun
common+=(-e DEVLAKE_SERVER_RERUN=true)
crash "$prefix-rerun" server TestGraphqlServerResumeCrash server-rerun
docker run --rm "${common[@]}" -e DEVLAKE_CRASH_PHASE=resume golang:1.26 \
 /out/server.test -test.run '^TestGraphqlServerResumeCrash$' -test.v -test.timeout 2m \
 > "$artifacts/server-rerun.log" 2>&1 || { cat "$artifacts/server-rerun.log"; exit 1; }
echo "PASS RESUME_PIPELINES=false and rerun API"
sql "CREATE DATABASE other_collectors" >/dev/null
set_database other_collectors
docker run --rm "${common[@]}" golang:1.26 /out/server.test \
 -test.run '^TestGraphqlOtherCollectorsResume$' -test.v -test.timeout 2m \
 > "$artifacts/other-collectors.log" 2>&1 || { cat "$artifacts/other-collectors.log"; exit 1; }
echo "PASS real GitHub PR/Issue and Linear collectors"
for scenario in batch page extract; do
 sql "CREATE DATABASE jobs_$scenario" >/dev/null
 set_database "jobs_$scenario"
 common+=(-e "DEVLAKE_JOBS_SCENARIO=$scenario")
 crash "$prefix-jobs-$scenario" server TestGithubJobsServerResumeCrash "jobs-$scenario"
 if [[ "$scenario" == page ]]; then
  crash "$prefix-jobs-page-again" server TestGithubJobsServerResumeCrash jobs-page-again crash-again
 fi
 docker run --rm "${common[@]}" -e DEVLAKE_CRASH_PHASE=resume golang:1.26 \
  /out/server.test -test.run '^TestGithubJobsServerResumeCrash$' -test.v -test.timeout 2m \
  > "$artifacts/jobs-$scenario-resume.log" 2>&1 || { cat "$artifacts/jobs-$scenario-resume.log"; exit 1; }
 echo "PASS real CollectJobs/ExtractJobs $scenario"
done
