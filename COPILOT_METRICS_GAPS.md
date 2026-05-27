# GitHub Copilot Metrics Gaps — Changes Documentation

## Overview

This document describes the gaps identified between the DevLake `gh-copilot` plugin and the
current GitHub Copilot Metrics API, and the changes made to close those gaps. All changes
target per-user metrics exposure, correlation data, and full API field parity.

---

## Gap Analysis Summary

The GitHub Copilot Metrics API (new report-download API, replacing the deprecated `/copilot/metrics`
inline endpoint sunset on April 2, 2026) exposes significantly more data than the DevLake plugin
was capturing. The gaps fell into five categories:

| Category | Gap Description | Impact |
|---|---|---|
| **Enterprise/Org Daily Metrics** | Missing CLI user counts, code review user counts, chat panel modes, expanded PR metrics, CLI breakdown | Cannot track CLI adoption, code review usage, chat mode distribution, or detailed PR impact |
| **Per-User Daily Metrics** | Missing `used_cli`, `used_copilot_code_review_active/passive` flags and CLI breakdown | Cannot identify which users use CLI or code review features |
| **User-Team Mapping** | Entirely missing — no collector or model for `user-teams-1-day` endpoint | Cannot correlate user metrics to teams for team-level dashboards |
| **Seat Assignments** | Missing team assignment fields and user name/email | Cannot identify which team assigned a seat or show user display names |
| **Pull Request Metrics** | Only 4 of 14 PR fields captured | Cannot track merged PRs, merge time, Copilot review suggestions, or copilot-authored merge rates |

---

## Changes Made

### 1. Enterprise/Org Daily Metrics Model (`enterprise_metrics.go`)

**New embedded struct: `CopilotCliMetrics`**
- `CliSessionCount` — Number of CLI sessions
- `CliRequestCount` — Number of CLI requests
- `CliPromptCount` — Number of CLI prompts
- `CliOutputTokenSum` — Total output tokens from CLI
- `CliPromptTokenSum` — Total prompt tokens from CLI

**New fields on `GhCopilotEnterpriseDailyMetrics`:**

| Field | Type | API Source | Description |
|---|---|---|---|
| `DailyActiveCliUsers` | int | `daily_active_cli_users` | Daily active CLI users |
| `DailyActiveCopilotCodeReviewUsers` | int | `daily_active_copilot_code_review_users` | Users actively using code review |
| `DailyPassiveCopilotCodeReviewUsers` | int | `daily_passive_copilot_code_review_users` | Users passively receiving code review |
| `WeeklyActiveCopilotCodeReviewUsers` | int | `weekly_active_copilot_code_review_users` | 7-day trailing active code review users |
| `WeeklyPassiveCopilotCodeReviewUsers` | int | `weekly_passive_copilot_code_review_users` | 7-day trailing passive code review users |
| `MonthlyActiveCopilotCodeReviewUsers` | int | `monthly_active_copilot_code_review_users` | 28-day trailing active code review users |
| `MonthlyPassiveCopilotCodeReviewUsers` | int | `monthly_passive_copilot_code_review_users` | 28-day trailing passive code review users |
| `ChatPanelAgentMode` | int | `chat_panel_agent_mode` | Chat panel interactions in agent mode |
| `ChatPanelAskMode` | int | `chat_panel_ask_mode` | Chat panel interactions in ask mode |
| `ChatPanelCustomMode` | int | `chat_panel_custom_mode` | Chat panel interactions in custom mode |
| `ChatPanelEditMode` | int | `chat_panel_edit_mode` | Chat panel interactions in edit mode |
| `ChatPanelPlanMode` | int | `chat_panel_plan_mode` | Chat panel interactions in plan mode |
| `ChatPanelUnknownMode` | int | `chat_panel_unknown_mode` | Chat panel interactions in unknown mode |
| `PRTotalMerged` | int | `pull_requests.total_merged` | Total PRs merged |
| `PRMedianMinutesToMerge` | float64 | `pull_requests.median_minutes_to_merge` | Median minutes to merge PRs |
| `PRTotalSuggestions` | int | `pull_requests.total_suggestions` | Total PR review suggestions |
| `PRTotalAppliedSuggestions` | int | `pull_requests.total_applied_suggestions` | Total applied PR suggestions |
| `PRTotalMergedCreatedByCopilot` | int | `pull_requests.total_merged_created_by_copilot` | Merged PRs created by Copilot |
| `PRTotalMergedReviewedByCopilot` | int | `pull_requests.total_merged_reviewed_by_copilot` | Merged PRs reviewed by Copilot |
| `PRMedianMinToMergeCopilotAuthored` | float64 | `pull_requests.median_minutes_to_merge_copilot_authored` | Median merge time for Copilot-authored PRs |
| `PRMedianMinToMergeCopilotReviewed` | float64 | `pull_requests.median_minutes_to_merge_copilot_reviewed` | Median merge time for Copilot-reviewed PRs |
| `PRTotalCopilotSuggestions` | int | `pull_requests.total_copilot_suggestions` | Total Copilot review suggestions |
| `PRTotalCopilotAppliedSuggestions` | int | `pull_requests.total_copilot_applied_suggestions` | Total Copilot applied suggestions |
| `CopilotCliMetrics` (embedded) | struct | `totals_by_cli.*` | CLI session/request/prompt/token metrics |

