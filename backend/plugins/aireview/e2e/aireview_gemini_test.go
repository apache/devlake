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
	"strings"
	"testing"

	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/aireview/impl"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/apache/devlake/plugins/aireview/tasks"
)

// TestExtractGeminiAiReviews verifies that Gemini Code Assist review comments
// are correctly extracted, classified, and summarised.
//
// Fixture data is based on real comments from google-gemini/gemini-cli PR #15147:
//   - Issue comment 3661243217: PR-level "Summary of Changes" format
//   - Review comment 2623871161: inline comment with critical priority badge
//     (https://github.com/google-gemini/gemini-cli/pull/15147)
func TestExtractGeminiAiReviews(t *testing.T) {
	var plug impl.AiReview
	dataflowTester := e2ehelper.NewDataFlowTester(t, "aireview", plug)

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      "github:GithubRepo:1:200",
			ScopeConfig: models.GetDefaultScopeConfig(),
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("failed to compile patterns: %v", err)
	}

	dataflowTester.FlushTabler(&crossdomain.Account{})
	dataflowTester.FlushTabler(&code.PullRequest{})
	dataflowTester.FlushTabler(&code.PullRequestComment{})
	dataflowTester.FlushTabler(&models.AiReview{})

	dataflowTester.ImportCsvIntoTabler("./raw_tables/pull_requests_gemini.csv", &code.PullRequest{})
	dataflowTester.ImportCsvIntoTabler("./raw_tables/pull_request_comments_gemini.csv", &code.PullRequestComment{})

	dataflowTester.Subtask(tasks.ExtractAiReviewsMeta, taskData)

	var reviews []models.AiReview
	if err := dataflowTester.Dal.All(&reviews); err != nil {
		t.Fatalf("failed to query reviews: %v", err)
	}

	// Both Gemini comments should be detected and extracted.
	if len(reviews) != 2 {
		t.Fatalf("expected 2 Gemini reviews, got %d", len(reviews))
	}

	byComment := make(map[string]models.AiReview)
	for _, r := range reviews {
		byComment[r.ReviewId] = r
	}

	// Shared properties: all reviews must be attributed to Gemini.
	for _, r := range reviews {
		if r.AiTool != models.AiToolGemini {
			t.Errorf("expected AiTool=%s, got %s (review_id=%s)", models.AiToolGemini, r.AiTool, r.ReviewId)
		}
		if r.AiToolUser != "gemini-code-assist[bot]" {
			t.Errorf("expected AiToolUser=gemini-code-assist[bot], got %s", r.AiToolUser)
		}
		if r.SourcePlatform != "github" {
			t.Errorf("expected SourcePlatform=github, got %s", r.SourcePlatform)
		}
		if r.Summary == "" {
			t.Errorf("expected non-empty summary for review_id=%s", r.ReviewId)
		}
	}

	// PR-level "Summary of Changes" comment (issue comment 3661243217).
	// Summary extraction should pull the first substantive paragraph and Highlights items.
	// Risk should be low — no risk keywords in the body.
	if summary, ok := byComment["github:GithubPrComment:1:3661243217"]; ok {
		if !strings.Contains(summary.Summary, "native Git integration") {
			t.Errorf("PR summary should mention native Git integration, got: %s", summary.Summary)
		}
		if summary.RiskLevel != models.RiskLevelLow {
			t.Errorf("PR summary should be low risk, got %s", summary.RiskLevel)
		}
	} else {
		t.Error("PR-level summary comment not found in results")
	}

	// Inline review comment with critical priority badge (review comment 2623871161).
	// Summary extraction should strip the gstatic.com badge image.
	// Risk should be high because the body contains the word "critical".
	if inline, ok := byComment["github:GithubPrComment:1:2623871161"]; ok {
		if inline.RiskLevel != models.RiskLevelHigh {
			t.Errorf("inline review should be high risk (critical keyword), got %s", inline.RiskLevel)
		}
		if strings.Contains(inline.Summary, "gstatic.com") {
			t.Errorf("inline summary should have badge URL stripped, got: %s", inline.Summary)
		}
	} else {
		t.Error("inline review comment not found in results")
	}
}
