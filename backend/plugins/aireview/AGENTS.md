# aireview Plugin — Agent Context

Metric/transformer plugin that extracts AI-generated code reviews from GitHub/GitLab PR comments, tracks prediction accuracy against CI outcomes, and computes precision/recall/F1 for AI review tools (CodeRabbit, Qodo, Gemini, Cursor Bugbot).

## Build & Test

```bash
cd backend
go test ./plugins/aireview/... -v              # unit tests
go test ./plugins/aireview/e2e/... -v           # e2e tests (needs MySQL + lake_test)
golangci-lint run ./plugins/aireview/...        # lint
```

Single-file verification: `go vet ./plugins/aireview/...`

## Layout

- `impl/impl.go` — plugin interfaces (PluginMeta, PluginTask, PluginMetric, MetricPluginBlueprintV200)
- `models/` — tool-layer models + `migrationscripts/register.go` (all migrations listed in `All()`)
- `models/scope_config.go` — per-team regex patterns for AI tool detection and risk classification
- `tasks/` — subtask pipeline: extract reviews → extract AI commits → enrich reactions → findings → convert → match diffs → fetch CI → predict → metrics
- `api/` — REST endpoints (reviews, findings, stats, scope-configs, analyze)
- `e2e/raw_tables/` — CSV fixtures for e2e tests

## Conventions

- This is a **metric plugin** (not data-source): no Connection model, configured via Projects
- Implements `MetricPluginBlueprintV200`; runs *after* github/gitlab plugins
- Subtask order matters: see `SubTaskMetas()` in `impl/impl.go`
- All regex patterns are compiled once in `tasks.CompilePatterns()` and stored in `AiReviewTaskData`
- New AI tool support: add fields to `AiReviewScopeConfig`, update `CompilePatterns()`, update `detectAiTool()`
- AI-assisted **commits** are classified by `detectAiCommit()` (trailer matchers + optional `AiCommitPatterns`) 

## Don'ts

- Don't add models to `GetTablesInfo()` without a migration script in `migrationscripts/register.go`
- Don't import from other plugins (plugins must be independent)
- Don't skip the Apache 2.0 license header on new files
- Don't use `strings.ToLower()` for case-insensitive matching — use `strings.EqualFold()` or `(?i)` regex flag

## Pattern References

| Change Type | Example File |
|---|---|
| Add new AI tool support | `models/scope_config.go`, `tasks/extract_ai_reviews.go`, `tasks/detect_ai_commit.go` |
| Add migration | `models/migrationscripts/20260415_add_flaky_infra_filters.go` |
| Add subtask | `tasks/calculate_failure_predictions.go`, then register in `impl/impl.go:SubTaskMetas()` |
| Add e2e test | `e2e/aireview_test.go` + CSV fixtures in `e2e/raw_tables/` |
| Add API endpoint | `api/reviews.go`, register in `impl/impl.go:ApiResources()` |

## Skills

- **PR Definition of Done**: see [skills/pr-definition-of-done/SKILL.md](skills/pr-definition-of-done/SKILL.md)
