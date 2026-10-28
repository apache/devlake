# V1 vs V2 Confusion Matrix Discrepancy — Root Cause & Fix Plan

**Status:** In progress  
**Date:** 2026-04-02  

## Problem

TP / TN / FP / FN counts differ between the AiFailurePredictionV1 and AiFailurePredictionV2 Grafana dashboards.
Users would expect them to be identical (same data, different rendering), but they are not.

---

## Data Inventory (as of 2026-04-02)

```
_tool_aireview_failure_predictions  (406 rows total)
  ci_failure_source = NULL          → 122 rows   (orphaned, stale)
  ci_failure_source = 'test_cases'  → 142 rows   (current)
  ci_failure_source = 'job_result'  → 142 rows   (current)

_tool_aireview_reviews              → 1,205 rows
  created_date range: 2025-11-11 → 2026-04-01
```

Confusion matrix for `test_cases` source (current):
```
TP = 32   FP = 89   TN = 15   FN = 6
```

---

## Root Cause 1 — `flagged_at` is never populated (PRIMARY)

**File:** `backend/plugins/aireview/tasks/calculate_failure_predictions.go`

`loadAiReviewPrSummaries` does not select `created_date` from `_tool_aireview_reviews`.
The resulting `AiFailurePrediction` record has `FlaggedAt` left as zero / nil.

`created_at` is set to `time.Now()` — the moment the DevLake task ran.
All 284 current predictions share the same two task-run timestamps (2026-03-31 and 2026-04-02).

**Consequence:**

| Dashboard | Time filter column     | What it means                      |
|-----------|------------------------|------------------------------------|
| V1        | `ar.created_date`      | When the AI review was posted      |
| V2        | `created_at`           | When the DevLake task ran          |

In V2, `$__timeFilter(created_at)` is effectively meaningless: if the Grafana time range includes
the task run date (today), all 284 current rows pass. If it doesn't, zero rows pass.
V1 filters against the real review timeline (Nov 2025 – Apr 2026), giving genuine time-windowed results.

**Fix:** Include `MIN(ar.created_date) AS created_date` in the summary query, store it in `prAiSummary`,
and set `FlaggedAt = ps.CreatedDate` when building each `AiFailurePrediction`.
Then update the V2 dashboard to use `$__timeFilter(flagged_at)`.

---

## Root Cause 2 — Stale orphaned predictions (`ci_failure_source = NULL`)

**When:** 20260330–20260331, before migration `20260331_add_ci_failure_source.go` ran.

`generatePredictionId` hashes `prId + aiTool + ciFailureSource`.
At that time `ciFailureSource` was always `""` (empty string), producing IDs like:

```
aipred:sha256("github:pr-123:CodeRabbit:")
```

After the migration added `CiFailureSource`, new task runs use e.g. `"test_cases"` or `"job_result"`,
producing **different IDs**. The old rows are never matched by `CreateOrUpdate` and accumulate as orphans.

**Consequence in V2 dashboard:**

- Filter `ci_failure_source = 'both'`:  all 406 rows included (122 stale + 284 current).
- Filter `ci_failure_source = 'test_cases'`: only 142 rows (NULLs don't match).

This inflates counts under "both" and makes comparisons across filter values inconsistent.

**Fix:** Migration `20260402_cleanup_null_ci_source_predictions.go` that deletes all rows where
`ci_failure_source IS NULL`. They will be regenerated correctly on the next task run.

---

## Root Cause 3 — 3-month CI lookback vs. Grafana time window (minor)

`loadCiOutcomesByTestCases` and `loadCiOutcomesByJobResult` hard-code a 3-month CI lookback:

```go
dal.Where("... AND j.finished_at >= ?", time.Now().AddDate(0, -3, 0))
```

V1's dashboard uses `$__timeFilter(j.finished_at)`, which respects the user-chosen Grafana time range.
If a user sets a 6-month window in V1, they see more PRs than V2 pre-computed.

This is a design trade-off (performance vs. flexibility). Not addressed in this fix cycle;
the lookback is controlled by `CiBackfillDays` in scope config (added 20260401).

---

## Fix Plan

### Fix 1: Populate `flagged_at` in `calculate_failure_predictions.go`

- Add `CreatedDate time.Time` to `prAiSummary`.
- Extend the SELECT in `loadAiReviewPrSummaries` with `MIN(ar.created_date) AS created_date`.
- Set `FlaggedAt: ps.CreatedDate` in the prediction record.

### Fix 2: Update V2 Grafana dashboard

Replace every occurrence of `$__timeFilter(created_at)` with `$__timeFilter(flagged_at)` in
`grafana/dashboards/AiFailurePredictionV2.json`.

### Fix 3: Add cleanup migration

Create `backend/plugins/aireview/models/migrationscripts/20260402_cleanup_null_ci_source_predictions.go`:

```go
db.Exec("DELETE FROM _tool_aireview_failure_predictions WHERE ci_failure_source IS NULL")
```

Register in `register.go`.

---

## Expected Outcome After Fixes

After next task run:
- All predictions have `flagged_at` set to the AI review's `created_date`.
- V2 dashboard time filter behaves identically to V1 (filters on review date).
- The 122 stale NULL rows are removed; fresh rows with correct `ci_failure_source` are written.
- TP/TN/FP/FN counts in V1 and V2 converge for the same Grafana time range and threshold.

---

## Files Changed

| File | Change |
|------|--------|
| `backend/plugins/aireview/tasks/calculate_failure_predictions.go` | Populate `FlaggedAt` |
| `grafana/dashboards/AiFailurePredictionV2.json` | `created_at` → `flagged_at` in time filter |
| `backend/plugins/aireview/models/migrationscripts/20260402_cleanup_null_ci_source_predictions.go` | New migration |
| `backend/plugins/aireview/models/migrationscripts/register.go` | Register new migration |
