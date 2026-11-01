<!--
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->
### Directory structure explanation

* <i>e2e/</i>: Contains DevLake-server tests that interact with either fake plugins or no plugins at all.
* <i>integration/</i>: Contains DevLake-server tests written against real data-sources which contain test data.

### Running E2E tests locally

Start a MySQL instance and create a test database that the `merico` user can access. The database URL must use
`127.0.0.1` instead of `localhost` because some Python dependencies do not resolve `localhost` correctly.

```bash
export E2E_DB_URL='mysql://merico:merico@127.0.0.1:3306/lake_test?charset=utf8mb4&parseTime=True'

cd backend
go run ./test/init.go
make e2e-test
```

The service E2E tests run the `gitextractor` plugin, which reads repositories
with the `git` command line. Make sure `git` (2.31 or newer) is installed.

### Planning-failure database regression test

`TestBlueprintPlanFailureDatabase` verifies failed-pipeline history, exclusion from
the pending queue, and atomic rollback when label insertion fails. Use dedicated,
disposable databases: the test creates pipeline, label, and task tables, and cleans
up its own records. It does not require external data sources or a running server.

After generating the core and helper mocks, run from `backend/`:

```bash
export DEVLAKE_TEST_MYSQL_DSN='merico:merico@tcp(127.0.0.1:3306)/blueprint_test?charset=utf8mb4&parseTime=True&loc=UTC'
export DEVLAKE_TEST_POSTGRES_DSN='host=127.0.0.1 user=postgres dbname=blueprint_test sslmode=disable'
go test -count=1 -v -run TestBlueprintPlanFailureDatabase ./server/services
```

Each database subtest is skipped when its DSN is unset; ordinary unit tests do not
require database servers.
