# AI Review Metrics Reference

This document describes all metrics collected and calculated by the AI Review plugin.

## Data Tables

### `_tool_aireview_reviews`

Stores extracted AI-generated code reviews from pull request comments.

| Column | Type | Description |
|--------|------|-------------|
| `id` | string | Unique review ID (hash-based) |
| `pull_request_id` | string | Domain layer PR ID |
| `repo_id` | string | Domain layer repository ID |
| `ai_tool` | string | AI tool identifier (e.g., `coderabbit`, `cursor-bugbot`) |
| `ai_tool_user` | string | Username/account of the AI bot |
| `review_id` | string | Original comment ID |
| `body` | text | Full review comment body |
| `summary` | string | Extracted summary (max 500 chars) |
| `created_date` | datetime | When the review was posted |
| `risk_level` | string | Detected risk level: `high`, `medium`, `low` |
| `risk_score` | int | Numeric risk score (0-100) |
| `risk_confidence` | int | Confidence in risk assessment (0-100) |
| `issues_found` | int | Number of issues detected in review |
| `suggestions_count` | int | Number of suggestions made |
| `files_reviewed` | int | Number of files mentioned |
| `lines_reviewed` | int | Lines of code reviewed |
| `effort_complexity` | string | Complexity level: `trivial`, `simple`, `moderate`, `complex` |
| `effort_minutes` | int | Estimated review effort in minutes |
| `review_state` | string | Review outcome: `approved`, `changes_requested`, `commented` |
| `source_platform` | string | Source platform: `github`, `gitlab` |
| `source_url` | string | URL to the pull request |

### `_tool_aireview_commits`

Sparse table of commits classified as AI-assisted. Only AI-positive commits are stored.
Bot identity is **not** stored here — join `accounts.is_bot`.

| Column | Type | Description |
|--------|------|-------------|
| `id` | string | Unique ID (hash of `repo_id:sha`) |
| `commit_sha` | string | Commit SHA |
| `repo_id` | string | Domain layer repository ID |
| `ai_tool` | string | `cursor`, `claude`, `copilot`, `coderabbit`, `assisted_by_unknown`, `made_with_unknown`, `other` |
| `author_name` | string | Git author name |
| `authored_date` | datetime | Commit authored date |

### `ai_commits`

Project-scoped domain copy of `_tool_aireview_commits` (same columns plus `project_name`). Grafana and n8n should join this table, not `_tool_aireview_commits`.

After an aireview **project** pipeline run:

- `aiAssisted` / `aiToolUsed` = `LEFT JOIN ai_commits` (use `ai_tool` as stored; do not remap in SQL)
- `bot` = `accounts.is_bot` and not in `ai_commits`
- `human` = neither of the above, excluding merge-commit message prefixes (`Merge pull request #`, `Merge branch `, `Merge remote-tracking branch `)
- Merge commits are not bots. Join `accounts` on `email` / `user_name` (`commits.author_id` is a git email, not `accounts.id`).

### `_tool_aireview_findings`

Individual issues, suggestions, or observations extracted from reviews.

| Column | Type | Description |
|--------|------|-------------|
| `id` | string | Unique finding ID |
| `review_id` | string | Parent AI review ID |
| `pull_request_id` | string | Associated PR ID |
| `repo_id` | string | Repository ID |
| `category` | string | Finding category (see below) |
| `severity` | string | Severity: `critical`, `major`, `minor`, `info` |
| `title` | string | Brief finding title |
| `description` | text | Full finding description |
| `file_path` | string | Affected file path |
| `line_start` | int | Starting line number |
| `line_end` | int | Ending line number |
| `suggestion` | text | Suggested fix or improvement |
| `is_resolved` | bool | Whether the finding was addressed |

#### Finding Categories

| Category | Description |
|----------|-------------|
| `security` | Security vulnerabilities, authentication issues |
| `performance` | Performance problems, inefficiencies |
| `bug` | Logic errors, potential bugs |
| `style` | Code style, formatting issues |
| `documentation` | Missing or incorrect documentation |
| `testing` | Test coverage, test quality |
| `maintainability` | Code complexity, readability |