### 2. Per-User Daily Metrics Model (`user_metrics.go`)

**New fields on `GhCopilotUserDailyMetrics`:**

| Field | Type | API Source | Description |
|---|---|---|---|
| `UsedCli` | bool | `used_cli` | Whether user used Copilot CLI on this day |
| `UsedCopilotCodeReviewActive` | bool | `used_copilot_code_review_active` | Whether user actively used code review |
| `UsedCopilotCodeReviewPassive` | bool | `used_copilot_code_review_passive` | Whether user passively used code review |
| `CopilotCliMetrics` (embedded) | struct | `totals_by_cli.*` | Per-user CLI session/request/prompt/token metrics |

### 3. User-Team Mapping (NEW — `user_team.go`)

**New model: `GhCopilotUserTeam`** (table: `_tool_copilot_user_teams`)

This is an entirely new data source. The GitHub API `user-teams-1-day` endpoint returns which
teams each user belongs to per day. This enables **team-level metrics aggregation** by joining
with the per-user daily metrics tables.

| Field | Type | API Source | Description |
|---|---|---|---|
| `ConnectionId` | uint64 | — | DevLake connection (PK) |
| `ScopeId` | string | — | DevLake scope (PK) |
| `Day` | time.Time | `day` | Date of the mapping (PK) |
| `UserId` | int64 | `user_id` | GitHub user ID (PK) |
| `TeamId` | int64 | `team_id` | GitHub team ID (PK) |
| `UserLogin` | string | `user_login` | GitHub username |
| `OrganizationId` | string | `organization_id` | Organization ID |
| `EnterpriseId` | string | `enterprise_id` | Enterprise ID |
| `TeamSlug` | string | `slug` | Team slug for display |

**New collector:** `CollectUserTeams` — fetches from `user-teams-1-day` endpoint (JSONL format)
**New extractor:** `ExtractUserTeams` — parses JSONL records into the model

### 4. Seat Assignments (`seat.go`)

**New fields on `GhCopilotSeat`:**

| Field | Type | API Source | Description |
|---|---|---|---|
| `UserName` | string | `assignee.name` | User's display name |
| `UserEmail` | string | `assignee.email` | User's email address |
| `AssigningTeamId` | int64 | `assigning_team.id` | Team that assigned the Copilot seat |
| `AssigningTeamName` | string | `assigning_team.name` | Team display name |
| `AssigningTeamSlug` | string | `assigning_team.slug` | Team URL slug |

### 5. Extractor Updates

All extractors were updated to populate the new fields:

- **`enterprise_metrics_extractor.go`** — Updated `enterpriseDayTotal` struct with new API fields;
  updated `pullRequestStats` from 4 to 14 fields; added `totalsByCli` struct; expanded extraction
  logic for all new enterprise-level fields.
- **`metrics_extractor.go`** (org metrics) — Same expansion as enterprise extractor since org reports
  use the identical format. Updated seat response structs to include `copilotTeam` for team assignment.
- **`user_metrics_extractor.go`** — Updated `userDailyReport` struct with 3 new boolean flags and
  CLI breakdown; expanded extraction logic.
- **`seat_extractor.go`** — Updated to extract `UserName`, `UserEmail`, and `AssigningTeam` fields.

### 6. Migration Script

**New file:** `20260527_add_copilot_metrics_gaps.go`

- Adds new columns to `_tool_copilot_enterprise_daily_metrics` (28 columns)
- Adds new columns to `_tool_copilot_user_daily_metrics` (8 columns)
- Adds new columns to `_tool_copilot_seats` (5 columns)
- Creates new table `_tool_copilot_user_teams`

