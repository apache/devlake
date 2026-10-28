# aisdlc Plugin — Agent Context

Data-source plugin that collects AISDLC station artifacts from GitLab repositories and stores shared observations. The first parsers cover RFE Creator (`rfe-assess-data`) and Strategy Creator (`strat-pipeline-data`).

## Build & Test

```bash
cd backend
go test ./plugins/aisdlc/... -v
golangci-lint run ./plugins/aisdlc/...
```

Single-file verification: `go vet ./plugins/aisdlc/...`

## Layout

- `impl/impl.go` — plugin interfaces
- `models/` — connection, scope, scope config, observations, and `migrationscripts/register.go`
- `tasks/` — `collectFiles` → `extractObservations` → `linkWorkItems`
- `tasks/parse/` — pure parsers and identity rules
- `tasks/testdata/` — small fixtures taken from the two GitLab repos
- `api/` — connections, scopes, scope configs, remote scopes, blueprints

## History mode

Both repositories keep immutable timestamped batch directories on the branch tip. Collect that tree. Do not walk git history.

`RHAIRFE/current` and `RHAISTRAT/current` are symlinks to the latest batch. Skip any path segment named `current` so the batch is stored once, under its timestamp directory.

Observation identity is `source_id + blob sha + path + field selector + observation_kind`. A later commit that does not change the blob updates `collected_at` and does not add a row. `collected_at` is the time `collectFiles` last saw the blob in the tree. Extraction copies that time, so a raw file that has left the tree keeps its previous stamp.

## Field mapping

Configured on the scope, never inferred from the repo name: `tenant_id`, `source_id`, `station_id`, `parser` (`rfe_assess` or `strat_pipeline`).

`rfe_assess` reads `*.result.md` only. `scores.csv` repeats those totals and is ignored. Each file is one `assessment` observation for the Jira RFE key in the filename. The batch folder is a timestamp with an unspecified timezone, not a CI execution, so no execution row is created. Attempt, pipeline id, and consumed revision stay unknown.

`strat_pipeline` reads `pipeline-data.json` when the batch has one. Each `strategies[]` entry is one `processing_result` observation. A score on that entry stays on the same observation. `source_rfe` is an input link (`source_work_item`), not a causal handoff. The file's `timestamp` identifies a batch execution whose id is the batch folder. `run_id: "artifacts"` is not unique and is not used as the execution id. Pipeline id and attempt stay unknown. Timezone-free timestamps are not normalized.

Batches with no `pipeline-data.json` use `strat-tasks/*.md` front matter. Those files do not identify a run, so they do not create an execution. `jira_key: null` leaves the subject unconfirmed.

`summary.json`, reviews, originals, reports, and `claude-otel.jsonl` are not collected.

## Conventions

- Connection stores `gitlab_connection_id` only. The token is read from `_tool_gitlab_connections`.
- Do not import `plugins/gitlab`.
- Do not insert `issues` or `issue_relationships`. `linkWorkItems` sets `domain_issue_id` only when `issues.issue_key` matches exactly one row.
- Project-scoped tool rows set `_raw_data_params` from the scope (`connectionId` and `fullName`) so deleting a scope removes them. Work items are shared by the connection and stay unstamped.
- Apache 2.0 license header on every new `.go` file.

## Don'ts

- Don't add a model to `GetTablesInfo()` without a migration script.
- Don't treat a folder name as an attempt.
- Don't convert a date-only value to midnight UTC.
- Don't store GitLab credentials on the aisdlc connection.
