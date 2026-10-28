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
# Proposal: `jira_snowflake` Plugin for Snowflake-Backed Jira Ingestion

**This is a documentation-only proposal for team review — no code changes.**

## Summary

- Red Hat's Jira history is already replicated into Snowflake by Fivetran (`JIRA_DB.CLOUDRHAI_MARTS`), covering 5.5M+ issues, 47M+ changelog rows, sprints, worklogs, and issue links.
- DevLake currently ingests Jira data by calling the Jira REST API directly, duplicating work already done by Fivetran and placing unnecessary load on the Jira API.
- Per-project historical backfills that take hours via the Jira API complete in minutes via Snowflake.
- Proposed: a new `jira_snowflake` plugin that reads from Snowflake and writes into the same `_tool_jira_*` tables as the existing Jira plugin, then calls existing Jira convertor tasks (with a `*JiraTaskData` shim — see implementation notes).
- Migration is per-board and non-destructive: a team removes a board from the Jira API connection and adds it to the Snowflake connection. The two connections cannot hold the same board simultaneously, preventing duplication.

## Problem Statement

DevLake ingests Jira data by polling the Jira REST API. For Red Hat projects this creates two problems:

**Redundant data collection.** Fivetran already continuously replicates the full Red Hat Jira instance into Snowflake on a 15–60 minute schedule. Running a separate DevLake collector against the same Jira instance means two systems polling the same API for the same data.

**Slow historical backfill.** The Jira API is rate-limited and paginates in small pages. A full changelog backfill for a large project (e.g. KONFLUX) takes many hours. The same data is immediately available in Snowflake with no rate limit — a per-project backfill that takes hours via the API takes minutes via a Snowflake query.

## What is Available in Snowflake

All data required to fully populate DevLake's Jira tool-layer tables is present in `JIRA_DB.CLOUDRHAI_MARTS`:

| Snowflake table | DevLake target table | Notes |
|---|---|---|
| `JIRA_ISSUE_NON_PII` | `_tool_jira_issues` | 5.5M+ issues |
| `JIRA_CHANGELOG` + `JIRA_CHANGELOGITEM` | `_tool_jira_issue_changelogs` / `_tool_jira_issue_changelog_items` | 47M / 17M rows |
| `JIRA_SPRINT` | `_tool_jira_sprints`, `_tool_jira_board_sprints` | Sprint state derived from `STARTED`/`CLOSED` booleans |
| `JIRA_CUSTOMFIELDVALUE_NON_PII` (field `Sprint`) | `_tool_jira_sprint_issues`, `_tool_jira_board_issues` | Sprint membership via custom field |
| `JIRA_WORKLOG` | `_tool_jira_worklogs` | `TIMEWORKED` in seconds — store directly into `TimeSpentSeconds` |
| `JIRA_ISSUELINK` | `_tool_jira_issue_relationships` | |
| `JIRA_LABEL` | `_tool_jira_issue_labels` | |
| `JIRA_PROJECT_RHAI` | `_tool_jira_boards` (scope) | |

**Confirmed gap — user PII.** Assignee and creator names/IDs are stripped from the Snowflake tables (`_NON_PII` suffix). DevLake `assignee_name`, `assignee_id`, `creator_name` fields will be null. This affects "issues by assignee" dashboard breakdowns but does not affect flow metrics (lead time, cycle time, DORA).

**Non-team contributor noise — known gap.** A Jira project space is often shared — upstream contributors or members of adjacent teams may file or work on issues in the same project. Schema investigation confirmed that all user-identifying data is stripped: the `Team` custom field has 5.17M rows but all value columns are null, `JIRA_RAPIDVIEW_RHAI` (board JQL filter) is not present, and assignee/creator are stripped. There is currently no reliable way to filter non-team contributors using Snowflake data alone.

This gap will be addressed if and when the use case arises. The fallback is to supplement the Snowflake sync with a targeted Jira API query to retrieve team membership or board membership data — a minimal additional call rather than a full API-based ingestion.

