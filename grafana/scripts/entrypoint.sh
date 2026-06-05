#!/bin/bash
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

DATASOURCE_FILE="/etc/grafana/provisioning/datasources/datasource.yml"

# Detect database type
# Priority: DATABASE_TYPE, then legacy MYSQL_*/POSTGRES_* auto-detection
if [ -n "$DATABASE_TYPE" ]; then
  MODE="$DATABASE_TYPE"

  case "$MODE" in
    mysql)
      export MYSQL_URL="${DATABASE_HOST:-mysql}:${DATABASE_PORT:-3306}"
      export MYSQL_DATABASE="${DATABASE_NAME:-lake}"
      export MYSQL_USER="${DATABASE_USER:-merico}"
      export MYSQL_PASSWORD="${DATABASE_PASSWORD:-merico}"
      ;;
    postgresql)
      export POSTGRES_URL="${DATABASE_HOST:-postgres}:${DATABASE_PORT:-5432}"
      export POSTGRES_DATABASE="${DATABASE_NAME:-lake}"
      export POSTGRES_USER="${DATABASE_USER:-merico}"
      export POSTGRES_PASSWORD="${DATABASE_PASSWORD:-merico}"
      ;;
    *)
      echo "ERROR: DATABASE_TYPE must be 'mysql' or 'postgresql'"
      exit 1
      ;;
  esac
else
  # Legacy: auto-detect from MYSQL_*/POSTGRES_* vars
  if [ -n "$POSTGRES_URL" ]; then
    MODE="postgresql"
  elif [ -n "$MYSQL_URL" ]; then
    MODE="mysql"
  else
    echo "WARNING: No database vars. Defaulting to mysql."
    MODE="mysql"
    export MYSQL_URL="mysql:3306"
    export MYSQL_DATABASE="lake"
    export MYSQL_USER="merico"
    export MYSQL_PASSWORD="merico"
  fi
fi

echo "Database type: $MODE"

# Remove unused dashboard folder to prevent confusion
if [ "$MODE" = "mysql" ]; then
  rm -rf /etc/grafana/dashboards/postgresql
  export GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH="/etc/grafana/dashboards/mysql/Homepage.json"
else
  rm -rf /etc/grafana/dashboards/mysql
  SSL_MODE="${DATABASE_SSL_MODE:-disable}"
  export GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH="/etc/grafana/dashboards/postgresql/Homepage.json"
  echo "SSL Mode: ${SSL_MODE}"
fi

echo "Homepage: $GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH"

# Create empty datasource.yml (datasources created via API)
cat > "$DATASOURCE_FILE" << HEADER
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
HEADER

# Start Grafana in background
/run.sh "$@" &
GRAFANA_PID=$!

# Wait for Grafana API
echo "Waiting for Grafana API..."
for i in $(seq 1 60); do
  if curl -f -s http://localhost:3000/api/health >/dev/null 2>&1; then
    echo "Grafana API ready"
    sleep 5  # Extra wait for migrations
    break
  fi
  sleep 2
done

# Create datasource via API (both MySQL and PostgreSQL)
PAYLOAD_FILE="/tmp/datasource-api.json"

if [ "$MODE" = "mysql" ]; then
  cat > "$PAYLOAD_FILE" <<APIJSON
{
  "uid": "devlake-mysql-api",
  "name": "mysql",
  "type": "mysql",
  "url": "${MYSQL_URL}",
  "database": "${MYSQL_DATABASE}",
  "user": "${MYSQL_USER}",
  "secureJsonData": {
    "password": "${MYSQL_PASSWORD}"
  },
  "access": "proxy",
  "isDefault": true,
  "editable": true
}
APIJSON

  echo "Deleting old MySQL datasources..."
  curl -s -X DELETE "http://admin:${GF_SECURITY_ADMIN_PASSWORD}@localhost:3000/api/datasources/name/mysql" 2>&1 || true
  curl -s -X DELETE "http://admin:${GF_SECURITY_ADMIN_PASSWORD}@localhost:3000/api/datasources/uid/devlake-mysql-api" 2>&1 || true
  sleep 2

else
  SSL_MODE="${DATABASE_SSL_MODE:-disable}"
  cat > "$PAYLOAD_FILE" <<APIJSON
{
  "uid": "devlake-postgres-api",
  "name": "postgresql",
  "type": "postgres",
  "url": "${POSTGRES_URL}",
  "database": "${POSTGRES_DATABASE}",
  "user": "${POSTGRES_USER}",
  "secureJsonData": {
    "password": "${POSTGRES_PASSWORD}"
  },
  "jsonData": {
    "sslmode": "${SSL_MODE}",
    "postgresVersion": 1400
  },
  "access": "proxy",
  "isDefault": true,
  "editable": true
}
APIJSON

fi

echo "Creating datasource via API..."
for i in $(seq 1 10); do
  RESPONSE=$(curl -s -X POST "http://admin:${GF_SECURITY_ADMIN_PASSWORD}@localhost:3000/api/datasources" \
    -H "Content-Type: application/json" \
    -d @"$PAYLOAD_FILE" 2>&1)

  if echo "$RESPONSE" | grep -q '"id"'; then
    echo "Datasource created successfully"
    break
  elif echo "$RESPONSE" | grep -q "database is locked"; then
    echo "DB locked, retry $i/10..."
    sleep 3
  else
    echo "API error: $RESPONSE"
    sleep 2
  fi
done

# Wait for Grafana
wait $GRAFANA_PID
