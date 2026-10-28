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

type githubCommitRow struct {
	Sha          string `gorm:"primaryKey;type:varchar(40);column:sha"`
	Message      string `gorm:"column:message"`
	AuthorName   string `gorm:"column:author_name"`
	AuthoredDate string `gorm:"column:authored_date"`
}

func (githubCommitRow) TableName() string { return "_tool_github_commits" }

func TestExtractAndConvertAiCommits(t *testing.T) {
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

	tester.FlushTabler(&domainCode.Commit{})
	tester.FlushTabler(&domainCode.RepoCommit{})
	tester.FlushTabler(&domainCode.PullRequest{})
	tester.FlushTabler(&domainCode.PullRequestCommit{})
	tester.FlushTabler(&models.AiCommit{})
	tester.FlushTabler(&domainCode.AiCommit{})
	tester.FlushTabler(&crossdomain.ProjectMapping{})
	tester.FlushTabler(&repoRow{})
	tester.FlushTabler(&githubCommitRow{})

	tester.ImportCsvIntoTabler("./raw_tables/ai_commits.csv", &domainCode.Commit{})
	tester.ImportCsvIntoTabler("./raw_tables/ai_repo_commits.csv", &domainCode.RepoCommit{})
	tester.ImportCsvIntoTabler("./raw_tables/ai_pull_requests.csv", &domainCode.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ai_pull_request_commits.csv", &domainCode.PullRequestCommit{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})
	tester.ImportCsvIntoTabler("./raw_tables/project_mapping.csv", &crossdomain.ProjectMapping{})

	tester.Subtask(tasks.ExtractAiCommitsMeta, taskData)
	tester.Subtask(tasks.ConvertAiCommitsMeta, taskData)

	var toolRows []models.AiCommit
	if err := tester.Dal.All(&toolRows); err != nil {
		t.Fatalf("Failed to query _tool_aireview_commits: %v", err)
	}
	if len(toolRows) != 4 {
		t.Fatalf("Expected 4 AI-positive tool commits (cursor, claude, copilot, coderabbit), got %d", len(toolRows))
	}

	bySha := make(map[string]models.AiCommit, len(toolRows))
	for _, row := range toolRows {
		bySha[row.CommitSha] = row
		if row.RepoId != testRepoId {
			t.Errorf("sha %s: expected repo_id=%q, got %q", row.CommitSha, testRepoId, row.RepoId)
		}
	}

	want := map[string]string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": models.AiToolCursor,
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": models.AiToolClaude,
		"cccccccccccccccccccccccccccccccccccccccc": models.AiToolCopilot,
		"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee": models.AiToolCodeRabbit,
	}
	for sha, tool := range want {
		got, ok := bySha[sha]
		if !ok {
			t.Errorf("missing tool-layer row for sha %s (want tool %s)", sha, tool)
			continue
		}
		if got.AiTool != tool {
			t.Errorf("sha %s: ai_tool=%q, want %q", sha, got.AiTool, tool)
		}
	}
	if _, ok := bySha["dddddddddddddddddddddddddddddddddddddddd"]; ok {
		t.Error("human commit mentioning Claude in the body must not be classified as AI")
	}

	var domainRows []domainCode.AiCommit
	if err := tester.Dal.All(&domainRows); err != nil {
		t.Fatalf("Failed to query ai_commits: %v", err)
	}
	if len(domainRows) != 4 {
		t.Fatalf("Expected 4 domain ai_commits, got %d", len(domainRows))
	}
	for _, row := range domainRows {
		if row.ProjectName != testProject {
			t.Errorf("domain sha %s: project_name=%q, want %q", row.CommitSha, row.ProjectName, testProject)
		}
	}
}

func TestConvertAiCommits_SkipsInRepoIdMode(t *testing.T) {
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

	tester.FlushTabler(&domainCode.Commit{})
	tester.FlushTabler(&domainCode.RepoCommit{})
	tester.FlushTabler(&domainCode.PullRequest{})
	tester.FlushTabler(&domainCode.PullRequestCommit{})
	tester.FlushTabler(&models.AiCommit{})
	tester.FlushTabler(&domainCode.AiCommit{})
	tester.FlushTabler(&repoRow{})
	tester.FlushTabler(&githubCommitRow{})

	tester.ImportCsvIntoTabler("./raw_tables/ai_commits.csv", &domainCode.Commit{})
	tester.ImportCsvIntoTabler("./raw_tables/ai_repo_commits.csv", &domainCode.RepoCommit{})
	tester.ImportCsvIntoTabler("./raw_tables/ai_pull_requests.csv", &domainCode.PullRequest{})
	tester.ImportCsvIntoTabler("./raw_tables/ai_pull_request_commits.csv", &domainCode.PullRequestCommit{})
	tester.ImportCsvIntoTabler("./raw_tables/ci_repos.csv", &repoRow{})

	tester.Subtask(tasks.ExtractAiCommitsMeta, taskData)
	tester.Subtask(tasks.ConvertAiCommitsMeta, taskData)

	var domainRows []domainCode.AiCommit
	if err := tester.Dal.All(&domainRows); err != nil {
		t.Fatalf("Failed to query ai_commits: %v", err)
	}
	if len(domainRows) != 0 {
		t.Errorf("Expected 0 domain ai_commits in single-repo mode, got %d", len(domainRows))
	}

	var toolRows []models.AiCommit
	if err := tester.Dal.All(&toolRows); err != nil {
		t.Fatalf("Failed to query _tool_aireview_commits: %v", err)
	}
	if len(toolRows) == 0 {
		t.Error("Expected tool-layer AI commits in single-repo mode")
	}
}
