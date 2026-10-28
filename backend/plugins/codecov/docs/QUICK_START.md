# Codecov Plugin - Quick Start Guide

## 5-Minute Setup

### 1. Get Your Codecov API Token
- Log in to [Codecov](https://codecov.io)
- Go to Settings → API
- Generate a new API token
- Copy the token (you'll need it in step 2)

### 2. Add Connection in DevLake
1. Open DevLake → **Data Connections**
2. Click **Add Connection** → Select **Codecov**
3. Paste your API token
4. Click **Test Connection** to verify
5. Click **Save**

### 3. Add Repositories
1. After saving, click **Add Repositories**
2. Search for your repository
3. Select it and click **Add**

### 4. Create Blueprint
1. Go to **Blueprints** → **Create Blueprint**
2. Select your Codecov connection
3. Choose repositories to track
4. Set frequency (recommended: Daily)
5. Click **Save and Run**

### 5. View Dashboard
1. Wait a few minutes for data collection
2. Go to **Dashboards** → **Codecov**
3. Select your project and repository
4. View coverage metrics and trends

## What You'll See

### Key Numbers
- **Overall Coverage**: Percentage of code covered by tests
- **Total Lines**: Total lines of code in repository
- **Lines Covered**: Lines with test coverage
- **Uncovered Lines**: Lines without test coverage
- **Patch Coverage**: Coverage for new/modified code

### Charts
- **Coverage Trend**: How coverage changes over time
- **Patch Coverage Trend**: Running average of patch coverage
- **Lines Trend**: How total and covered lines change

## Common Questions

**Q: How long does data collection take?**  
A: First collection takes 5-15 minutes depending on repository size. Subsequent collections are faster.

**Q: Why is patch coverage sometimes 0% or null?**  
A: This happens when no files changed in a commit, or when Codecov API data is unavailable for that commit.

**Q: Can I track multiple test types?**  
A: Yes! Use the "Test Name" filter to switch between different flags (e.g., "unittests", "e2e-tests").

**Q: How do I generate a report?**  
A: Use the executive summary report generator (see main README for details).

## Next Steps

- Read the [full documentation](README.md) for detailed information
- Explore the dashboard filters to analyze different time periods
- Set up regular reports for your team
- Monitor coverage trends to maintain code quality

