# agentready Plugin — Agent Context

Data-source plugin that collects AI readiness assessments from a GitHub submissions repository, extracts structured findings per attribute, computes aggregated metrics, and exposes results via REST API and Grafana dashboards.

## Build & Test

```bash
cd backend
go test ./plugins/agentready/... -v               # unit tests
golangci-lint run ./plugins/agentready/...         # lint
```

Single-file verification: `go vet ./plugins/agentready/...`

No e2e tests yet — data is validated via unit tests and manual Grafana dashboard checks.

## Layout

- `impl/impl.go` — plugin interfaces (PluginMeta, PluginInit, PluginApi, PluginModel, PluginMigration, PluginTask, DataSourcePluginBlueprintV200)
- `models/` — tool-layer models (connection, scope, scope_config, assessment, finding, metric) + `migrationscripts/register.go`
- `tasks/` — 3-stage pipeline: collector → extractor → calculator
- `tasks/task_data.go` — shared `AgentReadyOptions` and `AgentReadyTaskData` structs
- `api/` — REST endpoints (connections, scopes, scope-configs, remote-scopes, assessments, stats, blueprints)
- `grafana/` — 3 dashboards (fleet-overview, findings-analysis, repo-detail)

## Conventions

- This is a **data-source plugin**: has Connection, Scope (`org/repo`), ScopeConfig models
- Implements `DataSourcePluginBlueprintV200` for blueprint-driven collection
- Subtask execution order: `collectSubmissions` → `extractAssessments` → `calculateMetrics` (see `SubTaskMetas()`)
- `FullName` format: `"org/repo"` — derived from the GitHub submissions repo tree structure
- Composite primary keys: `(id, connection_id)` on assessments, findings, and metrics tables
- GitHub delegation: references a GitHub connection ID for API auth; does not store credentials itself
- Default branch auto-detection: `PrepareTaskData()` resolves the actual default branch from the GitHub API

## Don'ts

- Don't add models to `GetTablesInfo()` without a migration script in `migrationscripts/register.go`
- Don't import from other plugins (plugins must be independent)
- Don't skip the Apache 2.0 license header on new files
- Don't hardcode branch names — use the auto-detected branch from `PrepareTaskData()`
- Don't store GitHub credentials in the connection model — reference the GitHub connection ID instead

## Pattern References

| Change Type | Example File |
|---|---|
| Add new subtask | `tasks/submissions_collector.go` + register in `impl/impl.go:SubTaskMetas()` |
| Add migration | `models/migrationscripts/20260609_add_composite_primary_keys.go` |
| Add model field | model file + migration + update `GetTablesInfo()` |
| Add API endpoint | `api/assessments.go`, register in `impl/impl.go:ApiResources()` |
| Add Grafana dashboard | `grafana/fleet-overview.json` |

## Skills

- **PR Definition of Done**: see [skills/pr-definition-of-done/SKILL.md](skills/pr-definition-of-done/SKILL.md)
