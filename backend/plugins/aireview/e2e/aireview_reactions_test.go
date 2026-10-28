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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/impls/dalgorm"
	"github.com/apache/devlake/plugins/aireview/impl"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/apache/devlake/plugins/aireview/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnrichGithubReviewReactions verifies that GitHub PR comment reactions are enriched
// into _tool_aireview_reviews from the raw GitHub API data table.
//
// Synthetic data is modelled on the real reaction found at:
// https://github.com/meilisearch/meilisearch-rust/pull/756#discussion_r2736607742
// (CodeRabbit review comment with 1 thumbsup).
//
// Test data covers three cases:
//   - PR 1001 comment 5001: +1=2 (two thumbsups)
//   - PR 1002 comment 5003: +1=1, -1=1 (mixed)
//   - PR 1003 comment 5005: no reactions
func TestEnrichGithubReviewReactions(t *testing.T) {
	var plug impl.AiReview
	dataflowTester := e2ehelper.NewDataFlowTester(t, "aireview", plug)

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      "github:GithubRepo:1:100",
			ScopeConfig: models.GetDefaultScopeConfig(),
		},
	}
	require.NoError(t, tasks.CompilePatterns(taskData))

	// Flush and load domain tables
	dataflowTester.FlushTabler(&crossdomain.Account{})
	dataflowTester.FlushTabler(&code.PullRequest{})
	dataflowTester.FlushTabler(&code.PullRequestComment{})
	dataflowTester.FlushTabler(&models.AiReview{})

	dataflowTester.ImportCsvIntoTabler("./raw_tables/pull_requests.csv", &code.PullRequest{})
	dataflowTester.ImportCsvIntoTabler("./raw_tables/pull_request_comments.csv", &code.PullRequestComment{})

	// Load raw GitHub API comments table that contains $.reactions JSON
	dataflowTester.FlushRawTable("_raw_github_api_comments")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_github_api_comments.csv", "_raw_github_api_comments")

	// Step 1: extract reviews from comments
	dataflowTester.Subtask(tasks.ExtractAiReviewsMeta, taskData)

	// Step 2: enrich with reactions from raw table
	dataflowTester.Subtask(tasks.EnrichGithubReviewReactionsMeta, taskData)

	// Verify reactions were written back to _tool_aireview_reviews
	var reviews []models.AiReview
	require.NoError(t, dataflowTester.Dal.All(&reviews))
	require.Equal(t, 3, len(reviews), "expected 3 AI reviews")

	// Build lookup by domain comment ID for deterministic assertions
	type expected struct {
		totalCount int
		thumbsUp   int
		thumbsDown int
	}
	want := map[string]expected{
		"github:GithubPrComment:1:5001": {totalCount: 2, thumbsUp: 2, thumbsDown: 0},
		"github:GithubPrComment:1:5003": {totalCount: 2, thumbsUp: 1, thumbsDown: 1},
		"github:GithubPrComment:1:5005": {totalCount: 0, thumbsUp: 0, thumbsDown: 0},
	}

	for _, r := range reviews {
		exp, ok := want[r.ReviewId]
		if !ok {
			t.Errorf("unexpected review with review_id %s", r.ReviewId)
			continue
		}
		assert.Equal(t, exp.totalCount, r.ReactionsTotalCount, "total_count for %s", r.ReviewId)
		assert.Equal(t, exp.thumbsUp, r.ReactionsThumbsUp, "thumbs_up for %s", r.ReviewId)
		assert.Equal(t, exp.thumbsDown, r.ReactionsThumbsDown, "thumbs_down for %s", r.ReviewId)
	}
}