### 7. Registration Updates

- `models.go` — Added `GhCopilotUserTeam` to `GetTablesInfo()`
- `register.go` — Added `CollectUserTeamsMeta` and `ExtractUserTeamsMeta` to subtask list
- `subtasks.go` — Added `CollectUserTeamsMeta` and `ExtractUserTeamsMeta` definitions with dependencies
- `models_test.go` — Updated `TestGetTablesInfo` to include the new table
- `migrationscripts/register.go` — Added `addCopilotMetricsGaps` migration

---

## Files Changed

| File | Change Type | Description |
|---|---|---|
| `models/enterprise_metrics.go` | Modified | Added `CopilotCliMetrics` embedded struct; 28 new fields on enterprise model |
| `models/user_metrics.go` | Modified | Added 3 boolean flags + CLI metrics embed to per-user model |
| `models/seat.go` | Modified | Added 5 team/user detail fields |
| `models/user_team.go` | **Created** | New user-team mapping model |
| `models/models.go` | Modified | Registered `GhCopilotUserTeam` in `GetTablesInfo()` |
| `models/models_test.go` | Modified | Updated table count assertion |
| `models/migrationscripts/20260527_add_copilot_metrics_gaps.go` | **Created** | Migration for all schema changes |
| `models/migrationscripts/register.go` | Modified | Registered new migration |
| `tasks/enterprise_metrics_extractor.go` | Modified | Expanded structs and extraction for 28 new fields |
| `tasks/metrics_extractor.go` | Modified | Expanded org extraction + seat response structs |
| `tasks/user_metrics_extractor.go` | Modified | Added 3 flags + CLI breakdown extraction |
| `tasks/seat_extractor.go` | Modified | Added team and user detail extraction |
| `tasks/user_teams_collector.go` | **Created** | New collector for user-teams-1-day endpoint |
| `tasks/user_teams_extractor.go` | **Created** | New extractor for user-team JSONL records |
| `tasks/subtasks.go` | Modified | Added 2 new subtask metas |
| `tasks/register.go` | Modified | Added 2 new subtasks to execution order |

---

## Correlation Capabilities Enabled

With these changes, the following per-user and cross-dimensional analyses become possible:

1. **User → Team correlation**: Join `_tool_copilot_user_teams` with `_tool_copilot_user_daily_metrics`
   on `(day, user_id)` to compute team-level usage aggregations
2. **CLI adoption tracking**: Filter users/days where `used_cli = true` or analyze CLI token usage
3. **Code review adoption**: Track active vs passive code review usage per user per day
4. **Chat mode analysis**: Understand distribution of agent/ask/edit/plan/custom modes
5. **PR impact analysis**: Track Copilot's impact on merge velocity (`median_minutes_to_merge_copilot_authored`
   vs `median_minutes_to_merge`) and suggestion acceptance rates
6. **Seat utilization by team**: Join seats with `assigning_team` data for team-level licensing analysis
7. **User identity enrichment**: `user_name` and `user_email` on seats enable richer user profiles

---

## API Endpoints Used

| Endpoint | Existing/New | Purpose |
|---|---|---|
| `enterprises/{e}/copilot/metrics/reports/enterprise-1-day` | Existing | Enterprise daily aggregate metrics |
| `orgs/{o}/copilot/metrics/reports/organization-1-day` | Existing | Org daily aggregate metrics |
| `enterprises/{e}/copilot/metrics/reports/users-1-day` | Existing | Per-user daily metrics |
| `orgs/{o}/copilot/metrics/reports/users-1-day` | Existing | Per-user daily metrics (org scope) |
| `enterprises/{e}/copilot/metrics/reports/user-teams-1-day` | **New** | User-team mapping |
| `orgs/{o}/copilot/metrics/reports/user-teams-1-day` | **New** | User-team mapping (org scope) |
| `orgs/{o}/copilot/billing/seats` | Existing | Seat assignments (expanded fields) |
| `enterprises/{e}/copilot/billing/seats` | Existing | Seat assignments (expanded fields) |

---

## Testing

- All existing unit tests pass (`go test ./plugins/gh-copilot/... -count=1`)
- `TestGetTablesInfo` updated and passing with 19 tables
- `go vet` clean
- Build successful (`go build ./plugins/gh-copilot/...`)
