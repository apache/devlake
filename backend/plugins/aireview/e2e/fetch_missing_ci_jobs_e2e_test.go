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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/pkg/gcshelper"
	"github.com/apache/devlake/plugins/aireview/impl"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/apache/devlake/plugins/aireview/tasks"
)

// e2eHistoryStore is an in-memory HistoryStore used in e2e tests to avoid real
// GCS network access.
type e2eHistoryStore struct {
	subdirs map[string][]string
	files   map[string][]byte
}

func (f *e2eHistoryStore) ListSubdirectories(_ context.Context, prefix string) ([]string, error) {
	return f.subdirs[prefix], nil
}

func (f *e2eHistoryStore) ReadFile(_ context.Context, path string) ([]byte, error) {
	data, ok := f.files[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return data, nil
}

var _ gcshelper.HistoryStore = (*e2eHistoryStore)(nil)

// TestFetchMissingCiJobs verifies the end-to-end flow:
//  1. AI reviews are extracted from PR comments (no CI data present).
//  2. FetchMissingCiJobs detects the gap and fetches from the fake GCS store.
//  3. ci_test_jobs is populated with the correct rows.
//  4. CalculateFailurePredictions can then use the backfilled rows.
func TestFetchMissingCiJobs(t *testing.T) {
	const repoId = "github:GithubRepo:1:200"

	var plugin impl.AiReview
	tester := e2ehelper.NewDataFlowTester(t, "aireview", plugin)

	// Fake GCS store — provides build results for PRs 101 and 103.
	// PR 101: unit-tests FAILED (high-risk AI review → TP)
	// PR 103: e2e-tests FAILED (low-risk AI review → FN)
	// PR 102 and 104: no GCS data (no builds recorded)
	now := time.Now()
	recentTS := now.Add(-48 * time.Hour).Unix()

	store := &e2eHistoryStore{
		subdirs: map[string][]string{
			"pr-logs/pull/myorg_myrepo/101/": {
				"pr-logs/pull/myorg_myrepo/101/pull-ci-myorg-myrepo-main-unit-tests/",
			},
			"pr-logs/pull/myorg_myrepo/101/pull-ci-myorg-myrepo-main-unit-tests/": {
				"pr-logs/pull/myorg_myrepo/101/pull-ci-myorg-myrepo-main-unit-tests/9000001/",
			},
			"pr-logs/pull/myorg_myrepo/103/": {
				"pr-logs/pull/myorg_myrepo/103/pull-ci-myorg-myrepo-main-e2e-tests/",
			},
			"pr-logs/pull/myorg_myrepo/103/pull-ci-myorg-myrepo-main-e2e-tests/": {
				"pr-logs/pull/myorg_myrepo/103/pull-ci-myorg-myrepo-main-e2e-tests/9000003/",
			},
		},
		files: map[string][]byte{
			"pr-logs/pull/myorg_myrepo/101/pull-ci-myorg-myrepo-main-unit-tests/9000001/finished.json": []byte(
				fmt.Sprintf(`{"timestamp":%d,"passed":false,"result":"FAILURE"}`, recentTS),
			),
			"pr-logs/pull/myorg_myrepo/103/pull-ci-myorg-myrepo-main-e2e-tests/9000003/finished.json": []byte(
				fmt.Sprintf(`{"timestamp":%d,"passed":false,"result":"FAILURE"}`, recentTS),
			),
		},
	}

	scopeConfig := models.GetDefaultScopeConfig()
	scopeConfig.WarningThreshold = 50
	scopeConfig.CiBackfillEnabled = true
	scopeConfig.CiBackfillDays = 90
	scopeConfig.CiFailureSource = models.CiSourceJobResult // simplest source for this test

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      repoId,
			ScopeConfig: scopeConfig,
		},
		GcsStoreOverride: store,
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("CompilePatterns: %v", err)
	}

	// Flush tables.
	tester.FlushTabler(&code.PullRequest{})
	tester.FlushTabler(&code.PullRequestComment{})
	tester.FlushTabler(&models.AiReview{})
	tester.FlushTabler(&models.AiFailurePrediction{})
	tester.FlushTabler(&ciTestJob{})
	tester.FlushTabler(&repoRow{})

	// Import fixtures — same PRs and comments as the main CI prediction test,
	// but WITHOUT importing ci_test_jobs so all PRs are "missing".
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_requests.csv", &code.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_pull_request_comments.csv", &code.PullRequestComment{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})

	// Step 1: extract AI reviews.
	tester.Subtask(tasks.ExtractAiReviewsMeta, taskData)

	// Step 2: backfill CI data from fake GCS.
	tester.Subtask(tasks.FetchMissingCiJobsMeta, taskData)

	// Verify ci_test_jobs was populated with the GCS-backfilled rows.
	var jobs []ciTestJob
	if err := tester.Dal.All(&jobs); err != nil {
		t.Fatalf("Failed to query ci_test_jobs: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("Expected ci_test_jobs to be populated after GCS backfill, got 0 rows")
	}

	// We should have exactly 2 rows (PR 101 and PR 103).
	if len(jobs) != 2 {
		t.Errorf("Expected 2 ci_test_jobs rows, got %d", len(jobs))
	}

	prJobMap := make(map[int64]string) // pr_number → result
	for _, j := range jobs {
		prJobMap[j.PullRequestNumber] = j.Result
		// All backfilled rows use connection_id=0 sentinel.
		if j.ConnectionId != 0 {
			t.Errorf("Expected connection_id=0 for backfilled row, got %d", j.ConnectionId)
		}
		if j.TriggerType != "pull_request" {
			t.Errorf("Expected trigger_type=pull_request, got %q", j.TriggerType)
		}
	}

	if prJobMap[101] != "failure" {
		t.Errorf("PR 101: expected result=failure, got %q", prJobMap[101])
	}
	if prJobMap[103] != "failure" {
		t.Errorf("PR 103: expected result=failure, got %q", prJobMap[103])
	}

	// Step 3: run predictions on the backfilled data.
	tester.Subtask(tasks.CalculateFailurePredictionsMeta, taskData)

	var predictions []models.AiFailurePrediction
	if err := tester.Dal.All(&predictions); err != nil {
		t.Fatalf("Failed to query predictions: %v", err)
	}

	byPR := make(map[string]*models.AiFailurePrediction)
	for i := range predictions {
		p := &predictions[i]
		if p.CiFailureSource == models.CiSourceJobResult {
			byPR[p.PullRequestKey] = p
		}
	}

	// PR 101: high-risk AI review (score=80 > threshold=50), CI failed → TP
	if p := byPR["101"]; p != nil {
		if p.PredictionOutcome != models.PredictionTP {
			t.Errorf("PR 101: expected TP, got %s", p.PredictionOutcome)
		}
	} else {
		t.Error("PR 101: no prediction found")
	}

	// PR 103: low-risk AI review (score=20 < threshold=50), CI failed → FN
	if p := byPR["103"]; p != nil {
		if p.PredictionOutcome != models.PredictionFN {
			t.Errorf("PR 103: expected FN, got %s", p.PredictionOutcome)
		}
	} else {
		t.Error("PR 103: no prediction found")
	}

	t.Logf("Backfill e2e: %d ci_test_jobs rows, %d predictions", len(jobs), len(predictions))
}
