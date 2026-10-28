# AI Review Plugin

## Overview

The AI Review plugin extracts and analyzes AI-generated code reviews from pull requests to calculate the "AI Predicted Failure Avoidance" metric. This metric measures whether AI code review tools can effectively predict and flag risky code changes before they reach production.

## Plugin Architecture

This is a **metric/transformer plugin** (not a data source plugin):

- Works on domain layer data from GitHub/GitLab plugins
- Implements `MetricPluginBlueprintV200` interface
- Configured via Projects, not Connections
- Runs after data collection plugins

### Directory Structure

```
plugins/aireview/
├── aireview.go                    # Entry point (standalone debugging)
├── impl/
│   └── impl.go                    # Plugin implementation (interfaces)
├── models/
│   ├── ai_review.go               # AiReview model
│   ├── ai_commit.go               # AiCommit model (AI-assisted commits)
│   ├── ai_review_finding.go       # AiReviewFinding model
│   ├── ai_failure_prediction.go   # Prediction models
│   ├── scope_config.go            # Configurable patterns
│   └── migrationscripts/
│       └── init_schema.go         # Database migrations
├── tasks/
│   ├── task_data.go               # Task options and validation
│   ├── extract_ai_reviews.go      # Extract AI reviews from comments
│   ├── extract_ai_commits.go      # Classify AI-assisted commits
│   ├── convert_ai_commits.go
│   ├── extract_ai_review_findings.go
│   ├── calculate_failure_predictions.go
│   ├── calculate_prediction_metrics.go
│   └── extract_ai_reviews_test.go # Unit tests
└── e2e/
    ├── aireview_test.go           # E2E tests
    └── raw_tables/                # Test fixtures (CSVs)
```

### Interfaces Implemented

```go
var _ interface {
    plugin.PluginMeta
    plugin.PluginTask
    plugin.PluginModel
    plugin.PluginMetric
    plugin.PluginMigration
    plugin.MetricPluginBlueprintV200
} = (*AiReview)(nil)
```

## Features

- **Multi-platform support**: Works with both GitHub PRs and GitLab MRs
- **Multi-tool support**: Currently supports CodeRabbit, with extensibility for Cursor Bugbot, SonarQube, and other AI review tools
- **AI-assisted commit classification**: Precomputes which commits were authored with Cursor/Claude/Copilot/CodeRabbit (from git trailers) into `ai_commits`.
- **Per-team configuration**: Teams can configure which AI tools they use and customize detection patterns
- **Prediction accuracy tracking**: Tracks AI predictions against actual outcomes (CI failures, bugs, rollbacks)
- **Metrics calculation**: Computes precision, recall, accuracy, and F1 scores for AI predictions

## Metrics

### AI Predicted Failure Avoidance

This metric uses a confusion matrix approach:

| Metric | Formula | Target |
|--------|---------|--------|
| Precision | TP / (TP + FP) | > 70% |
| Recall | TP / (TP + FN) | > 65% |
| Accuracy | (TP + TN) / Total | > 80% |

Where:
- **True Positive (TP)**: AI flagged as risky AND failure occurred
- **False Positive (FP)**: AI flagged as risky AND no failure occurred
- **False Negative (FN)**: AI didn't flag AND failure occurred
- **True Negative (TN)**: AI didn't flag AND no failure occurred

### Autonomy Level Recommendations

Based on prediction accuracy, the plugin recommends AI autonomy levels:

| Precision | Recall | Recommendation |
|-----------|--------|----------------|
| > 80% | > 70% | Auto-block risky PRs |
| 60-80% | 50-70% | Mandatory human review |
| < 60% | < 50% | Advisory only |

## Data Models

### AiReview
Stores AI-generated reviews with metadata:
- Review body and summary
- Risk assessment (level, score, confidence)
- Metrics (issues found, suggestions, files reviewed)
- Effort estimation (complexity, time)

### AiReviewFinding
Individual findings from AI reviews:
- Category (security, performance, bug, style, etc.)
- Severity (info, warning, error, critical)
- Code location and suggested fixes
- Resolution tracking

### AiFailurePrediction
Tracks prediction outcomes:
- Was the PR flagged as risky?
- Did CI fail after merge?
- Were bugs reported within the observation window?
- Was the change rolled back?

### AiPredictionMetrics
Aggregated metrics over time periods:
- Confusion matrix counts
- Precision, recall, accuracy, F1 score
- Recommended autonomy level

## Configuration

### Scope Config Options

```json
{
  "codeRabbitEnabled": true,
  "codeRabbitUsername": "coderabbitai",
  "codeRabbitPattern": "(?i)(coderabbit|walkthrough|summary by coderabbit)",
  "cursorBugbotEnabled": false,
  "cursorBugbotUsername": "cursor-bugbot",
  "cursorBugbotPattern": "(?i)(cursor|bugbot)",
  "riskHighPattern": "(?i)(critical|security|breaking|major)",
  "riskMediumPattern": "(?i)(warning|medium|moderate)",
  "riskLowPattern": "(?i)(minor|low|info|suggestion)",
  "observationWindowDays": 14,
  "bugLinkPattern": "(?i)(fixes|closes|resolves)\\s*#(\\d+)"
}
```

## Usage

### Prerequisites

This plugin requires data from either the GitHub or GitLab plugin. Ensure you have:
1. Collected PR/MR data using the respective plugin
2. PR comments are collected (including bot comments)

### Running the Plugin

The plugin runs as part of the DevLake pipeline. Configure it in your blueprint:

```json
{
  "plugin": "aireview",
  "options": {
    "repoId": "github:GithubRepo:1:12345",
    "scopeConfig": {
      "codeRabbitEnabled": true,
      "observationWindowDays": 14
    }
  }
}
```

### Standalone Debugging

```bash
go run plugins/aireview/aireview.go \
  --repoId="github:GithubRepo:1:12345" \
  --codeRabbitUsername="coderabbitai" \
  --timeAfter="2024-01-01T00:00:00Z"
```

## Subtasks

1. **extractAiReviews**: Identifies and extracts AI-generated reviews from PR comments
2. **extractAiReviewFindings**: Parses reviews to extract individual findings
3. **calculateFailurePredictions**: Tracks prediction outcomes against actual failures
4. **calculatePredictionMetrics**: Aggregates data into precision/recall metrics

## Database Tables

- `_tool_aireview_reviews`: AI review records
- `_tool_aireview_findings`: Individual findings from reviews
- `_tool_aireview_failure_predictions`: Prediction outcome tracking
- `_tool_aireview_prediction_metrics`: Aggregated metrics
- `_tool_aireview_scope_configs`: Per-scope configuration
- `_tool_aireview_commits`: Sparse AI-assisted commit classifications
- `ai_commits`: Project-scoped domain table for Grafana

## Extending for New AI Tools

To add support for a new AI review tool:

1. Add configuration fields to `AiReviewScopeConfig`:
   ```go
   NewToolEnabled  bool   `gorm:"type:boolean"`
   NewToolUsername string `gorm:"type:varchar(255)"`
   NewToolPattern  string `gorm:"type:varchar(500)"`
   ```

2. Update `CompilePatterns()` in `task_data.go` to compile the new patterns

3. Update `detectAiTool()` in `extract_ai_reviews.go` to check for the new tool

4. Add tool-specific parsing in `parseFindings()` if the tool has a unique format

## Related Metrics

This plugin is part of the AI Quality Metrics Framework and provides foundational data for:
- AI Value Delivery tracking
- Developer productivity metrics
- Code quality trends
- Review efficiency analysis
