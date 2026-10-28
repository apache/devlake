---
description: >-
  Checklist for merge-ready PRs touching the aireview plugin.
  Use when preparing, reviewing, or finalizing a PR for aireview.
---

# PR Definition of Done — aireview Plugin

## Required for All PRs

- [ ] Changes scoped to `backend/plugins/aireview/`
- [ ] `go vet ./plugins/aireview/...` passes
- [ ] `go test ./plugins/aireview/... -v` passes (unit tests)
- [ ] `golangci-lint run ./plugins/aireview/...` passes (CI only runs lint on main, so check locally)
- [ ] No `//nolint` without justification comment
- [ ] Apache 2.0 license header on every new `.go` file

## Schema / Model Changes

- [ ] Migration script in `models/migrationscripts/` named `YYYYMMDD_description.go`
- [ ] Migration registered in `migrationscripts/register.go:All()`
- [ ] New models added to `GetTablesInfo()` in `impl/impl.go`
- [ ] Table name follows `_tool_aireview_<entity>` convention

## New Subtasks

- [ ] Registered in `impl/impl.go:SubTaskMetas()` in correct execution order
- [ ] `SubTaskMeta` has `Name`, `EntryPoint`, `EnabledByDefault`, `Description`, `DomainTypes`

## Plugin-Specific

- [ ] New AI tool patterns added to `AiReviewScopeConfig` and `CompilePatterns()`
- [ ] `detectAiTool()` updated if new review-tool support added
- [ ] `detectAiCommit()` updated if commit-trailer detection changes
- [ ] Scope config defaults updated in `GetDefaultScopeConfig()`
- [ ] E2E CSV fixtures updated if domain output changed (`e2e/raw_tables/`)
- [ ] New domain tables: model in `backend/core/models/domainlayer/`, migration in `backend/core/models/migrationscripts/`, registered in `domaininfo.go`

## CI Behavior

- **Unit tests**: Run on PRs via `make unit-test-go` (excludes `e2e/` and `models/` packages)
- **E2E tests**: Run only on push to main via `make e2e-test-go-plugins` (auto-discovers `e2e/` dirs)
- **Lint**: Runs only on push to main/tags, not on PRs
- Upstream CI also runs `plugins/table_info_test.go` — fails if `GetTablesInfo()` is incomplete