## Proposed Approach

Build a new plugin `jira_snowflake` that:

1. Opens a Snowflake connection using key-pair (JWT) authentication — the only viable auth method for a headless server process.
2. For each sync task, runs a Snowflake SQL query scoped to all project keys belonging to the configured board and (for incremental runs) filtered by `UPDATED > timeRangeStart` from DevLake's `Sync Policy`.
3. Writes rows directly into the existing `_tool_jira_*` tool-layer tables, including populating `StdType` and `StdStatus` using the same mapping logic that the Jira extractor applies.
4. Calls the **existing Jira convertor tasks** by constructing a `*tasks.JiraTaskData` shim (see convertor reuse strategy below).

## How Teams Migrate

The Snowflake-Jira plugin is a **drop-in replacement** for the Jira API plugin, not an addition. A board is configured in one connection at a time.

Steps for a team migrating a board:

1. A DevLake admin creates a `Snowflake-Jira` connection (Snowflake account, service user, RSA key, database/schema config).
2. For each board being migrated: remove it from the Jira API connection, add it to the Snowflake connection.
3. On the next pipeline run, DevLake performs a full sync from Snowflake for that board. The convertor layer replaces existing domain data for the board.
4. The Jira API connection stops receiving requests for those boards.

Teams with boards from multiple Jira instances (Red Hat + external) can split per-board: Red Hat boards via Snowflake, others via the existing Jira API connection. No duplication occurs as long as the same board is not in both connections.

The plugin must reach full feature parity with the Jira API plugin before any board is migrated, so that no dashboard metrics regress after migration.

## Data Freshness and Trade-offs

| Dimension | Snowflake connection | Jira API connection |
|---|---|---|
| Freshness | Lags real-time by 1 Fivetran sync (15–60 min) + pipeline run time | Near real-time |
| Historical backfill speed | Minutes per project | Hours per project |
| API load on Jira | None | Continuous polling |
| Assignee/creator fields | Null (PII stripped) | Populated |
| Rate limiting | None (Snowflake compute credits only) | Yes (Jira API rate limits) |

With DevLake pipelines running on a daily schedule and Fivetran syncing every 15–60 minutes, the Snowflake path may actually deliver fresher data than a daily Jira API pipeline. Performance at scale should be verified during piloting.

## Operational Considerations

**Authentication.** A dedicated Snowflake service account with an RSA key pair is required. Browser SSO (`externalbrowser`) cannot be used in a headless server. The private key PEM is stored encrypted in DevLake's connection record.

**Warehouse.** The plugin uses the `DEFAULT` warehouse, which is the warehouse we have access to. It is a Medium, multi-cluster (1–2) warehouse with auto-resume enabled. No additional provisioning is required. Because it is a shared warehouse, DevLake queries run alongside other workloads — this is expected and the multi-cluster configuration should handle concurrent demand automatically.

**Concurrency.** With a multi-cluster warehouse, queries queue only when all clusters are at capacity. 

---

## Implementation Reference

> The section below is a technical specification intended for the implementer or AI agent. It is not required reading for the team design review.

### Development build order

The plugin must reach full parity before any board migration. Recommended order:

1. Connection model, `sync_issues`, reuse `ConvertIssuesMeta` — verify issue counts in dashboards
2. `sync_sprints`, `sync_sprint_issues`, `sync_changelogs` — verify sprint reports and lead time
3. `sync_worklogs`, `sync_labels`, `sync_issue_links` — verify time tracking and dependency views

### Plugin file structure