### `_tool_aireview_failure_predictions`

Tracks AI prediction accuracy against actual outcomes.

| Column | Type | Description |
|--------|------|-------------|
| `id` | string | Unique prediction ID |
| `review_id` | string | Source AI review ID |
| `pull_request_id` | string | Associated PR ID |
| `repo_id` | string | Repository ID |
| `predicted_risk` | string | AI predicted risk level |
| `predicted_score` | int | AI predicted risk score |
| `actual_outcome` | string | What actually happened |
| `outcome_type` | string | `TP`, `FP`, `FN`, `TN` (confusion matrix) |
| `merged_date` | datetime | When PR was merged |
| `failure_date` | datetime | When failure occurred (if any) |
| `observation_window_days` | int | Days after merge to observe |

#### Outcome Types

| Type | Description |
|------|-------------|
| `TP` | True Positive - AI predicted risk, failure occurred |
| `FP` | False Positive - AI predicted risk, no failure |
| `FN` | False Negative - AI missed risk, failure occurred |
| `TN` | True Negative - AI predicted safe, no failure |

### `_tool_aireview_prediction_metrics`

Aggregated prediction accuracy metrics.

| Column | Type | Description |
|--------|------|-------------|
| `id` | string | Unique metrics ID |
| `repo_id` | string | Repository ID |
| `time_period` | string | Aggregation period (e.g., `2024-01`) |
| `total_predictions` | int | Total predictions made |
| `true_positives` | int | TP count |
| `false_positives` | int | FP count |
| `false_negatives` | int | FN count |
| `true_negatives` | int | TN count |
| `precision` | float | TP / (TP + FP) |
| `recall` | float | TP / (TP + FN) |
| `f1_score` | float | 2 * (precision * recall) / (precision + recall) |
| `accuracy` | float | (TP + TN) / total |

## Calculated Metrics

### Risk Level Detection

Risk levels are detected by matching patterns in review content:

| Risk Level | Score | Trigger Patterns |
|------------|-------|------------------|
| High | 80 | `critical`, `security`, `vulnerability`, `breaking` |
| Medium | 50 | `warning`, `caution`, `potential`, `moderate` |
| Low | 20 | `minor`, `suggestion`, `consider`, `nitpick` |
| Default | 10 | No risk patterns matched |

### Effort Estimation

Effort is estimated from complexity keywords:

| Complexity | Estimated Minutes |
|------------|-------------------|
| Trivial | 5 |
| Simple | 5 |
| Moderate | 15 |
| Complex | 30 |

If explicit time mentions are found (e.g., "~12 minutes"), those override the estimate.

### Review State Detection

Review state is determined from content and status:

| State | Detection Method |
|-------|------------------|
| `approved` | Contains "approved", "LGTM", or status is APPROVED |
| `changes_requested` | Contains "changes requested" or status is CHANGES_REQUESTED |
| `commented` | Default state for informational reviews |

## Grafana Dashboard Queries

### Total Reviews by Risk Level

```sql
SELECT risk_level, COUNT(*) as count
FROM _tool_aireview_reviews
GROUP BY risk_level
```

### Reviews Over Time

```sql
SELECT
  DATE(created_date) as time,
  risk_level,
  COUNT(*) as count
FROM _tool_aireview_reviews
WHERE created_date >= DATE_SUB(NOW(), INTERVAL 90 DAY)
GROUP BY DATE(created_date), risk_level
ORDER BY time
```

### AI Prediction Accuracy

```sql
SELECT
  time_period,
  precision,
  recall,
  f1_score,
  accuracy
FROM _tool_aireview_prediction_metrics
WHERE repo_id = 'your-repo-id'
ORDER BY time_period
```

### High Risk Reviews Requiring Attention

```sql
SELECT
  created_date,
  ai_tool,
  summary,
  issues_found,
  source_url
FROM _tool_aireview_reviews
WHERE risk_level = 'high'
  AND review_state != 'approved'
ORDER BY created_date DESC
LIMIT 20
```
