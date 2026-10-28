---
description: >-
  Checklist for merge-ready PRs touching the codecov plugin.
  Use when preparing, reviewing, or finalizing a PR for codecov.
---

# PR Definition of Done — codecov Plugin

## Required for All PRs

- [ ] Changes scoped to `backend/plugins/codecov/`
- [ ] `go vet ./plugins/codecov/...` passes
- [ ] `go test ./plugins/codecov/... -v` passes
- [ ] `golangci-lint run ./plugins/codecov/...` passes (CI only runs lint on main, so check locally)
- [ ] No `//nolint` without justification comment
- [ ] Apache 2.0 license header on every new `.go` file

## Schema / Model Changes

- [ ] Migration script in `models/migrationscripts/` named `YYYYMMDD_description.go`
- [ ] Migration registered in `migrationscripts/register.go:All()`
- [ ] New models added to `GetTablesInfo()` in `impl/impl.go`
- [ ] Table name follows `_tool_codecov_<entity>` convention

## New Subtasks

- [ ] Registered in `impl/impl.go:SubTaskMetas()` in correct execution order
- [ ] Collectors before extractors before converters

## Plugin-Specific

- [ ] `FullName` format validated as `"owner/repo"` (use `tasks.ParseFullName`)
- [ ] Rate limiter unchanged at 5000 req/hour unless Codecov plan warrants it
- [ ] Branch auto-detection in `PrepareTaskData()` not broken

## CI Behavior

- **Unit tests**: Run on PRs via `make unit-test-go` (excludes `e2e/` and `models/` packages)
- **E2E tests**: Playwright spec at `e2e/codecov-flow.spec.ts` runs in `test-e2e-coverage` workflow (requires `CODECOV_TOKEN` secret)
- **Lint**: Runs only on push to main/tags, not on PRs
- Upstream CI also runs `plugins/table_info_test.go` — fails if `GetTablesInfo()` is incomplete
