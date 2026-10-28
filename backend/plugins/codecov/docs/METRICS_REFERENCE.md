# Codecov Metrics Reference

This guide explains all the metrics collected by the Codecov plugin in simple, non-technical terms.

## Coverage Metrics

### Overall Coverage Percentage
**What it is**: The percentage of your entire codebase that is tested.

**Example**: 85% means 85 out of every 100 lines of code are covered by tests.

**Why it matters**: Higher coverage generally means more of your code is tested, reducing the risk of bugs.

**Good to know**: 
- 100% coverage doesn't mean perfect code - tests can still be wrong
- Focus on covering important, critical code paths
- Industry standards vary, but 70-80% is often considered good

### Patch Coverage
**What it is**: The percentage of **new or modified code** that is tested in a specific commit.

**Example**: If you change 100 lines and 80 are tested, patch coverage is 80%.

**Why it matters**: Ensures new code changes include tests, preventing coverage from declining over time.

**Good to know**:
- Patch coverage can be null/empty if no files changed or API data is unavailable
- High patch coverage helps maintain overall coverage as code grows
- Many teams require minimum patch coverage (e.g., 80%) before merging code

### Modified Coverage
**What it is**: Similar to patch coverage - coverage for code that was changed in a commit.

**Example**: If you modify 50 lines and 45 are tested, modified coverage is 90%.

**Why it matters**: Helps ensure code changes don't reduce overall test coverage.

## Line Metrics

### Total Lines
**What it is**: The total number of lines of code in your repository.

**Example**: 29,333 lines means your codebase has 29,333 lines of code.

**Why it matters**: Helps understand the size of your codebase and track growth over time.

**Good to know**: 
- Includes all code files (not comments or blank lines in some calculations)
- Grows as you add features
- Larger codebases need more tests to maintain coverage

### Lines Covered
**What it is**: The number of lines that are executed by your tests.

**Example**: 24,901 lines covered means 24,901 lines of code are tested.

**Why it matters**: Shows the absolute number of lines with test coverage.

**Good to know**: 
- Higher is generally better
- Should grow as you add tests
- Compare with total lines to understand coverage percentage

### Lines Missed (Uncovered)
**What it is**: The number of lines that are NOT executed by your tests.

**Example**: 4,399 lines missed means 4,399 lines have no test coverage.

**Why it matters**: Identifies areas of your code that need tests.

**Good to know**:
- Lower is generally better
- Focus on testing critical paths first
- Some code (like error handlers) may be hard to test

### Lines Total (in Modified Code)
**What it is**: Total number of lines in code that was changed in a commit.

**Example**: If you modify 200 lines, lines total is 200.

**Why it matters**: Helps understand the scope of changes in each commit.

## Execution Metrics

### Hits
**What it is**: The number of times a line of code was executed during test runs.

**Example**: A line with 100 hits was run 100 times during tests.

**Why it matters**: Shows how thoroughly code is tested - more hits often means more thorough testing.

**Good to know**:
- Higher hits don't always mean better tests
- Some code should be tested multiple times with different inputs
- Very high hits might indicate inefficient tests

### Misses
**What it is**: The number of times a line of code was NOT executed during test runs.

**Example**: A line with 5 misses was not executed 5 times (but might have been executed other times).

**Why it matters**: Identifies code paths that are rarely or never tested.

**Good to know**:
- Zero misses means the line is always executed in tests
- High misses might indicate untested error paths or edge cases

### Partials
**What it is**: Lines that are partially covered (some conditions tested, others not).

**Example**: An if/else statement where the "if" is tested but the "else" is not.

**Why it matters**: Identifies code that needs more complete test coverage.

**Good to know**:
- Partials indicate incomplete test coverage
- Should aim to reduce partials by testing all code paths

## Method Metrics

### Methods Covered
**What it is**: The number of functions/methods that are tested.

**Example**: 5,790 methods covered means 5,790 functions have tests.

**Why it matters**: Ensures individual functions are tested, not just lines of code.

**Good to know**:
- Methods are functions or procedures in your code
- Higher is generally better
- Should track alongside line coverage

### Methods Total
**What it is**: The total number of functions/methods in your code.

