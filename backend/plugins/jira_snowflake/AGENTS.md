# jira_snowflake Plugin — Agent Context

Drop-in replacement for the Jira API plugin per board. Reads Jira data from a
Snowflake replica (Fivetran, `JIRA_DB.CLOUDRHAI_MARTS`) and writes into the
existing `_tool_jira_*` tool-layer tables, then runs domain-layer convertors to
produce `ticket.*` domain records.

## Build & Test

```bash
cd backend
go build ./plugins/jira_snowflake/...
go test  ./plugins/jira_snowflake/... -v
golangci-lint run ./plugins/jira_snowflake/...
```

## Layout

```
impl/impl.go              — plugin interfaces, SubTaskMetas, PrepareTaskData
api/connection_api.go     — connection CRUD (POST/GET/PATCH/DELETE)
models/connection.go      — SnowflakeJiraConnection (table: _tool_jira_snowflake_connections)
models/migrationscripts/  — DB migrations
tasks/task_data.go        — JiraSnowflakeOptions, JiraSnowflakeTaskData, OpenSnowflakeDB
tasks/shared.go           — getStdStatus, getTypeMappings (adapted from jira/tasks)
tasks/sync_*.go           — Snowflake SQL queries → _tool_jira_* tool-layer tables
tasks/convert_*.go        — domain-layer convertors (adapted copies of jira/tasks/*)
```

## Subtask pipeline order

1. `syncIssues`            — Snowflake JIRA_ISSUE_NON_PII → `_tool_jira_issues` + `_tool_jira_board_issues`
2. `syncSprints`           — Snowflake JIRA_SPRINT → `_tool_jira_sprints` + `_tool_jira_board_sprints`
3. `syncSprintIssues`      — Snowflake JIRA_CUSTOMFIELDVALUE_NON_PII → `_tool_jira_sprint_issues`
4. `syncChangelogs`        — Snowflake JIRA_CHANGELOG/ITEM → changelog tables
5. `syncWorklogs`          — Snowflake JIRA_WORKLOG → `_tool_jira_worklogs`
6. `syncLabels`            — Snowflake JIRA_LABEL → `_tool_jira_issue_labels`
7. `syncIssueLinks`        — Snowflake JIRA_ISSUELINK → `_tool_jira_issue_relationships`
8. `convertBoard`          — `_tool_jira_boards` → domain `ticket.Board`
9. `convertIssues`         — `_tool_jira_issues` → domain `ticket.Issue` (adapted deletion)
10. `convertIssueLabels`   — `_tool_jira_issue_labels` → domain `ticket.IssueLabel`
11. `convertWorklogs`      — `_tool_jira_worklogs` → domain `ticket.IssueWorklog`
12. `convertChangelogs`    — changelog tables → domain `ticket.IssueChangelogs`
13. `convertIssueRelationships` — relationships → domain `ticket.IssueRelationship`
14. `convertSprints`       — `_tool_jira_sprints` → domain `ticket.Sprint`
15. `convertSprintIssues`  — `_tool_jira_sprint_issues` → domain `ticket.SprintIssue`

## Key conventions

- **Scope unit is `JiraBoard`** (numeric BoardId + explicit ProjectKeys list).
  A board may span multiple Jira projects; all queries use `WHERE i.PROJECT IN (:projectKeys)`.
- **No raw-table layer**: the plugin writes directly to `_tool_jira_*` tables. The
  `convert_issues.go` uses `_raw_data_params`-scoped deletion instead of the
  upstream `_raw_data_table`-based deletion.
- **`StatusKey` mapping**: `JIRA_ISSUESTATUS_RHAI.STATUSCATEGORY` stores the Jira
  status category as a **numeric string**. On `redhat.atlassian.net` the ids are
  `'2'`=new/todo, `'3'`=done, `'4'`=in-progress (verified against
  `/rest/api/3/status` — note this differs from older Atlassian docs that listed
  3=indeterminate, 4=done). The `CASE` in `sync_issues.go` maps these to
  DevLake keys (`new` / `done` / `indeterminate`).
- **`StdType` / `StdStatus`**: populated in `sync_issues.go` using scope config
  type/status mappings (same logic as `issue_extractor.go` in the jira plugin).
- **Scope config precedence** (in `PrepareTaskData`, same as the jira plugin):
  1. Inline `options.scopeConfig`
  2. Else `options.scopeConfigId`
  3. Else `_tool_jira_boards.scope_config_id` for this connection + board
  4. Else empty config (raw type names uppercased)
  Scope configs live in shared `_tool_jira_scope_configs` — reuse a jira-plugin
  config id when possible. For prod, set the board link once:
  `UPDATE _tool_jira_boards SET scope_config_id = <id> WHERE connection_id = <sf> AND board_id = <board>;`
  then blueprints only need `connectionId` / `boardId` / `projectKeys`.
- **`TIMEWORKED` → `TimeSpentSeconds`**: stored as-is (seconds); the worklog
  convertor divides by 60 when writing `TimeSpentMinutes` to the domain layer.
- **AuthType**: `"keypair"` (default, JWT) or `"externalbrowser"` (SSO, dev only).
  Browser auth only works when DevLake runs natively on a desktop host, not inside a container.

## Snowflake schema notes (JIRA_DB.CLOUDRHAI_MARTS — verified 2026-07-09)

