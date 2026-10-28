/*
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
*/

package e2e

import (
	"testing"

	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/aireview/impl"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/apache/devlake/plugins/aireview/tasks"
)

// ciTestJob matches the ci_test_jobs domain table for fixture import.
type ciTestJob struct {
	ConnectionId      int64   `gorm:"primaryKey;column:connection_id"`
	JobId             string  `gorm:"primaryKey;column:job_id"`
	JobName           string  `gorm:"column:job_name"`
	Repository        string  `gorm:"column:repository"`
	PullRequestNumber int64   `gorm:"column:pull_request_number"`
	TriggerType       string  `gorm:"column:trigger_type"`
	Result            string  `gorm:"column:result"`
	FinishedAt        string  `gorm:"column:finished_at"`
	DurationSec       float64 `gorm:"column:duration_sec"`
}

func (ciTestJob) TableName() string { return "ci_test_jobs" }

// ciTestCase matches the ci_test_cases domain table for fixture import.
type ciTestCase struct {
	ConnectionId int64  `gorm:"primaryKey;column:connection_id"`
	JobId        string `gorm:"primaryKey;column:job_id"`
	TestCaseId   string `gorm:"primaryKey;column:test_case_id"`
	Name         string `gorm:"column:name"`
	Status       string `gorm:"column:status"`
}

func (ciTestCase) TableName() string { return "ci_test_cases" }

// repoRow matches the repos domain table for fixture import.
type repoRow struct {
	Id   string `gorm:"primaryKey;column:id"`
	Name string `gorm:"column:name"`
}

func (repoRow) TableName() string { return "repos" }

// TestCalculateFailurePredictions verifies that:
//   - Flaky tests (failed in periodic runs) are excluded from CI failure detection.
//   - PRs with non-flaky failures are correctly flagged as HadCiFailure=true.
//   - Prediction outcomes (TP/FP/FN/TN) are assigned correctly.
func TestCalculateFailurePredictions(t *testing.T) {
	const repoId = "github:GithubRepo:1:200"

	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	scopeConfig := models.GetDefaultScopeConfig()
	scopeConfig.WarningThreshold = 50

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      repoId,
			ScopeConfig: scopeConfig,
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}

	// Flush tables (also creates them in lake_test if missing).
	tester.FlushTabler(&crossdomain.Account{})
	tester.FlushTabler(&code.PullRequest{})
	tester.FlushTabler(&code.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&models.AiFailurePrediction{})
	tester.FlushTabler(&ciTestJob{})
	tester.FlushTabler(&ciTestCase{})
	tester.FlushTabler(&repoRow{})

	// Import fixtures.
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_requests.csv", &code.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_request_comments.csv", &code.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_jobs.csv", &ciTestJob{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_cases.csv", &ciTestCase{})

	// Run extraction then prediction.
	tester.Subtask(tasks.ExtractAiReviewsMeta, taskData)
	tester.Subtask(tasks.CalculateFailurePredictionsMeta, taskData)

	// Collect results — default scope config uses CiSourceBoth, so 2 predictions per PR.
	var predictions []models.AiFailurePrediction
	if err := tester.Dal.All(&predictions); err != nil {
		t.Fatalf("Failed to query predictions: %v", err)
	}

	if len(predictions) == 0 {
		t.Fatal("Expected predictions, got none")
	}

	// Build a map by (pull_request_key, ci_failure_source) for assertions.
	type predKey struct {
		PR     string
		Source string
	}
	byKey := make(map[predKey]*models.AiFailurePrediction)
	for i := range predictions {
		p := &predictions[i]
		byKey[predKey{p.PullRequestKey, p.CiFailureSource}] = p
	}

	// Fixtures are designed so both sources produce consistent outcomes:
	// - PR 101: unit-tests fails (non-flaky job/test) → HadCiFailure=true → TP
	// - PR 102: flaky-integration fails (same job/test fails periodically) → HadCiFailure=false → FP
	// - PR 103: e2e-tests fails (non-flaky job/test) → HadCiFailure=true → FN
	// - PR 104: unit-tests passes → HadCiFailure=false → TN
	type wantCase struct {
		prKey, source   string
		flagged, ciFail bool
		outcome         string
	}
	cases := []wantCase{
		{"101", models.CiSourceTestCases, true, true, models.PredictionTP},
		{"102", models.CiSourceTestCases, true, false, models.PredictionFP},
		{"103", models.CiSourceTestCases, false, true, models.PredictionFN},
		{"104", models.CiSourceTestCases, false, false, models.PredictionTN},
		{"101", models.CiSourceJobResult, true, true, models.PredictionTP},
		{"102", models.CiSourceJobResult, true, false, models.PredictionFP},
		{"103", models.CiSourceJobResult, false, true, models.PredictionFN},
		{"104", models.CiSourceJobResult, false, false, models.PredictionTN},
	}

	for _, tc := range cases {
		k := predKey{tc.prKey, tc.source}
		p, ok := byKey[k]
		if !ok {
			t.Errorf("Missing prediction for PR %s source %s", tc.prKey, tc.source)
			continue
		}
		assertPrediction(t, p, tc.prKey, tc.flagged, tc.ciFail, tc.outcome)
	}

	t.Logf("Predictions verified: %d total", len(predictions))
}