```
backend/plugins/jira_snowflake/
├── jira_snowflake.go          # binary entry, var PluginEntry impl.JiraSnowflake
├── impl/
│   └── impl.go                # plugin registration, SubTaskMetas(), ApiResources()
├── api/
│   └── connection_api.go      # connection CRUD, uses helper.BaseConnection
├── models/
│   └── connection.go          # SnowflakeJiraConnection struct
└── tasks/
    ├── task_data.go            # JiraSnowflakeTaskData, Options struct
    ├── sync_issues.go          # → _tool_jira_issues
    ├── sync_sprints.go         # → _tool_jira_sprints, _tool_jira_boards, _tool_jira_board_sprints
    ├── sync_sprint_issues.go   # → _tool_jira_sprint_issues, _tool_jira_board_issues
    ├── sync_changelogs.go      # → _tool_jira_issue_changelogs, _tool_jira_issue_changelog_items
    ├── sync_worklogs.go        # → _tool_jira_worklogs
    ├── sync_issue_links.go     # → _tool_jira_issue_relationships
    └── sync_labels.go          # → _tool_jira_issue_labels
```

Reference: `backend/plugins/jira/` for all patterns. The plugin has no collector or raw-table layer.

### Convertor reuse strategy

After the sync tasks populate `_tool_jira_*` tables, the plugin calls existing Jira convertor tasks from `impl.go`. The full list of convertors to include (cross-checked against `jira/impl/impl.go`):

```go
import jiratasks "github.com/apache/devlake/backend/plugins/jira/tasks"

// SubTaskMetas() — convertor tasks only (no Collect/Extract needed):
jiratasks.ConvertBoardMeta,
jiratasks.ConvertIssuesMeta,
jiratasks.ConvertIssueLabelsMeta,
// ConvertIssueCommentsMeta: omit — no JIRA_COMMENT table available in Snowflake; also EnabledByDefault: false
jiratasks.ConvertWorklogsMeta,
jiratasks.ConvertIssueChangelogsMeta,
jiratasks.ConvertIssueRelationshipsMeta,
jiratasks.ConvertSprintsMeta,
jiratasks.ConvertSprintIssuesMeta,
// ConvertIssueCommitsMeta / ConvertIssueRepoCommitsMeta: omit unless dev-panel data is available
// ConvertAccountsMeta: omit — assignee/creator PII is stripped in Snowflake
```

**`JiraTaskData` shim — type assertion compatibility.** Every convertor performs `subtaskCtx.GetData().(*JiraTaskData)`. The plugin's `PrepareTaskData` must therefore return a `*tasks.JiraTaskData`, not a custom struct. Critically, after auditing all convertor source files, **no convertor accesses `data.ApiClient` or `data.JiraServerInfo`** — those fields are only used by collector and extractor tasks. The shim is safe to construct with `ApiClient = nil` and a zero-value `JiraServerInfo`:

```go
func (p JiraSnowflake) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
    // decode options into JiraSnowflakeOptions...
    return &jiratasks.JiraTaskData{
        Options: &jiratasks.JiraOptions{
            ConnectionId: opt.ConnectionId,
            BoardId:      opt.BoardId,
            // ScopeConfig from DB as normal
        },
        ApiClient:      nil,         // not used by any convertor
        JiraServerInfo: models.JiraServerInfo{}, // zero value safe for convertors
    }, nil
}
```

**Raw-table lineage — full-sync deletion.** The Jira convertor tasks use `StatefulDataConverter`, which in full-sync mode deletes domain rows matched by `_raw_data_table` and `_raw_data_params`. Because this plugin has no raw-table layer, those delete queries will match no rows and stale domain records will not be removed automatically. Mitigation: the sync tasks should upsert (not insert-only) into `_tool_jira_*`, and the `BeforeConvert` hook or explicit pre-deletion should be used to remove stale domain rows by scope. This is the same pattern used by the GitLab MR convertor. Document and implement this explicitly before production migration.

### Connection model

