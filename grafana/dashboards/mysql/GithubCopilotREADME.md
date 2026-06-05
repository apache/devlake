<!--
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->
# GitHub Copilot Dashboards

Grafana dashboards for analyzing GitHub Copilot usage and its correlation with developer productivity metrics.

## Dashboards

### 1. GitHub Copilot Adoption Dashboard (`GithubCopilotAdoption.json`)

**UID**: `copilot_adoption`

Tracks GitHub Copilot usage metrics across your organization:
- Active users and seat utilization
- Language breakdown of GitHub Copilot activity
- IDE distribution (VS Code, JetBrains, Neovim, etc.)
- Acceptance rates for code suggestions
- Chat and PR summary feature usage

### 2. GitHub Copilot Impact Dashboard (`GithubCopilotImpact.json`)

**UID**: `copilot_impact`

Correlates GitHub Copilot adoption with DORA metrics and engineering productivity:

#### Key Features

- **Correlation-First Analysis**: No implementation date required! Dashboard automatically correlates GitHub Copilot adoption intensity with productivity metrics.
- **Adoption Tier Comparison**: Groups metrics by adoption level (<25%, 25-50%, 50-75%, >75%)
- **Dual-Axis Charts**: See GitHub Copilot adoption trends alongside each DORA metric
- **Pearson Correlation Coefficients**: Statistical correlation (r) values for each metric pair
- **Optional Rollout Milestone**: Annotate specific dates when GitHub Copilot was rolled out to different teams

#### Panels by Section

| Section | Metrics Tracked |
|---------|-----------------|
| Correlation Overview | Adoption trend, aggregate correlation, current adoption % |
| PR Velocity Impact | PR cycle time, coding time, pickup time, review time, PR throughput |
| Deployment Frequency | Deploys per week, correlation with adoption |
| Change Failure Rate | CFR %, correlation (negative r = improvement) |
| Recovery Time (MTTR) | Mean time to recovery, adoption tier comparison |
| Code Review Time | Review time by adoption tier, trend analysis |
| Code Quality | Optional (requires SonarQube): complexity, coverage, duplicates |

#### Correlation Interpretation

The dashboard uses Pearson correlation coefficients (r):

| Value | Interpretation |
|-------|----------------|
| r > 0.7 | Strong positive correlation |
| 0.3 < r < 0.7 | Moderate correlation |
| r < 0.3 | Weak correlation |
| r < 0 | Negative correlation (for CFR/MTTR, negative = improvement) |

**Note**: For failure-related metrics (CFR, MTTR, Review Time), **negative** correlations are desirable - they indicate that higher GitHub Copilot adoption correlates with fewer failures or faster resolution.

## Prerequisites

These dashboards require:

1. **GitHub Copilot Plugin** (`gh-copilot`) - Configured and collecting data
2. **GitHub Plugin** - For PR metrics (`project_pr_metrics` table)
3. **DORA Metrics** - Deployments and incidents data from your CI/CD tools

## Configuration

### Variables

Both dashboards use these template variables:

| Variable | Description |
|----------|-------------|
| `connection_id` | DevLake GitHub Copilot connection |
| `scope_id` | Enterprise/Organization scope |
| `project` | DevLake project filter |

### Optional: Rollout Milestone

To add an implementation date annotation:

1. Go to **Connections** > **GitHub Copilot** > Edit Scope
2. Set **Implementation Date** (when GitHub Copilot was rolled out)
3. Set **Baseline Period** (days to use for "before" comparison)

The dashboard works fully without these settings—correlation analysis doesn't require an implementation date.

## Data Model

The Impact Dashboard joins GitHub Copilot metrics with DORA data:

```sql
-- Weekly aggregation pattern
_copilot_adoption → _adoption_weekly (by week_start) 
                  → JOIN with metric CTEs (pr_metrics, deployments, incidents)
```

Key tables used:
- `_tool_copilot_org_metrics` - Daily GitHub Copilot usage
- `project_pr_metrics` - PR cycle time, review time
- `cicd_deployment_commits` - Deployment frequency, CFR
- `issues` (type='INCIDENT') - MTTR calculation
- `cq_file_metrics` - Code quality (SonarQube)

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "No data" on panels | Check that time range includes GitHub Copilot data; verify `scope_id` variable |
| Correlation shows "N/A" | Need at least 2 weeks of overlapping data |
| CFR/MTTR empty | Ensure incidents are collected (Jira/GitHub issues with 'incident' label) |
| Code Quality empty | Configure SonarQube integration |
