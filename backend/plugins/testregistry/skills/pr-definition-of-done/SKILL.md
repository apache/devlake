---
description: >-
  Checklist for merge-ready PRs touching the testregistry plugin.
  Use when preparing, reviewing, or finalizing a PR for testregistry.
---

# PR Definition of Done — testregistry Plugin

## Required for All PRs

- [ ] Changes scoped to `backend/plugins/testregistry/`
- [ ] `go vet ./plugins/testregistry/...` passes
- [ ] `go test ./plugins/testregistry/... -v` passes
- [ ] `golangci-lint run ./plugins/testregistry/...` passes (CI only runs lint on main, so check locally)
- [ ] No `//nolint` without justification comment
- [ ] Apache 2.0 license header on every new `.go` file

## Schema / Model Changes

- [ ] Migration script in `models/migrationscripts/` named `YYYYMMDD_description.go`
- [ ] Migration registered in `migrationscripts/register.go:All()`
- [ ] New models added to `GetTablesInfo()` in `impl/impl.go`
- [ ] Table name follows `_tool_testregistry_<entity>` convention

## New Subtasks

- [ ] Registered in `impl/impl.go:SubTaskMetas()` in correct execution order

## Plugin-Specific

- [ ] `CITool` field checked in collector entry point (skip if wrong CI tool type)
- [ ] Prow retry logic preserved (5 attempts, exponential backoff, transient status codes)
- [ ] Secrets use `serializer:encdec` gorm tag; sanitized in API responses
- [ ] JUnit regex changes tested with both default and custom patterns

## CI Behavior

- **Unit tests**: Run on PRs via `make unit-test-go` (excludes `e2e/` and `models/` packages)
- **E2E tests**: No e2e tests exist yet; adding `testregistry/e2e/` would be auto-discovered by CI
- **Lint**: Runs only on push to main/tags, not on PRs
- Upstream CI also runs `plugins/table_info_test.go` — fails if `GetTablesInfo()` is incomplete