```go
type SnowflakeJiraConnection struct {
    helper.BaseConnection `mapstructure:",squash"` // provides ID, Name, CreatedAt, UpdatedAt

    Account    string `json:"account"    gorm:"column:account"`
    User       string `json:"user"       gorm:"column:sf_user"`
    PrivateKey string `json:"privateKey" encrypt:"yes" gorm:"column:private_key"`
    Database   string `json:"database"   gorm:"column:sf_database"` // e.g. JIRA_DB
    Schema     string `json:"schema"     gorm:"column:sf_schema"`   // e.g. CLOUDRHAI_MARTS
    Warehouse  string `json:"warehouse"  gorm:"column:warehouse"`   // DEFAULT
    Role       string `json:"role"       gorm:"column:sf_role"`
}
```

Use `gosnowflake.DSN(&gosnowflake.Config{Authenticator: gosnowflake.AuthTypeJwt, PrivateKey: ...})`. The private key PEM is stored encrypted in DevLake's connection record.

**Scope model — board-level, not project-level.** A Jira board can contain issues from multiple projects (e.g. a board tracking both DPROD and HELM). The scope unit is therefore the **board** (`models.JiraBoard`), not a single project PKEY. The `Options` struct carries a numeric `BoardId` (matching `JiraBoard.BoardId uint64`). The set of project keys belonging to that board must be resolved at sync time — either by querying `JIRA_SPRINT.RAPID_VIEW_ID` → project association, or by providing the list explicitly in scope config. The `WHERE` clause in all sync queries uses `i.PROJECT IN (:projectKeys)`, not `i.PROJECT = :singleKey`. Reuse `models.JiraScopeConfig` for type/status mappings.

### Key sync queries

**`sync_issues.go`** — incremental filter uses DevLake's `Sync Policy` `timeRangeStart` (set per blueprint). For a full sync `timeRangeStart` is zero/null and the filter is omitted.

**Critical: `StdType` and `StdStatus` must be populated by this sync task.** The Jira convertor reads `jiraIssue.StdType` and `jiraIssue.StdStatus` directly from `_tool_jira_issues` to populate `ticket.Issue.Type` and `ticket.Issue.Status` in the domain layer. These fields are normally set by `ExtractIssuesMeta` — since this plugin has no extract phase, the sync task must apply equivalent logic:

- `StdType`: look up `JiraIssueType` records for the connection from `_tool_jira_issue_types`, apply scope config `StdTypeMappings`; fall back to `strings.ToUpper(typeName)`.
- `StdStatus`: apply `getStdStatus(statusKey)` (maps Jira status category IDs to `"TODO"/"IN_PROGRESS"/"DONE"`); override from scope config `StandardStatusMappings` if present.

Reference: `backend/plugins/jira/tasks/issue_extractor.go` lines 174–182 and `getTypeMappings()`.

```sql
SELECT
    i.ID,
    i.ISSUE_KEY,
    p.ID        AS project_id,    -- numeric uint64, not the PKEY string
    i.PROJECT   AS project_key,
    p.PNAME     AS project_name,
    it.PNAME    AS issue_type,
    s.PNAME     AS status_name,
    s.STATUSCATEGORY_ID AS status_key,
    i.SUMMARY,
    i.DESCRIPTION,
    i.CREATED,
    i.UPDATED,
    i.RESOLUTIONDATE,
    i.DUEDATE,
    i.PARENT_ID,
    i.TIMEORIGINALESTIMATE AS original_estimate_seconds,
    i.TIMEESTIMATE         AS remaining_estimate_seconds,
    i.TIMESPENT            AS time_spent_seconds,
    sp.NUMBERVALUE         AS story_point,
    el.STRINGVALUE         AS epic_key,
    sprint_cf.NUMBERVALUE  AS sprint_id
FROM JIRA_DB.CLOUDRHAI_MARTS.JIRA_ISSUE_NON_PII i
JOIN JIRA_DB.CLOUDRHAI_MARTS.JIRA_ISSUETYPE_RHAI   it ON i.ISSUETYPE      = it.ID
JOIN JIRA_DB.CLOUDRHAI_MARTS.JIRA_ISSUESTATUS_RHAI  s  ON i.ISSUESTATUS_ID = s.ID
JOIN JIRA_DB.CLOUDRHAI_MARTS.JIRA_PROJECT_RHAI      p  ON i.PROJECT        = p.PKEY
LEFT JOIN JIRA_DB.CLOUDRHAI_MARTS.JIRA_CUSTOMFIELDVALUE_NON_PII sp
    ON sp.ISSUE = i.ID AND sp.CUSTOMFIELD_NAME = 'Story Points'
LEFT JOIN JIRA_DB.CLOUDRHAI_MARTS.JIRA_CUSTOMFIELDVALUE_NON_PII el
    ON el.ISSUE = i.ID AND el.CUSTOMFIELD_NAME = 'Epic Link'
LEFT JOIN JIRA_DB.CLOUDRHAI_MARTS.JIRA_CUSTOMFIELDVALUE_NON_PII sprint_cf
    ON sprint_cf.ISSUE = i.ID AND sprint_cf.CUSTOMFIELD_NAME = 'Sprint'
WHERE i.PROJECT IN (:projectKeys)       -- all projects on the board
  AND i.UPDATED > :timeRangeStart       -- omitted on full sync
QUALIFY ROW_NUMBER() OVER (
    PARTITION BY i.ID
    ORDER BY sprint_cf.NUMBERVALUE DESC NULLS LAST,
             sp.NUMBERVALUE DESC NULLS LAST
) = 1
```