// TestEnrichGitlabReviewReactions verifies that GitLab award emoji reactions are fetched
// and stored against AI reviews.
//
// It uses a mock HTTP server serving a pre-recorded response captured from
// gitlab.cee.redhat.com MR #42 note 20330331 (cmulliga added a thumbsup).
// The fixture is at raw_tables/gitlab_award_emoji_note_20330331.json.
func TestEnrichGitlabReviewReactions(t *testing.T) {
	// Load the fixture — real award emoji response captured from gitlab.cee.redhat.com.
	// Project: devtools/developer-practices-documentation (ID 105624)
	// MR: #42 (gitlab_id 2170267), note: 20330331 (1 thumbsup from cmulliga)
	fixture, err := os.ReadFile("./raw_tables/gitlab_award_emoji_note_20330331.json")
	require.NoError(t, err)

	// Constants matching the captured fixture data.
	var connID uint64 = 1
	const (
		projectID  = 105624
		mrGitlabID = 2170267
		mrIID      = 42
		noteID     = 20330331
	)

	// Start a mock HTTP server that serves the award emoji fixture.
	// The task calls: GET {endpoint}/projects/{id}/merge_requests/{iid}/notes/{noteId}/award_emoji
	expectedPath := fmt.Sprintf("/projects/%d/merge_requests/%d/notes/%d/award_emoji", projectID, mrIID, noteID)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	var plug impl.AiReview
	dataflowTester := e2ehelper.NewDataFlowTester(t, "aireview", plug)

	// Register the encdec GORM serializer so the task can decrypt the connection token.
	encryptionSecret := dataflowTester.Cfg.GetString("ENCRYPTION_SECRET")
	require.NotEmpty(t, encryptionSecret, "ENCRYPTION_SECRET must be set in .env")
	dalgorm.Init(encryptionSecret)

	// Pre-encrypt a dummy token; the mock server does not validate it.
	encToken, encErr := plugin.Encrypt(encryptionSecret, "mock-token")
	require.NoError(t, encErr)

	domainCommentID := fmt.Sprintf("gitlab:GitlabMrComment:%d:%d", connID, noteID)
	repoID := "gitlab:GitlabRepo:1:100"

	// Create minimal _tool_gitlab_connections and insert connection pointing at mock server.
	err = dataflowTester.Dal.Exec(`
		CREATE TABLE IF NOT EXISTS _tool_gitlab_connections (
			id BIGINT UNSIGNED PRIMARY KEY,
			endpoint VARCHAR(255),
			token TEXT
		)`)
	require.NoError(t, err)
	dataflowTester.Dal.Exec("DELETE FROM _tool_gitlab_connections WHERE id = ?", connID)
	err = dataflowTester.Dal.Exec(
		"INSERT INTO _tool_gitlab_connections (id, endpoint, token) VALUES (?, ?, ?)",
		connID, srv.URL, encToken,
	)
	require.NoError(t, err)

	// Create minimal _tool_gitlab_merge_requests and insert test MR.
	err = dataflowTester.Dal.Exec(`
		CREATE TABLE IF NOT EXISTS _tool_gitlab_merge_requests (
			connection_id BIGINT UNSIGNED,
			gitlab_id INT,
			iid INT,
			project_id INT,
			PRIMARY KEY (connection_id, gitlab_id)
		)`)
	require.NoError(t, err)
	dataflowTester.Dal.Exec("DELETE FROM _tool_gitlab_merge_requests WHERE connection_id = ? AND gitlab_id = ?", connID, mrGitlabID)
	err = dataflowTester.Dal.Exec(
		"INSERT INTO _tool_gitlab_merge_requests (connection_id, gitlab_id, iid, project_id) VALUES (?, ?, ?, ?)",
		connID, mrGitlabID, mrIID, projectID,
	)
	require.NoError(t, err)

	// Create minimal _tool_gitlab_mr_comments and insert the note under test.
	err = dataflowTester.Dal.Exec(`
		CREATE TABLE IF NOT EXISTS _tool_gitlab_mr_comments (
			connection_id BIGINT UNSIGNED,
			gitlab_id INT,
			merge_request_id INT,
			merge_request_iid INT,
			PRIMARY KEY (connection_id, gitlab_id)
		)`)
	require.NoError(t, err)
	dataflowTester.Dal.Exec("DELETE FROM _tool_gitlab_mr_comments WHERE connection_id = ? AND gitlab_id = ?", connID, noteID)
	err = dataflowTester.Dal.Exec(
		"INSERT INTO _tool_gitlab_mr_comments (connection_id, gitlab_id, merge_request_id, merge_request_iid) VALUES (?, ?, ?, ?)",
		connID, noteID, mrGitlabID, mrIID,
	)
	require.NoError(t, err)

	// Seed _tool_aireview_reviews with one GitLab review pointing at the test note.
	dataflowTester.FlushTabler(&models.AiReview{})
	testReview := &models.AiReview{
		Id:             "aireview:gitlab-e2e-test",
		PullRequestId:  fmt.Sprintf("gitlab:GitlabMergeRequest:%d:%d", connID, mrGitlabID),
		RepoId:         repoID,
		AiTool:         models.AiToolCodeRabbit,
		AiToolUser:     "coderabbitai",
		ReviewId:       domainCommentID,
		Body:           "test AI review comment",
		SourcePlatform: "gitlab",
		CreatedDate:    time.Now(),
	}
	err = dataflowTester.Dal.CreateOrUpdate(testReview)
	require.NoError(t, err)

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      repoID,
			ScopeConfig: models.GetDefaultScopeConfig(),
		},
	}
	require.NoError(t, tasks.CompilePatterns(taskData))

	// Run the GitLab reaction enrichment task against the mock server.
	dataflowTester.Subtask(tasks.EnrichGitlabReviewReactionsMeta, taskData)

	// Verify the review was enriched with the reactions from the fixture.
	// Fixture has 1 thumbsup (cmulliga on note 20330331).
	var reviews []models.AiReview
	require.NoError(t, dataflowTester.Dal.All(&reviews))
	require.Equal(t, 1, len(reviews))

	r := reviews[0]
	assert.Equal(t, 1, r.ReactionsTotalCount, "expected total_count=1 from fixture")
	assert.Equal(t, 1, r.ReactionsThumbsUp, "expected thumbsup=1 from fixture")
	assert.Equal(t, 0, r.ReactionsThumbsDown, "expected thumbsdown=0 from fixture")
}
