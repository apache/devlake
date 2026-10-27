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
# Redmine

Apache DevLake plugin for ingesting data from [Redmine](https://www.redmine.org/) through the [REST API](https://www.redmine.org/projects/redmine/wiki/Rest_api). Each DevLake scope is a Redmine project.

Collected data is normalized into DevLake's ticket domain so it can be used in Grafana dashboards and cross-domain metrics.

## Collected data

| Redmine resource | Raw table | Tool table | Domain tables |
| --- | --- | --- | --- |
| Issues | `_raw_redmine_issues` | `_tool_redmine_issues` | `issues`, `board_issues`, `issue_assignees`, `issue_labels`, `issue_relationships` |
| Journals (optional) | `_raw_redmine_issuejournals` | `_tool_redmine_journals` | `issue_comments`, `issue_changelogs` |
| Time entries (optional) | `_raw_redmine_timeentries` | `_tool_redmine_timeentries` | `issue_worklogs` |
| Projects (scope) | — | `_tool_redmine_projects` | `boards` |

Custom field values stay on `_tool_redmine_issues.custom_fields_json`.

Issues are collected incrementally by `updated_on`. A full sync applies the project sync policy `timeAfter` as a Redmine `created_on` filter.

## Configuration

1. Enable the REST API on the Redmine instance.
2. Create an API key under **My account → API access key** for a user who can see the projects to sync.
3. In Config UI, add a Redmine connection with the instance base URL and that API key.
4. Select projects and, if needed, map tracker and status names. Journals and time entries are off unless enabled on the scope config. Time entries require the Redmine permission **View time entries**.

### Default mappings

When a scope config does not override a name, the plugin uses these defaults. Any other tracker becomes `TASK`. Any other status becomes `OTHER`.

| Redmine tracker | DevLake type |
| --- | --- |
| Bug | INCIDENT |
| Feature | REQUIREMENT |
| Support | TASK |
| Task | TASK |

| Redmine status | DevLake status |
| --- | --- |
| New | TODO |
| In Progress | IN_PROGRESS |
| Feedback | IN_PROGRESS |
| Resolved | DONE |
| Closed | DONE |
| Rejected | DONE |

## Dashboard

After a blueprint run, open the Redmine dashboard in Grafana. MySQL and PostgreSQL definitions are in `grafana/dashboards/mysql/Redmine.json` and `grafana/dashboards/postgresql/Redmine.json`.