**Example**: 6,000 methods total means your code has 6,000 functions.

**Why it matters**: Helps understand how many functions need testing.

**Good to know**:
- Compare with methods covered to see percentage
- Large number of untested methods indicates areas needing attention

## Change Metrics

### Files Changed
**What it is**: The number of files modified in a commit.

**Example**: 5 files changed means 5 files were modified in that commit.

**Why it matters**: Helps understand the scope of changes - more files often means bigger changes.

**Good to know**:
- Larger changes may need more tests
- Many small file changes might indicate refactoring
- Track alongside coverage to ensure changes include tests

## Trend Metrics

### Coverage Trend
**What it is**: Whether coverage is improving, declining, or staying stable over time.

**Example**: "Improving (2.5% change)" means coverage increased by 2.5 percentage points.

**Why it matters**: Shows if your testing efforts are effective and coverage is improving.

**Good to know**:
- Improving: Coverage is going up - good!
- Declining: Coverage is going down - may need attention
- Stable: Coverage is staying the same - maintain current testing practices

### Running Average (Patch Coverage Trend)
**What it is**: A cumulative average calculated from the first day to each day.

**Example**: 
- Day 1: 80% → Average: 80%
- Day 2: 90% → Average: (80% + 90%) / 2 = 85%
- Day 3: 85% → Average: (80% + 90% + 85%) / 3 = 85%

**Why it matters**: Shows overall trend direction - is patch coverage generally improving or declining?

**Good to know**:
- Smooth line that continues even on days with no data
- Rising line = improving coverage over time
- Falling line = declining coverage over time
- Flat line = stable coverage

## Flag-Based Metrics

### Coverage by Flag (Test Type)
**What it is**: Coverage percentage for a specific type of test (e.g., "unit tests", "e2e tests").

**Example**: 
- Unit Tests: 85% coverage
- E2E Tests: 60% coverage

**Why it matters**: Different test types cover different aspects - unit tests test individual functions, e2e tests test entire workflows.

**Good to know**:
- Each flag can have different coverage levels
- Compare flags to see which test types need more coverage
- Some flags may have higher coverage than others (this is normal)

## How Metrics Work Together

### Coverage Calculation
```
Coverage % = (Lines Covered / Lines Total) × 100
```

**Example**: 
- 24,901 lines covered
- 29,333 lines total
- Coverage = (24,901 / 29,333) × 100 = 84.89%

### Patch Coverage vs Overall Coverage
- **Overall Coverage**: All code in repository
- **Patch Coverage**: Only new/modified code in a commit
- **Goal**: Keep patch coverage high to maintain or improve overall coverage

### Understanding Trends
- **Improving Coverage**: Lines covered growing faster than lines total
- **Declining Coverage**: Lines total growing faster than lines covered
- **Stable Coverage**: Both growing at similar rates

## Best Practices

1. **Focus on Patch Coverage**: Ensure new code has high coverage to maintain overall coverage
2. **Monitor Trends**: Watch for declining coverage trends and address early
3. **Set Goals**: Establish minimum coverage thresholds (e.g., 80% overall, 90% patch)
4. **Review Regularly**: Check metrics weekly or monthly
5. **Combine Metrics**: Use multiple metrics together for a complete picture

## Common Scenarios

### Scenario 1: High Overall, Low Patch
**Situation**: Overall coverage is 90%, but patch coverage is 50%

**Meaning**: Existing code is well-tested, but new code isn't being tested adequately

**Action**: Focus on adding tests for new code changes

### Scenario 2: Improving Trend
**Situation**: Coverage trend shows "Improving (5% change)"

**Meaning**: Coverage increased by 5 percentage points in the time period

**Action**: Continue current testing practices

### Scenario 3: Declining Trend
**Situation**: Coverage trend shows "Declining (3% change)"

**Meaning**: Coverage decreased by 3 percentage points

**Action**: Review recent commits, ensure new code includes tests, consider increasing patch coverage requirements

### Scenario 4: High Misses
**Situation**: Many lines have high "misses" count

**Meaning**: These lines are rarely or never executed in tests

**Action**: Add tests to cover these code paths, especially for critical functionality

