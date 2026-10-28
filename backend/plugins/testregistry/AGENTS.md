# testregistry Plugin — Agent Context

Data-source plugin that collects CI test results from Openshift CI (Prow) and Tekton CI, fetches JUnit XML artifacts from GCS/Quay/ORAS, and stores normalized test suites/cases for quality tracking dashboards.

## Build & Test

```bash
cd backend
go test ./plugins/testregistry/... -v          # unit tests
golangci-lint run ./plugins/testregistry/...   # lint
```

Single-file verification: `go vet ./plugins/testregistry/...`

No e2e tests yet — Prow/GCS integration tested via unit tests and manual runs.

## Layout

- `impl/impl.go` — plugin interfaces (PluginSource, DataSourcePluginBlueprintV200)
- `models/` — connection, scope, scope_config, ci_job, test_suite, test_case, tekton_task + `migrationscripts/register.go`
- `tasks/prow_collector.go` — Prow job collection with retry logic (502/503/504/429)
- `tasks/tekton_collector.go` — Tekton pipeline run collection
- `tasks/gcs_client.go` — GCS bucket access for JUnit XML artifacts
- `tasks/quay_client.go` — Quay.io ORAS artifact access
- `tasks/junit-processor.go` — JUnit XML parsing
- `tasks/task_data.go` — options, task data, JUnit regex configuration
- `api/` — REST endpoints (connections, scopes, scope-configs, remote-scopes)

## Conventions

- Connection model has `CITool` field: `"Openshift CI"` or `"Tekton CI"` — collectors check this and skip if wrong type
- JUnit regex is configurable per-connection (`JUnitRegex` field) with a compiled default
- Prow API returns all jobs; filtering by org/repo happens client-side in `matchesScope()`
- Prow retries: 5 attempts with exponential backoff (10s base) for transient HTTP errors
- GitHub token in connection is encrypted via `serializer:encdec` tag

## Don'ts

- Don't add models to `GetTablesInfo()` without a migration script in `migrationscripts/register.go`
- Don't import from other plugins (plugins must be independent)
- Don't skip the Apache 2.0 license header on new files
- Don't store secrets in plain text — use `serializer:encdec` gorm tag
- Don't call Prow API without retry logic — the endpoint is unreliable

## Pattern References

| Change Type | Example File |
|---|---|
| Add new CI source | `tasks/tekton_collector.go` (follow Prow pattern) |
| Add migration | `models/migrationscripts/20250113_add_junit_regex_column.go` |
| Add model | `models/test_case.go` + migration + update `GetTablesInfo()` |
| Add API endpoint | `api/connection.go`, register in `impl/impl.go:ApiResources()` |
| Add artifact source | `tasks/quay_client.go` (follow GCS client pattern) |

## Skills

- **PR Definition of Done**: see [skills/pr-definition-of-done/SKILL.md](skills/pr-definition-of-done/SKILL.md)