The `QUALIFY ROW_NUMBER()` deduplicates rows produced by the three `LEFT JOIN`s on `JIRA_CUSTOMFIELDVALUE_NON_PII` — an issue with multiple sprint assignments, story point entries, or epic link entries would otherwise appear multiple times. The `ORDER BY` keeps the most recent sprint and highest story point value. Note: issues in multiple active sprints will retain only one sprint ID; this is an acceptable trade-off since the same limitation applies to the existing Jira API extractor.

**`sync_sprints.go`** — derive state from `STARTED`/`CLOSED` booleans:
- `CLOSED = true` → `"closed"`, `STARTED = true` → `"active"`, else → `"future"`
- `RAPID_VIEW_ID` → `origin_board_id` (also populates `_tool_jira_board_sprints`)

**`sync_sprint_issues.go`**:

```sql
SELECT cf.NUMBERVALUE AS sprint_id, i.ID AS issue_id
FROM JIRA_DB.CLOUDRHAI_MARTS.JIRA_CUSTOMFIELDVALUE_NON_PII cf
JOIN JIRA_DB.CLOUDRHAI_MARTS.JIRA_ISSUE_NON_PII i ON i.ID = cf.ISSUE
WHERE cf.CUSTOMFIELD_NAME = 'Sprint'
  AND cf.NUMBERVALUE IS NOT NULL
  AND i.PROJECT IN (:projectKeys)
```

**`sync_changelogs.go`** — join `JIRA_CHANGELOG` and `JIRA_CHANGELOGITEM` on `GROUPID = ID`. `std_status` mapping is handled by the reused `issue_changelog_convertor.go`.

**`sync_worklogs.go`** — `TIMEWORKED` is in seconds; store it directly into `JiraWorklog.TimeSpentSeconds` **without dividing**. The worklog convertor (`worklog_convertor.go`) already divides by 60 when writing to the domain layer. Pre-dividing would produce values 60× too small. Author fields set to null (PII stripped).

## Test Plan

- [ ] Review problem framing — does the team agree the API redundancy is worth addressing?
- [ ] Confirm the Snowflake service account provisioning process (who owns it, key rotation policy)
- [ ] Confirm the PII gap is acceptable for the metrics teams care about
- [ ] **Acknowledge non-team noise gap** — filtering non-team contributors from shared Jira projects is not solvable with Snowflake data alone (all user-identifying fields are stripped). Confirm this is acceptable for the initial use cases; revisit if needed, potentially supplementing with a targeted Jira API call.
- [ ] Agree on which project(s) to use as a pilot migration
- [ ] Review implementation reference section for correctness before coding begins
