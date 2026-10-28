# codecov Plugin — Agent Context

Data-source plugin that collects code coverage metrics from Codecov's API (flags, commits, coverages, comparisons, trends) and converts them into DevLake's domain layer for Grafana dashboards.

## Build & Test

```bash
cd backend
go test ./plugins/codecov/... -v               # unit tests
golangci-lint run ./plugins/codecov/...         # lint
```

Single-file verification: `go vet ./plugins/codecov/...`

Playwright e2e (requires `CODECOV_TOKEN`):

```bash
cd e2e
export CODECOV_TOKEN=...   # Codecov API token for konflux-ci org
npm run test:codecov
```

Exercises the full plugin lifecycle (connection → scope → blueprint → pipeline with all 13 subtasks). Runs in CI via `.github/workflows/test-e2e-coverage.yml` to drive Go coverage under the `e2e-go` flag.

## Layout

- `impl/impl.go` — plugin interfaces (PluginSource, DataSourcePluginBlueprintV200)
- `models/` — tool-layer models (connection, repo, flag, commit, coverage, comparison, trend) + `migrationscripts/register.go`
- `tasks/` — collector → extractor → converter pipeline for each entity
- `tasks/helpers.go` — shared utilities (`ParseFullName`)
- `api/` — REST endpoints (connections, scopes, scope-configs, remote-scopes, blueprints)
- `docs/` — user-facing documentation

## Conventions

- This is a **data-source plugin**: has Connection, Scope (repo), ScopeConfig models
- Implements `DataSourcePluginBlueprintV200` for blueprint-driven collection
- Subtask execution order: flags → commits → coverage data → converters (see `SubTaskMetas()`)
- API rate limit: 5000 req/hour hardcoded in `PrepareTaskData()`
- `FullName` format: `"owner/repo"` — parsed via `tasks.ParseFullName()`
- Branch auto-detection: `PrepareTaskData()` fetches default branch from Codecov API

## Don'ts

- Don't add models to `GetTablesInfo()` without a migration script in `migrationscripts/register.go`
- Don't import from other plugins (plugins must be independent)
- Don't skip the Apache 2.0 license header on new files
- Don't hardcode branch names — use the auto-detected branch from `PrepareTaskData()`

## Pattern References

| Change Type | Example File |
|---|---|
| Add new API collector | `tasks/commits_collector.go` + matching extractor/converter |
| Add migration | `models/migrationscripts/20260316000000_add_line_counts_to_commit_coverages.go` |
| Add model field | model file + migration + update `GetTablesInfo()` |
| Add API endpoint | `api/connection_api.go`, register in `impl/impl.go:ApiResources()` |
| Add converter | `tasks/coverage_converter.go`, register in `impl/impl.go:SubTaskMetas()` |

## Skills

- **PR Definition of Done**: see [skills/pr-definition-of-done/SKILL.md](skills/pr-definition-of-done/SKILL.md)