| Table | Key columns | Notes |
|---|---|---|
| `JIRA_ISSUE_NON_PII` | `ID, ISSUE_KEY, PROJECT, ISSUETYPE, ISSUESTATUS_ID, SUMMARY, DESCRIPTION, PARENT_ID, TIMEORIGINALESTIMATE, TIMEESTIMATE, TIMESPENT, CREATED, UPDATED, RESOLUTIONDATE, DUEDATE, PRIORITY, COMPONENT, FIXFOR` | No `SUBTASK_FLAG`; use `JIRA_ISSUETYPE_RHAI.PSTYLE = 'subtask'` instead. `TIMEESTIMATE`, `DESCRIPTION`, `COMPONENT`, and `FIXFOR` can be NULL. `PRIORITY` stores a numeric Jira priority ID, not a human-readable name. |
| `JIRA_ISSUESTATUS_RHAI` | `ID, PNAME, STATUSCATEGORY` | `STATUSCATEGORY` is a numeric string (`'2'`=new, `'3'`=done, `'4'`=indeterminate on redhat.atlassian.net). |
| `JIRA_ISSUETYPE_RHAI` | `ID, PNAME, PSTYLE` | `PSTYLE = 'subtask'` marks subtask types; can be NULL for normal types. |
| `JIRA_PROJECT_RHAI` | `ID, PNAME, PKEY` | Join via `i.PROJECT = p.PKEY`. |
| `JIRA_CUSTOMFIELDVALUE_NON_PII` | `ID, CUSTOMFIELD_NAME, ISSUE, NUMBERVALUE, STRINGVALUE` | `NUMBERVALUE` is `NUMBER(38,0)` — use `::FLOAT` / `::BIGINT` cast, not `TRY_CAST`. |
| `JIRA_COMPONENT_RHAI` | `ID, PROJECT, CNAME` | Component name lookup. However `JIRA_ISSUE_NON_PII.COMPONENT` is always NULL (see limitations below). |
| `JIRA_CHANGEGROUP_RHAI_CLUSTERED` | `ID, ISSUEID, CREATED` | Replaces `JIRA_CHANGELOG`. No author column (PII-stripped). |
| `JIRA_CHANGEITEM_NON_PII_CLUSTERED` | `ID, GROUPID, FIELD, FIELDTYPE, OLDVALUE, NEWVALUE` | Replaces `JIRA_CHANGELOGITEM`. No `FIELDID`, `OLDSTRING`, `NEWSTRING` columns. |

**Not available** in this schema (Fivetran PII policy or not replicated):
`JIRA_SPRINT`, `JIRA_WORKLOG`, `JIRA_LABEL`, `JIRA_ISSUELINK`, `JIRA_ISSUELINKTYPE`,
`JIRA_PRIORITY_RHAI` (priority name lookup), `JIRA_VERSION_RHAI` (fix-version name lookup)

Therefore only these subtasks produce data: `syncIssues`, `syncChangelogs`, and their convertors.
The remaining sync subtasks (`syncSprints`, `syncSprintIssues`, `syncWorklogs`, `syncLabels`, `syncIssueLinks`) will fail if enabled.

**Known field limitations (verified 2026-07-09):**

| Field | Status | Reason |
|---|---|---|
| `priority_name` | Numeric Jira priority ID (e.g. `10001`) — not a name | `JIRA_PRIORITY_RHAI` not replicated by Fivetran; no name lookup available |
| `components` | Always empty | `JIRA_ISSUE_NON_PII.COMPONENT` is always NULL; issue-to-component is a many-to-many relation not stored on the issue row |
| `fix_versions` | Always empty | `JIRA_ISSUE_NON_PII.FIXFOR` is always NULL; `JIRA_VERSION_RHAI` not replicated by Fivetran |

## Jira plugin models dependency

The `tasks/` and `impl/` packages import from `plugins/jira/models` (tool-layer
struct definitions like `JiraIssue`, `JiraSprint`, etc.). This is intentional:
both plugins share the same `_tool_jira_*` table schemas. This is not a business-
logic cross-import — it is a shared schema dependency.

The `jira` plugin does **not** need to be deployed alongside `jira_snowflake`.
`impl.Init` registers a minimal `jiraPluginStub` so that `didgen.NewDomainIdGenerator`
can resolve `jira/models` types without the full plugin being loaded.

## Convertor files — adapted copies

`tasks/convert_*.go` are adapted copies of the corresponding files in `plugins/jira/tasks/`.
They differ in:
- Package (`tasks` with `JiraSnowflakeTaskData` instead of `JiraTaskData`)
- `convert_issues.go`: full-sync deletion via `_raw_data_params` scope (not `_raw_data_table`)
- `convert_changelogs.go`: `getIssueFieldMap` call removed (no issue fields table populated)

**TODO (long-term):** Extract the shared convertor logic to
`helpers/jiraconvertors/` so both `jira` and `jira_snowflake` import from a
neutral shared package instead of having diverging copies.

## Don'ts

- Don't add models without a migration in `migrationscripts/register.go`
- Don't skip the Apache 2.0 license header on new `.go` files
- Don't configure the same board in both a Jira API connection and a
  jira_snowflake connection simultaneously — this causes domain ID duplication
