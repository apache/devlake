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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSuggestionAcceptanceTracking verifies that suggestion acceptance markers
// are parsed from AI tool comment bodies during review extraction.
//
// Test data covers three cases:
//   - PR 2001 (CodeRabbit): 2x "✅ Resolved" in tracking table + "2 suggestions applied" summary → 4 accepted
//   - PR 2002 (Qodo): 2x "[x]" checked + 2x "[ ]" unchecked checkboxes → 2 accepted
//   - PR 2003 (CodeRabbit): no acceptance markers → 0 accepted
func TestSuggestionAcceptanceTracking(t *testing.T) {
	var plug impl.AiReview
	dataflowTester := e2ehelper.NewDataFlowTester(t, "aireview", plug)

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      "github:GithubRepo:1:200",
			ScopeConfig: models.GetDefaultScopeConfig(),
		},
	}
	require.NoError(t, tasks.CompilePatterns(taskData))

	// Flush and load domain tables
	dataflowTester.FlushTabler(&crossdomain.Account{})
	dataflowTester.FlushTabler(&code.PullRequest{})
	dataflowTester.FlushTabler(&code.PullRequestComment{})
	dataflowTester.FlushTabler(&models.AiReview{})

	dataflowTester.ImportCsvIntoTabler("./raw_tables/suggestion_pull_requests.csv", &code.PullRequest{})
	dataflowTester.ImportCsvIntoTabler("./raw_tables/suggestion_pull_request_comments.csv", &code.PullRequestComment{})

	// Run extraction
	dataflowTester.Subtask(tasks.ExtractAiReviewsMeta, taskData)

	// Verify results
	var reviews []models.AiReview
	require.NoError(t, dataflowTester.Dal.All(&reviews))
	require.Equal(t, 3, len(reviews), "expected 3 AI reviews")

	// Build lookup by review_id (the domain comment ID)
	type expected struct {
		aiTool              string
		suggestionsAccepted int
	}
	want := map[string]expected{
		// CodeRabbit: 2x "✅ Resolved" + "2 suggestions applied" = 4
		"github:GithubPrComment:1:6001": {aiTool: models.AiToolCodeRabbit, suggestionsAccepted: 4},
		// Qodo: 2 checked [x] + 2 unchecked [ ] = 2 accepted
		"github:GithubPrComment:1:6002": {aiTool: models.AiToolQodo, suggestionsAccepted: 2},
		// CodeRabbit: no acceptance markers = 0
		"github:GithubPrComment:1:6003": {aiTool: models.AiToolCodeRabbit, suggestionsAccepted: 0},
	}

	for _, r := range reviews {
		exp, ok := want[r.ReviewId]
		if !ok {
			t.Errorf("unexpected review with review_id %s", r.ReviewId)
			continue
		}
		assert.Equal(t, exp.aiTool, r.AiTool, "ai_tool for %s", r.ReviewId)
		assert.Equal(t, exp.suggestionsAccepted, r.SuggestionsAccepted, "suggestions_accepted for %s", r.ReviewId)
	}
}
