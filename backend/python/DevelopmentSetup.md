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
# Python prerequisites for `make e2e-test`

`make e2e-test` also requires a test database and, for some service tests, `libgit2`. See
[`backend/test/Readme.md`](../test/Readme.md) for the full local E2E test flow.

For the Python plugin tests to run properly, the following steps must be taken:

1. The following packages are required for Ubuntu: `libffi-dev default-libmysqlclient-dev libpq-dev`
2. `python3.9` is required by the time of this document. 
   - Try `deadsnakes` if you are using Ubuntu 22.04 or above, the `python3.9-dev` is required.
   - Use `virtualenv` if you are having multiple python versions. `virtualenv -p python3.9 path/to/venv` and `source path/to/venv/bin/activate.sh` should do the trick
3. both `mysql-client` and `postgresql` are required. 
   - `postgresql` is required for `psycopg2` to work
4. [poetry](https://python-poetry.org/) is required. 
   - run `cd backend/python/pydevlake && poetry install`
   - run `cd backend/python/plugins/azuredevops && poetry install`
5. `sqlalchemy` won't work with `localhost` in the database connection string, use `127.0.0.1` instead
