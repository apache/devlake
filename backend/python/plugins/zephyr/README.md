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
# Zephyr

Apache DevLake plugin for ingesting data from [Zephyr Scale Cloud](https://smartbear.com/product/zephyr/) through the [REST API](https://support.smartbear.com/zephyr-scale-cloud/docs/en/rest-api/zephyr-scale-cloud-rest-api.html). Each DevLake scope is a Jira project.

Test cases and executions are normalized into DevLake's QA domain. Folders, cycles, labels, and issue links stay in the tool layer for the Grafana dashboard.

## Collected data

| Zephyr resource | Raw table | Tool table | Domain tables |
| --- | --- | --- | --- |
| Test cases | `_raw_zephyr_testcases` | `_tool_zephyr_testcases` | `qa_test_cases` |
| Test executions | `_raw_zephyr_testexecutions` | `_tool_zephyr_testexecutions` | `qa_test_case_executions` |
| Test cycles | `_raw_zephyr_testcycles` | `_tool_zephyr_testcycles` | — |
| Folders | `_raw_zephyr_folders` | `_tool_zephyr_folders` | — |
| Test case labels | `_raw_zephyr_testcaselabels` | `_tool_zephyr_testcaselabels` | — |
| Test case issue links | `_raw_zephyr_testcaseissuelinks` | `_tool_zephyr_testcaseissuelinks` | — |
| Test execution issue links | `_raw_zephyr_testexecutionissuelinks` | `_tool_zephyr_testexecutionissuelinks` | — |
| Projects (scope) | — | `_tool_zephyr_projects` | `qa_projects` |

Custom field values stay on `custom_fields_json`. The scope config names the custom field used as the test case type. The default name is `Test Type`.

A full sync applies the project sync policy `timeAfter` as a created-on filter for test cases, a start filter for cycles, and an actual-end filter for executions.

## Configuration

1. Create a Zephyr Scale API access token in Jira under **Profile → Zephyr Scale API Access Tokens**.
2. In Config UI, add a Zephyr connection with the API base URL and that token. The US default is `https://api.zephyrscale.smartbear.com/v2`. The EU base is `https://eu.api.zephyrscale.smartbear.com/v2`.
3. Select Jira projects. Set the test type custom field name when it is not `Test Type`.

Pass and Passed executions map to `SUCCESS`. Fail, Failed, and Blocked map to `FAILED`. In Progress maps to `IN_PROGRESS`. Any other status maps to `PENDING`.

## Dashboard

After a blueprint run, open the Zephyr dashboard in Grafana. MySQL and PostgreSQL definitions are in `grafana/dashboards/mysql/Zephyr.json` and `grafana/dashboards/postgresql/Zephyr.json`.
