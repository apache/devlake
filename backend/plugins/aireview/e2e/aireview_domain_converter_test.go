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

	domainCode "github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/aireview/impl"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/apache/devlake/plugins/aireview/tasks"
)

const testProject = "test-project"
const testRepoId = "github:GithubRepo:1:200"

// TestConvertAiReviews verifies that ConvertAiReviews writes project-scoped
// rows to the ai_reviews domain table with the correct project_name stamp.
func TestConvertAiReviews(t *testing.T) {
	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	scopeConfig := models.GetDefaultScopeConfig()
	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			ProjectName: testProject,
			ScopeConfig: scopeConfig,
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}

	tester.FlushTabler(&crossdomain.Account{})
	tester.FlushTabler(&domainCode.PullRequest{})
	tester.FlushTabler(&domainCode.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&domainCode.AiReview{})
	tester.FlushTabler(&crossdomain.ProjectMapping{})
	tester.FlushTabler(&repoRow{})

	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_requests.csv", &domainCode.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_request_comments.csv", &domainCode.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})
	tester.ImportCsvIntoTabler("./raw_tables/project_mapping.csv", &crossdomain.ProjectMapping{})

	tester.Subtask(tasks.ExtractAiReviewsMeta, taskData)
	tester.Subtask(tasks.ConvertAiReviewsMeta, taskData)

	var reviews []domainCode.AiReview
	if err := tester.Dal.All(&reviews); err != nil {
		t.Fatalf("Failed to query ai_reviews: %v", err)
	}
	if len(reviews) == 0 {
		t.Fatal("Expected domain ai_reviews, got none")
	}

	for _, r := range reviews {
		if r.ProjectName != testProject {
			t.Errorf("Expected project_name=%q, got %q", testProject, r.ProjectName)
		}
		if r.RepoId != testRepoId {
			t.Errorf("Expected repo_id=%q, got %q", testRepoId, r.RepoId)
		}
		if r.Id == "" {
			t.Error("Domain ai_review has empty Id")
		}
		if r.AiTool == "" {
			t.Error("Domain ai_review has empty AiTool")
		}
	}
	t.Logf("ai_reviews domain records: %d, all stamped with project %q", len(reviews), testProject)
}

// TestConvertAiReviews_SkipsInRepoIdMode verifies that the converter is a no-op
// when only repoId is set (no project mode), keeping domain table empty.
func TestConvertAiReviews_SkipsInRepoIdMode(t *testing.T) {
	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	scopeConfig := models.GetDefaultScopeConfig()
	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      testRepoId,
			ScopeConfig: scopeConfig,
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}

	tester.FlushTabler(&domainCode.PullRequest{})
	tester.FlushTabler(&domainCode.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&domainCode.AiReview{})
	tester.FlushTabler(&repoRow{})

	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_requests.csv", &domainCode.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_request_comments.csv", &domainCode.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})

	tester.Subtask(tasks.ExtractAiReviewsMeta, taskData)
	tester.Subtask(tasks.ConvertAiReviewsMeta, taskData)

	var reviews []domainCode.AiReview
	if err := tester.Dal.All(&reviews); err != nil {
		t.Fatalf("Failed to query ai_reviews: %v", err)
	}
	// Converter must not write anything in single-repo mode.
	if len(reviews) != 0 {
		t.Errorf("Expected 0 domain ai_reviews in single-repo mode, got %d", len(reviews))
	}
}

// TestConvertFailurePredictions verifies that ConvertFailurePredictions writes
// project-scoped rows to ai_failure_predictions with the correct project_name.
func TestConvertFailurePredictions(t *testing.T) {
	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	scopeConfig := models.GetDefaultScopeConfig()
	scopeConfig.WarningThreshold = 50

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			ProjectName: testProject,
			ScopeConfig: scopeConfig,
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}

	tester.FlushTabler(&crossdomain.Account{})
	tester.FlushTabler(&domainCode.PullRequest{})
	tester.FlushTabler(&domainCode.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&models.AiFailurePrediction{})
	tester.FlushTabler(&domainCode.AiFailurePrediction{})
	tester.FlushTabler(&ciTestJob{})
	tester.FlushTabler(&ciTestCase{})
	tester.FlushTabler(&repoRow{})
	tester.FlushTabler(&crossdomain.ProjectMapping{})

	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_requests.csv", &domainCode.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_request_comments.csv", &domainCode.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_jobs.csv", &ciTestJob{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_cases.csv", &ciTestCase{})
	tester.ImportCsvIntoTabler("./raw_tables/project_mapping.csv", &crossdomain.ProjectMapping{})

	tester.Subtask(tasks.ExtractAiReviewsMeta, taskData)
	tester.Subtask(tasks.CalculateFailurePredictionsMeta, taskData)
	tester.Subtask(tasks.ConvertFailurePredictionsMeta, taskData)

	var predictions []domainCode.AiFailurePrediction
	if err := tester.Dal.All(&predictions); err != nil {
		t.Fatalf("Failed to query ai_failure_predictions: %v", err)
	}
	if len(predictions) == 0 {
		t.Fatal("Expected domain ai_failure_predictions, got none")
	}

	for _, p := range predictions {
		if p.ProjectName != testProject {
			t.Errorf("Expected project_name=%q, got %q", testProject, p.ProjectName)
		}
		if p.Id == "" {
			t.Error("Domain ai_failure_prediction has empty Id")
		}
		if p.PredictionOutcome == "" {
			t.Errorf("Domain prediction PR %s has empty outcome", p.PullRequestKey)
		}
	}
	t.Logf("ai_failure_predictions domain records: %d, all stamped with project %q", len(predictions), testProject)
}

