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

# Linear

## Summary

This plugin collects data from [Linear](https://linear.app) through its
[GraphQL API](https://linear.app/developers/graphql) and maps it into DevLake's
standardized `ticket` domain, so Linear issues appear in DevLake dashboards
(throughput, lead/cycle time, sprint burndown, etc.).

The selectable **scope** is a Linear **Team**, which maps to a domain `Board`.

## Supported data

| Linear entity   | Tool-layer table                  | Domain-layer table                         |
|-----------------|-----------------------------------|--------------------------------------------|
| Team            | `_tool_linear_teams` (scope)      | `boards`                                   |
| User            | `_tool_linear_accounts`           | `accounts`                                 |
| Workflow state  | `_tool_linear_workflow_states`    | (drives issue status mapping)              |
| Issue           | `_tool_linear_issues`             | `issues`, `board_issues`                   |
| Label           | `_tool_linear_issue_labels`       | `issue_labels`                             |
| Comment         | `_tool_linear_comments`           | `issue_comments`                           |
| Cycle           | `_tool_linear_cycles`             | `sprints`, `board_sprints`, `sprint_issues`|
| Issue history   | `_tool_linear_issue_history`      | `issue_changelogs`                         |

### Field mapping highlights

- **Status** — derived deterministically from Linear's `WorkflowState.type`
  (no manual mapping needed, unlike Jira):
  - `backlog`, `unstarted` → `TODO`
  - `started` → `IN_PROGRESS`
  - `completed`, `canceled` → `DONE`
- **Priority** — Linear's integer priority maps to a label: `0` No priority,
  `1` Urgent, `2` High, `3` Medium, `4` Low.
- **Type** — Linear has no native issue type, so issues default to `REQUIREMENT`.
- **Lead time** — `completedAt − createdAt` (Linear provides `startedAt`/`completedAt`
  natively; the history changelog captures every status transition).
- **Story points** — Linear's `estimate`.

## Authentication

The plugin uses a Linear **personal API key**, passed verbatim in the
`Authorization` header (no `Bearer` prefix). Create one under
**Settings → Security & access → Personal API keys** in Linear.

## Configuration

Create a connection:

```
curl 'http://localhost:8080/api/plugins/linear/connections' \
--header 'Content-Type: application/json' \
--data-raw '{
    "name": "linear",
    "endpoint": "https://api.linear.app/graphql",
    "token": "<YOUR_LINEAR_API_KEY>",
    "rateLimitPerHour": 1500
}'
```

Add a team scope (the team id is the Linear team UUID):

```
curl 'http://localhost:8080/api/plugins/linear/connections/<CONNECTION_ID>/scopes' \
--header 'Content-Type: application/json' \
--data-raw '{
    "data": [{ "connectionId": <CONNECTION_ID>, "teamId": "<TEAM_ID>", "name": "Engineering" }]
}'
```

## Collecting data

```
curl 'http://localhost:8080/api/pipelines' \
--header 'Content-Type: application/json' \
--data-raw '{
    "name": "linear pipeline",
    "plan": [[{
        "plugin": "linear",
        "options": { "connectionId": <CONNECTION_ID>, "teamId": "<TEAM_ID>" }
    }]]
}'
```

## Rate limiting

Linear enforces a per-API-key request budget (1,500 requests/hour) plus a
complexity budget. The collector paces requests against the configured
`rateLimitPerHour` (default 1500). Issues are collected incrementally using
`updatedAt` ordering so re-runs only fetch changes.

## Limitations / roadmap

- Authentication is personal API key only; OAuth2 is a planned follow-up.
- Issue type defaults to `REQUIREMENT`; label-based type mapping via the scope
  config is a planned follow-up.
- config-ui integration (connection form + team picker) and the website
  documentation page are planned follow-ups; for now connections and scopes are
  managed via the API calls shown above.
