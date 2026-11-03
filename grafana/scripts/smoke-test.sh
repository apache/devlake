#!/usr/bin/env bash
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

# Smoke test for the devlake-dashboard (Grafana) image: starts it next to a
# throwaway database and checks that the entrypoint runs, Grafana becomes
# healthy and the DevLake datasource is created and can reach the database.
#
# Usage: grafana/scripts/smoke-test.sh <image> [mysql|postgresql]
# Uses uniquely named containers/network only; nothing else is touched.

set -euo pipefail

IMAGE="${1:?usage: $0 <image> [mysql|postgresql]}"
DB_TYPE="${2:-mysql}"
TIMEOUT="${SMOKE_TIMEOUT:-180}"
MYSQL_IMAGE="${MYSQL_IMAGE:-mysql:8.4}"
POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:18}"

PREFIX="grafana-smoke-$$"
NET="$PREFIX-net"
DB="$PREFIX-db"
GRAFANA="$PREFIX-grafana"

cleanup() {
  docker rm -fv "$GRAFANA" "$DB" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*"
  echo "----- grafana container logs -----"
  docker logs "$GRAFANA" 2>&1 | grep -v 'logger=' | tail -50 || true
  exit 1
}

docker network create "$NET" >/dev/null

case "$DB_TYPE" in
  mysql)
    docker run -d -q --name "$DB" --network "$NET" \
      -e MYSQL_ROOT_PASSWORD=admin -e MYSQL_DATABASE=lake \
      -e MYSQL_USER=merico -e MYSQL_PASSWORD=merico \
      "$MYSQL_IMAGE" >/dev/null
    DS_UID="devlake-mysql-api"
    ;;
  postgresql)
    docker run -d -q --name "$DB" --network "$NET" \
      -e POSTGRES_DB=lake -e POSTGRES_USER=merico -e POSTGRES_PASSWORD=merico \
      "$POSTGRES_IMAGE" >/dev/null
    DS_UID="devlake-postgres-api"
    ;;
  *)
    echo "unknown database type '$DB_TYPE' (expected mysql or postgresql)"
    exit 2
    ;;
esac

docker run -d --name "$GRAFANA" --network "$NET" \
  -e DATABASE_TYPE="$DB_TYPE" -e DATABASE_HOST="$DB" \
  "$IMAGE" >/dev/null

echo "Waiting for $IMAGE ($DB_TYPE) to create its datasource..."
deadline=$((SECONDS + TIMEOUT))
until docker logs "$GRAFANA" 2>&1 | grep -q "Datasource created successfully"; do
  if [ "$(docker inspect -f '{{.State.Running}}' "$GRAFANA")" != "true" ]; then
    fail "grafana container exited (exit code $(docker inspect -f '{{.State.ExitCode}}' "$GRAFANA"))"
  fi
  if docker logs "$GRAFANA" 2>&1 | grep -q "ERROR: datasource"; then
    fail "entrypoint could not create the datasource"
  fi
  [ "$SECONDS" -lt "$deadline" ] || fail "datasource not created within ${TIMEOUT}s"
  sleep 3
done

api() {
  docker exec "$GRAFANA" curl -fsS --user admin:admin "http://localhost:3000/api/$1"
}

api health >/dev/null || fail "/api/health not OK"

# The database may still be initialising; retry the datasource health check.
until api "datasources/uid/$DS_UID/health" >/dev/null 2>&1; do
  [ "$SECONDS" -lt "$deadline" ] || fail "datasource $DS_UID cannot reach the $DB_TYPE database"
  sleep 3
done

echo "OK: $IMAGE starts and datasource $DS_UID is healthy ($DB_TYPE)"
