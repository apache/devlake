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

The service E2E tests compile the `gitextractor` plugin, which links against `libgit2`. If `libgit2` is installed in
a non-standard location, expose it through `pkg-config` and add an rpath so the test binary can load the dynamic
library at runtime:

```bash
export PKG_CONFIG_PATH=/path/to/libgit2/build
export CGO_LDFLAGS='-L/path/to/libgit2/build -Wl,-rpath,/path/to/libgit2/build'
make e2e-test
```