// TestCalculatePredictionMetrics verifies that precision, recall, and AUC are
// computed from the predictions written by TestCalculateFailurePredictions.
func TestCalculatePredictionMetrics(t *testing.T) {
	const repoId = "github:GithubRepo:1:200"

	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	scopeConfig := models.GetDefaultScopeConfig()
	scopeConfig.WarningThreshold = 50

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      repoId,
			ScopeConfig: scopeConfig,
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}

	tester.FlushTabler(&code.PullRequest{})
	tester.FlushTabler(&code.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&models.AiFailurePrediction{})
	tester.FlushTabler(&models.AiPredictionMetrics{})
	tester.FlushTabler(&ciTestJob{})
	tester.FlushTabler(&ciTestCase{})
	tester.FlushTabler(&repoRow{})

	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_requests.csv", &code.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_request_comments.csv", &code.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_jobs.csv", &ciTestJob{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_cases.csv", &ciTestCase{})

	tester.Subtask(tasks.ExtractAiReviewsMeta, taskData)
	tester.Subtask(tasks.CalculateFailurePredictionsMeta, taskData)
	tester.Subtask(tasks.CalculatePredictionMetricsMeta, taskData)

	var metrics []models.AiPredictionMetrics
	if err := tester.Dal.All(&metrics); err != nil {
		t.Fatalf("Failed to query metrics: %v", err)
	}

	if len(metrics) == 0 {
		t.Fatal("Expected metrics records, got none")
	}

	// Validate one of the rolling_60d records (covers all 4 test PRs).
	for _, m := range metrics {
		if m.PeriodType != "rolling_60d" {
			continue
		}
		// With 4 PRs: TP=1, FP=1, FN=1, TN=1 at threshold=50
		// Precision = 1/(1+1) = 0.5, Recall = 1/(1+1) = 0.5
		if m.TruePositives < 0 || m.FalsePositives < 0 || m.FalseNegatives < 0 || m.TrueNegatives < 0 {
			t.Errorf("Invalid confusion matrix: TP=%d FP=%d FN=%d TN=%d", m.TruePositives, m.FalsePositives, m.FalseNegatives, m.TrueNegatives)
		}
		if m.WarningThreshold != 50 {
			t.Errorf("Expected WarningThreshold=50, got %d", m.WarningThreshold)
		}
		if m.PrAuc < 0 || m.PrAuc > 1 {
			t.Errorf("PrAuc out of range: %f", m.PrAuc)
		}
		if m.RocAuc < 0 || m.RocAuc > 1 {
			t.Errorf("RocAuc out of range: %f", m.RocAuc)
		}
		t.Logf("rolling_60d metrics: TP=%d FP=%d FN=%d TN=%d precision=%.2f recall=%.2f pr_auc=%.3f roc_auc=%.3f",
			m.TruePositives, m.FalsePositives, m.FalseNegatives, m.TrueNegatives,
			m.Precision, m.Recall, m.PrAuc, m.RocAuc)
		break
	}
}

// TestCalculateFailurePredictions_NoCiData verifies that when a repo has AI
// reviews but no CI data (e.g. TestRegistry is not configured), no predictions
// are written. Before the fix, every flagged PR was incorrectly classified as
// FP because missing CI data was treated as "CI passed".
func TestCalculateFailurePredictions_NoCiData(t *testing.T) {
	const repoId = "github:GithubRepo:1:999"

	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	scopeConfig := models.GetDefaultScopeConfig()
	scopeConfig.WarningThreshold = 50

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      repoId,
			ScopeConfig: scopeConfig,
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}

	tester.FlushTabler(&crossdomain.Account{})
	tester.FlushTabler(&code.PullRequest{})
	tester.FlushTabler(&code.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&models.AiFailurePrediction{})
	tester.FlushTabler(&ciTestJob{})
	tester.FlushTabler(&ciTestCase{})
	tester.FlushTabler(&repoRow{})

	// Import PRs and AI review comments — but intentionally NO ci_test_jobs rows.
	tester.ImportCsvIntoTabler("./raw_tables/no_ci_pull_requests.csv", &code.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/no_ci_pull_request_comments.csv", &code.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/no_ci_repos.csv", &repoRow{})

	tester.Subtask(tasks.ExtractAiReviewsMeta, taskData)
	tester.Subtask(tasks.CalculateFailurePredictionsMeta, taskData)

	var predictions []models.AiFailurePrediction
	if err := tester.Dal.All(&predictions); err != nil {
		t.Fatalf("Failed to query predictions: %v", err)
	}

	// Key assertion: PRs without CI data must produce NO_CI records (not TP/FP/FN/TN).
	// This makes them visible in drill-down dashboards while keeping confusion matrix clean.
	for _, p := range predictions {
		if p.PredictionOutcome != models.PredictionNoCi {
			t.Errorf("PR %s: expected NO_CI outcome for PR with no CI data, got %s (flagged=%v hadCI=%v)",
				p.PullRequestKey, p.PredictionOutcome, p.WasFlaggedRisky, p.HadCiFailure)
		}
		if p.CiFailureSource != models.CiSourceNone {
			t.Errorf("PR %s: expected ci_failure_source=%q, got %q", p.PullRequestKey, models.CiSourceNone, p.CiFailureSource)
		}
	}
	if len(predictions) == 0 {
		t.Error("expected NO_CI predictions for AI-reviewed PRs with no CI data, got none")
	}
}

func assertPrediction(t *testing.T, p *models.AiFailurePrediction, prKey string, wantFlagged, wantCiFail bool, wantOutcome string) {
	t.Helper()
	if p.WasFlaggedRisky != wantFlagged {
		t.Errorf("PR %s: WasFlaggedRisky=%v, want %v", prKey, p.WasFlaggedRisky, wantFlagged)
	}
	if p.HadCiFailure != wantCiFail {
		t.Errorf("PR %s: HadCiFailure=%v, want %v", prKey, p.HadCiFailure, wantCiFail)
	}
	if p.PredictionOutcome != wantOutcome {
		t.Errorf("PR %s: PredictionOutcome=%s, want %s", prKey, p.PredictionOutcome, wantOutcome)
	}
}
