# Codecov Plugin Documentation

## Overview

The Codecov plugin integrates with [Codecov](https://codecov.io), a code coverage analysis service, to collect and analyze test coverage data from your repositories. This plugin helps you understand how much of your code is tested and track coverage trends over time.

## What is Code Coverage?

**Code coverage** measures how much of your source code is executed by your tests. It's expressed as a percentage:

- **100% coverage** means every line of code is tested
- **50% coverage** means half of your code is tested
- **0% coverage** means no code is tested

Higher coverage generally indicates better test quality, but it's not the only factor in code quality.

## Key Concepts

### Coverage Types

The plugin collects several types of coverage metrics:

1. **Overall Coverage**: The percentage of all code lines that are covered by tests
2. **Patch Coverage**: The percentage of new or modified code lines that are covered by tests (also called "diff coverage")
3. **Flag-Based Coverage**: Coverage for specific test suites or test types (e.g., "unit tests", "e2e tests")

### Flags (Test Names)

In Codecov, **flags** are labels that categorize different types of tests:

- **Unit Tests**: Tests for individual functions or components
- **E2E Tests**: End-to-end tests that test entire workflows
- **Integration Tests**: Tests that verify how different parts work together

Each flag can have its own coverage metrics, allowing you to track coverage separately for different test types.

### Commits and Coverage

The plugin tracks coverage for each commit in your repository, allowing you to:
- See how coverage changes over time
- Identify commits that increased or decreased coverage
- Track coverage trends for specific branches

## What Data is Collected?

The plugin collects the following information:

### Repository Information

- Repository name and identifier
- Branch information
- Language and service type

### Coverage Metrics (per commit and flag)

- **Coverage Percentage**: Overall percentage of code covered
- **Lines Covered**: Number of code lines that are tested
- **Lines Total**: Total number of code lines
- **Lines Missed**: Number of code lines not tested
- **Hits**: Number of times code was executed during tests
- **Misses**: Number of times code was not executed
- **Methods Covered/Total**: Number of methods (functions) covered vs total

### Patch Coverage Metrics

- **Patch Coverage**: Coverage percentage for new or modified code
- **Modified Coverage**: Coverage for code changed in a commit
- **Files Changed**: Number of files modified in a commit
- **Lines Covered/Missed in Modified Code**: Coverage details for changed code

### Trends

- Daily coverage trends over time
- Coverage changes between commits
- Running averages of patch coverage

## Setup Instructions

### Prerequisites

1. A Codecov account with access to your repositories
2. A Codecov API token (obtained from your Codecov account settings)
3. Your repository must be connected to Codecov and have coverage reports being generated

### Step 1: Create a Connection

1. In DevLake, go to **Data Connections**
2. Click **Add Connection** and select **Codecov**
3. Enter your Codecov API token
4. Test the connection to verify it works
5. Save the connection

### Step 2: Add Repositories

1. After creating the connection, click **Add Repositories**
2. Search for and select the repositories you want to track
3. The plugin will automatically discover available repositories from your Codecov account

### Step 3: Create a Blueprint

1. Go to **Blueprints** in DevLake
2. Create a new blueprint
3. Select your Codecov connection
4. Choose the repositories to include
5. Set the data collection frequency (e.g., daily, weekly)
6. Save and run the blueprint

### Step 4: View Your Data

Once data collection starts, you can:
- View dashboards in Grafana (see below)
- Generate executive summary reports
- Query the data directly from the database

## Available Dashboards

### Codecov Dashboard

The main Codecov dashboard provides:

#### Key Metrics Panels
- **Overall Coverage**: Current coverage percentage for the selected test type
- **Total Lines**: Total lines of code in the repository
- **Lines Covered**: Number of lines with test coverage
- **Uncovered Lines**: Number of lines without test coverage
- **Patch Coverage**: Average patch coverage for the selected time period

#### Trend Charts
- **Overall Coverage Trend**: How overall coverage changes over time
- **Patch Coverage Trend**: Running average of patch coverage (shows cumulative average from first day to each day)
- **Lines Coverage Trend**: How lines covered and total lines change over time

#### Filters
- **Project**: Filter by DevLake project
- **Repository**: Select specific repository
- **Test Name (Flag)**: Filter by test type (e.g., "unittests", "e2e-tests")
- **Time Range**: Select date range (default: last 30 days)

### Understanding the Patch Coverage Trend

The Patch Coverage Trend chart shows a **running average**, which means:
- It calculates the average patch coverage from the first day to each subsequent day
- If there's no data for a day, it continues the line using the last known average
- This gives you a smooth trend line that shows overall patch coverage improvement or decline

**Example**: If you have patch coverage of 80% on day 1, 90% on day 2, and 85% on day 3:
- Day 1 average: 80%
- Day 2 average: (80% + 90%) / 2 = 85%
- Day 3 average: (80% + 90% + 85%) / 3 = 85%

## Executive Summary Reports

The plugin can generate PDF executive summary reports that provide high-level coverage metrics for leadership. These reports include:

- Overall project coverage percentage
- Coverage by test type
- Total repositories and lines of code
- Coverage trends (improving, declining, or stable)
- Average patch coverage
- Visual charts and graphs

Reports are based on configurable time periods (e.g., last 7, 24, or 30 days).

## Data Tables

The plugin stores data in the following database tables:

- **`_tool_codecov_repos`**: Repository information
- **`_tool_codecov_flags`**: Test flags (test types) for each repository
- **`_tool_codecov_commits`**: Commit metadata
- **`_tool_codecov_coverages`**: Coverage metrics per commit and flag
- **`_tool_codecov_comparisons`**: Patch coverage and comparison data
- **`_tool_codecov_commit_coverages`**: Overall commit-level coverage (without flags)

## Common Use Cases

### 1. Track Coverage Trends
Monitor how code coverage changes over time to ensure it's improving or staying stable.

### 2. Identify Coverage Gaps
Find areas of your codebase that lack test coverage and prioritize adding tests.

### 3. Compare Test Types
Compare coverage between different test types (unit tests vs. e2e tests) to understand where testing efforts should focus.

### 4. Monitor Patch Coverage
Ensure that new code changes include adequate test coverage by tracking patch coverage metrics.

### 5. Generate Reports for Leadership
Create executive summaries showing overall project health and coverage trends.

## Best Practices

1. **Set Coverage Goals**: Establish minimum coverage thresholds (e.g., 80%) and track progress
2. **Monitor Patch Coverage**: Ensure new code maintains or improves overall coverage
3. **Review Trends Regularly**: Check coverage trends weekly or monthly to catch declining coverage early
4. **Use Flags Appropriately**: Organize tests with meaningful flag names to track different test types separately
5. **Combine with Other Metrics**: Use coverage data alongside other quality metrics for a complete picture

## Troubleshooting

### No Data Appearing

- **Check Connection**: Verify your Codecov API token is valid
- **Verify Repository Setup**: Ensure the repository is connected to Codecov and generating coverage reports
- **Check Blueprint Status**: Make sure the blueprint ran successfully
- **Review Time Range**: Ensure your selected time range includes dates when coverage data exists

### Coverage Numbers Don't Match Codecov

- **Flag Filtering**: Make sure you're comparing the same flags/test types
- **Time Range**: Coverage may vary by time period - ensure you're comparing the same dates
- **Branch Selection**: Verify you're looking at the same branch (main/master)

### Missing Patch Coverage Data

- **API Availability**: Patch coverage requires the Codecov Compare API - verify your API token has access
- **Parent Commit**: Patch coverage compares to parent commit - ensure parent commits exist
- **Null Values**: Some commits may not have patch coverage if no files changed or API data is unavailable

## Support and Resources

- **Codecov Documentation**: [https://docs.codecov.com](https://docs.codecov.com)
- **DevLake Documentation**: [https://devlake.apache.org/docs](https://devlake.apache.org/docs)
- **Issues**: Report issues on the DevLake GitHub repository

## Glossary

- **Coverage**: The percentage of code executed by tests
- **Flag**: A label for categorizing different test types in Codecov
- **Patch Coverage**: Coverage percentage for new or modified code in a commit
- **Running Average**: A cumulative average calculated from the first value to each subsequent value
- **Commit**: A snapshot of code changes in a repository
- **Branch**: A parallel version of code in a repository (e.g., "main", "develop")

