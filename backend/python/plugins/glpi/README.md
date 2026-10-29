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
# GLPI

Apache DevLake plugin for ingesting data from [GLPI](https://glpi-project.org/) through its REST API. Each DevLake scope is a GLPI entity.

Tickets are normalized into DevLake's ticket domain. Users and groups are normalized into cross-domain accounts.

## Collected data

| GLPI resource | Raw table | Tool table | Domain tables |
| --- | --- | --- | --- |
| Tickets | `_raw_glpi_tickets` | `_tool_glpi_tickets` | `issues`, `board_issues`, `issue_labels` |
| Ticket users (optional) | `_raw_glpi_ticketusers` | `_tool_glpi_ticketusers` | `issue_assignees` |
| Ticket groups (optional) | `_raw_glpi_ticketgroups` | `_tool_glpi_ticketgroups` | `issue_assignees` |
| Follow-ups (optional) | `_raw_glpi_ticketfollowups` | `_tool_glpi_followups` | `issue_comments` |
| Tasks (optional) | `_raw_glpi_tickettasks` | `_tool_glpi_tickettasks` | `issue_worklogs` |
| History logs (optional) | `_raw_glpi_ticketlogs` | `_tool_glpi_ticketlogs` | `issue_changelogs` |
| Ticket links (optional) | `_raw_glpi_ticketlinks` | `_tool_glpi_ticketlinks` | `issue_relationships` |
| Solutions (optional) | `_raw_glpi_ticketsolutions` | `_tool_glpi_itilsolutions` | — |
| Users | `_raw_glpi_users` | `_tool_glpi_users` | `accounts` |
| Groups | `_raw_glpi_groups` | `_tool_glpi_groups` | `accounts` |
| Entities (scope) | — | `_tool_glpi_entities` | `boards` |

Categories, locations, suppliers, request types, labels, and custom fields collected with tickets stay on `_tool_glpi_itilcategories`, `_tool_glpi_locations`, `_tool_glpi_suppliers`, `_tool_glpi_requesttypes`, `_tool_glpi_ticketlabelrecords`, and `_tool_glpi_ticketcustomfields`.

A full sync applies the project sync policy `timeAfter` as a ticket creation filter. An incremental sync uses the last `date_mod`.

## Configuration

1. Enable the API under **Setup → General → API** and copy the application token.
2. Create a user API token under **My settings → Remote access keys**.
3. In Config UI, add a GLPI connection with the instance base URL, both tokens, and the timezone configured on the GLPI server.
4. Select entities. Assignees are collected unless disabled. Follow-ups, tasks, history, solutions, and ticket links stay off unless enabled on the scope config.

Incident tickets map to `INCIDENT`. Requests map to `TASK`. New maps to `TODO`. Processing and Pending map to `IN_PROGRESS`. Solved and Closed map to `DONE`. Any other status maps to `OTHER`.

## Dashboard

After a blueprint run, open the GLPI dashboard in Grafana. MySQL and PostgreSQL definitions are in `grafana/dashboards/mysql/Glpi.json` and `grafana/dashboards/postgresql/Glpi.json`.