// TestConvertFailurePredictions_IsolatedByProject verifies that running the converter
// twice for two different projects does not mix predictions between projects.
func TestConvertFailurePredictions_IsolatedByProject(t *testing.T) {
	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	scopeConfig := models.GetDefaultScopeConfig()
	scopeConfig.WarningThreshold = 50

	makeTaskData := func(proj string) *tasks.AiReviewTaskData {
		return &tasks.AiReviewTaskData{
			Options: &tasks.AiReviewOptions{
				ProjectName: proj,
				ScopeConfig: scopeConfig,
			},
		}
	}

	tester.FlushTabler(&crossdomain.Account{})
	tester.FlushTabler(&domainCode.PullRequest{})
	tester.FlushTabler(&domainCode.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&models.AiFailurePrediction{})
	tester.FlushTabler(&domainCode.AiFailurePrediction{})
	tester.FlushTabler(&ciTestJob{})
	tester.FlushTabler(&ciTestCase{})
	tester.FlushTabler(&repoRow{})
	tester.FlushTabler(&crossdomain.ProjectMapping{})

	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_requests.csv", &domainCode.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_request_comments.csv", &domainCode.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_jobs.csv", &ciTestJob{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_cases.csv", &ciTestCase{})
	// Both projects point to the same repo — simulating the duplication scenario.
	tester.ImportCsvIntoTabler("./raw_tables/project_mapping.csv", &crossdomain.ProjectMapping{})

	// Run pipeline for project A.
	dataA := makeTaskData(testProject)
	if err := tasks.CompilePatterns(dataA); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}
	tester.Subtask(tasks.ExtractAiReviewsMeta, dataA)
	tester.Subtask(tasks.CalculateFailurePredictionsMeta, dataA)
	tester.Subtask(tasks.ConvertFailurePredictionsMeta, dataA)

	// Run pipeline for project B (same repo, different project).
	dataB := makeTaskData("other-project")
	if err := tasks.CompilePatterns(dataB); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}
	// project_mapping for other-project doesn't exist so converter writes 0 rows.
	tester.Subtask(tasks.ConvertFailurePredictionsMeta, dataB)

	var allPredictions []domainCode.AiFailurePrediction
	if err := tester.Dal.All(&allPredictions); err != nil {
		t.Fatalf("Failed to query all ai_failure_predictions: %v", err)
	}

	for _, p := range allPredictions {
		if p.ProjectName != testProject {
			t.Errorf("Found prediction with unexpected project_name=%q (only %q should exist)", p.ProjectName, testProject)
		}
	}
	t.Logf("Isolation verified: %d predictions, all for project %q", len(allPredictions), testProject)
}

// TestConvertPredictionMetrics verifies the full pipeline including metrics conversion.
func TestConvertPredictionMetrics(t *testing.T) {
	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	scopeConfig := models.GetDefaultScopeConfig()
	scopeConfig.WarningThreshold = 50

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			ProjectName: testProject,
			ScopeConfig: scopeConfig,
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}

	tester.FlushTabler(&crossdomain.Account{})
	tester.FlushTabler(&domainCode.PullRequest{})
	tester.FlushTabler(&domainCode.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&models.AiFailurePrediction{})
	tester.FlushTabler(&models.AiPredictionMetrics{})
	tester.FlushTabler(&domainCode.AiPredictionMetrics{})
	tester.FlushTabler(&ciTestJob{})
	tester.FlushTabler(&ciTestCase{})
	tester.FlushTabler(&repoRow{})
	tester.FlushTabler(&crossdomain.ProjectMapping{})

	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_requests.csv", &domainCode.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_request_comments.csv", &domainCode.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_jobs.csv", &ciTestJob{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_test_cases.csv", &ciTestCase{})
	tester.ImportCsvIntoTabler("./raw_tables/project_mapping.csv", &crossdomain.ProjectMapping{})

	tester.Subtask(tasks.ExtractAiReviewsMeta, taskData)
	tester.Subtask(tasks.CalculateFailurePredictionsMeta, taskData)
	tester.Subtask(tasks.CalculatePredictionMetricsMeta, taskData)
	tester.Subtask(tasks.ConvertPredictionMetricsMeta, taskData)

	var metrics []domainCode.AiPredictionMetrics
	if err := tester.Dal.All(&metrics); err != nil {
		t.Fatalf("Failed to query ai_prediction_metrics: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("Expected domain ai_prediction_metrics, got none")
	}

	for _, m := range metrics {
		if m.ProjectName != testProject {
			t.Errorf("Expected project_name=%q, got %q", testProject, m.ProjectName)
		}
		if m.PeriodType == "" {
			t.Error("Domain metric has empty PeriodType")
		}
		if m.PrAuc < 0 || m.PrAuc > 1 {
			t.Errorf("PrAuc out of range: %f", m.PrAuc)
		}
		if m.RocAuc < 0 || m.RocAuc > 1 {
			t.Errorf("RocAuc out of range: %f", m.RocAuc)
		}
	}
	t.Logf("ai_prediction_metrics domain records: %d, all stamped with project %q", len(metrics), testProject)
}
