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
	"encoding/json"
	"sort"
	"testing"
	"time"

	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/helpers/e2ehelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	contextimpl "github.com/apache/devlake/impls/context"
	"github.com/apache/devlake/plugins/github/models"
	"github.com/apache/devlake/plugins/github/tasks"
	"github.com/apache/devlake/plugins/github_graphql/impl"
	githubGraphQLTasks "github.com/apache/devlake/plugins/github_graphql/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

const (
	rawGraphqlPrsTable         = "_raw_github_graphql_prs"
	rawGraphqlIssuesTable      = "_raw_github_graphql_issues"
	rawGraphqlDeploymentsTable = "_raw_github_graphql_deployment"
	rawGraphqlReleasesTable    = "_raw_github_graphql_release"
)

// TestGithubGraphqlPrExtractorIncremental checks that an incremental run of "Extract Pull
// Requests" reads only the raw rows added since the previous run, replaces the labels of a
// pull request it extracts again, and leaves the other pull requests alone.
func TestGithubGraphqlPrExtractorIncremental(t *testing.T) {
	dataflowTester, taskData := newIncrementalExtractorTester(t, &models.GithubScopeConfig{
		PrType:      "^type/",
		PrComponent: "^component/",
	})
	const (
		unchangedPr = 1002
		updatedPr   = 1001
	)
	for _, table := range []schema.Tabler{
		&models.GithubPullRequest{}, &models.GithubPrLabel{}, &models.GithubPrReview{}, &models.GithubReviewer{},
		&models.GithubPrCommit{}, &models.GithubCommit{}, &models.GithubRepoAccount{},
	} {
		dataflowTester.FlushTabler(table)
	}
	dataflowTester.FlushRawTable(rawGraphqlPrsTable)
	created := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	appendRawRow(t, dataflowTester, rawGraphqlPrsTable, rawPr(updatedPr, "OPEN", created, "type/bug", "component/api"))
	appendRawRow(t, dataflowTester, rawGraphqlPrsTable, rawPr(unchangedPr, "MERGED", created, "type/feature"))

	// no stored state, so the first run extracts every raw row
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractPrsMeta, taskData, false)
	firstRun := githubPrs(t, dataflowTester)
	require.Len(t, firstRun, 2)
	assert.Equal(t, "type/bug", firstRun[updatedPr].Type)
	assert.Equal(t, "component/api", firstRun[updatedPr].Component)
	assert.Equal(t, []string{"component/api", "type/bug"}, githubPrLabels(t, dataflowTester, updatedPr))

	// the collector adds a raw row for the pull request that was relabelled and merged after the first run
	appendRawRow(t, dataflowTester, rawGraphqlPrsTable, rawPr(updatedPr, "MERGED", created.Add(time.Hour), "type/feature"))
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractPrsMeta, taskData, false)
	incrementalRun := githubPrs(t, dataflowTester)
	require.Len(t, incrementalRun, 2)
	assert.Equal(t, "MERGED", incrementalRun[updatedPr].State)
	assert.Equal(t, "type/feature", incrementalRun[updatedPr].Type)
	assert.Empty(t, incrementalRun[updatedPr].Component)
	assert.Equal(t, []string{"type/feature"}, githubPrLabels(t, dataflowTester, updatedPr), "the labels it lost must go")
	assert.Equal(t, []string{"type/feature"}, githubPrLabels(t, dataflowTester, unchangedPr))
	assert.Equal(t, firstRun[unchangedPr].UpdatedAt, incrementalRun[unchangedPr].UpdatedAt,
		"an incremental run must not write the other pull request again")

	// Type and Component depend on the scope config patterns: after a change every raw row is extracted again
	taskData.Options.ScopeConfig.PrType = "^kind/"
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractPrsMeta, taskData, false)
	reconfiguredRun := githubPrs(t, dataflowTester)
	require.Len(t, reconfiguredRun, 2)
	assert.Empty(t, reconfiguredRun[unchangedPr].Type)
	assert.Equal(t, "MERGED", reconfiguredRun[updatedPr].State, "the newest raw row must win")

	// a pull request that is missing in the tool layer comes back with a full sync, not with an incremental run
	require.NoError(t, dataflowTester.Db.Where("github_id = ?", unchangedPr).Delete(&models.GithubPullRequest{}).Error)
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractPrsMeta, taskData, false)
	assert.NotContains(t, githubPrs(t, dataflowTester), unchangedPr, "an incremental run must not read old raw rows again")
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractPrsMeta, taskData, true)
	fullSyncRun := githubPrs(t, dataflowTester)
	require.Len(t, fullSyncRun, 2)
	assert.Equal(t, "MERGED", fullSyncRun[updatedPr].State, "the newest raw row must win")
}

// TestGithubGraphqlIssueExtractorIncremental checks that an incremental run of "Extract Issues"
// reads only the raw rows added since the previous run, replaces the labels and assignees of an
// issue it extracts again, and leaves the other issues alone.
func TestGithubGraphqlIssueExtractorIncremental(t *testing.T) {
	dataflowTester, taskData := newIncrementalExtractorTester(t, &models.GithubScopeConfig{
		IssueSeverity: "^severity/",
		IssueTypeBug:  "^type/bug$",
	})
	const (
		unchangedIssue = 2002
		updatedIssue   = 2001
	)
	for _, table := range []schema.Tabler{
		&models.GithubIssue{}, &models.GithubIssueLabel{}, &models.GithubIssueAssignee{},
		&models.GithubMilestone{}, &models.GithubRepoAccount{},
	} {
		dataflowTester.FlushTabler(table)
	}
	dataflowTester.FlushRawTable(rawGraphqlIssuesTable)
	created := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	appendRawRow(t, dataflowTester, rawGraphqlIssuesTable, rawIssue(updatedIssue, "OPEN", created, []int{31, 32}, "type/bug", "severity/high"))
	appendRawRow(t, dataflowTester, rawGraphqlIssuesTable, rawIssue(unchangedIssue, "OPEN", created, []int{31}, "severity/low"))

	// no stored state, so the first run extracts every raw row
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractIssuesMeta, taskData, false)
	firstRun := githubIssues(t, dataflowTester)
	require.Len(t, firstRun, 2)
	assert.Equal(t, "severity/high", firstRun[updatedIssue].Severity)
	assert.Equal(t, []string{"severity/high", "type/bug"}, githubIssueLabels(t, dataflowTester, updatedIssue))
	assert.Equal(t, []int{31, 32}, githubIssueAssignees(t, dataflowTester, updatedIssue))

	// the collector adds a raw row for the issue that was relabelled and reassigned after the first run
	appendRawRow(t, dataflowTester, rawGraphqlIssuesTable, rawIssue(updatedIssue, "CLOSED", created.Add(time.Hour), []int{33}, "severity/low"))
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractIssuesMeta, taskData, false)
	incrementalRun := githubIssues(t, dataflowTester)
	require.Len(t, incrementalRun, 2)
	assert.Equal(t, "CLOSED", incrementalRun[updatedIssue].State)
	assert.Equal(t, "severity/low", incrementalRun[updatedIssue].Severity)
	assert.Equal(t, []string{"severity/low"}, githubIssueLabels(t, dataflowTester, updatedIssue), "the labels it lost must go")
	assert.Equal(t, []int{33}, githubIssueAssignees(t, dataflowTester, updatedIssue), "the assignees it lost must go")
	assert.Equal(t, []string{"severity/low"}, githubIssueLabels(t, dataflowTester, unchangedIssue))
	assert.Equal(t, []int{31}, githubIssueAssignees(t, dataflowTester, unchangedIssue))
	assert.Equal(t, firstRun[unchangedIssue].UpdatedAt, incrementalRun[unchangedIssue].UpdatedAt,
		"an incremental run must not write the other issue again")

	// the severity depends on the scope config patterns: after a change every raw row is extracted again
	taskData.Options.ScopeConfig.IssueSeverity = "^sev/"
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractIssuesMeta, taskData, false)
	reconfiguredRun := githubIssues(t, dataflowTester)
	require.Len(t, reconfiguredRun, 2)
	assert.Empty(t, reconfiguredRun[unchangedIssue].Severity)
	assert.Equal(t, "CLOSED", reconfiguredRun[updatedIssue].State, "the newest raw row must win")
}

// TestGithubGraphqlDeploymentExtractorIncremental checks that an incremental run of "Extract
// Deployments" reads only the raw rows added since the previous run.
func TestGithubGraphqlDeploymentExtractorIncremental(t *testing.T) {
	dataflowTester, taskData := newIncrementalExtractorTester(t, nil)
	const (
		unchangedDeployment = "DE_unchanged"
		updatedDeployment   = "DE_updated"
	)
	dataflowTester.FlushTabler(&models.GithubDeployment{})
	dataflowTester.FlushRawTable(rawGraphqlDeploymentsTable)
	created := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	appendRawRow(t, dataflowTester, rawGraphqlDeploymentsTable, rawDeployment(updatedDeployment, "IN_PROGRESS", created))
	appendRawRow(t, dataflowTester, rawGraphqlDeploymentsTable, rawDeployment(unchangedDeployment, "ACTIVE", created))

	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractDeploymentsMeta, taskData, false)
	firstRun := githubDeployments(t, dataflowTester)
	require.Len(t, firstRun, 2)
	assert.Equal(t, "IN_PROGRESS", firstRun[updatedDeployment].State)

	// the collector adds a raw row for the deployment that finished after the first run
	appendRawRow(t, dataflowTester, rawGraphqlDeploymentsTable, rawDeployment(updatedDeployment, "ACTIVE", created.Add(time.Hour)))
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractDeploymentsMeta, taskData, false)
	incrementalRun := githubDeployments(t, dataflowTester)
	require.Len(t, incrementalRun, 2)
	assert.Equal(t, "ACTIVE", incrementalRun[updatedDeployment].State)
	assert.Equal(t, firstRun[unchangedDeployment].UpdatedAt, incrementalRun[unchangedDeployment].UpdatedAt,
		"an incremental run must not write the other deployment again")

	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractDeploymentsMeta, taskData, true)
	fullSyncRun := githubDeployments(t, dataflowTester)
	require.Len(t, fullSyncRun, 2)
	assert.Equal(t, "ACTIVE", fullSyncRun[updatedDeployment].State, "the newest raw row must win")
}

// TestGithubGraphqlReleaseExtractorIncremental checks that an incremental run of "Extract
// Releases" reads only the raw rows added since the previous run.
func TestGithubGraphqlReleaseExtractorIncremental(t *testing.T) {
	dataflowTester, taskData := newIncrementalExtractorTester(t, nil)
	const (
		unchangedRelease = "RE_unchanged"
		updatedRelease   = "RE_updated"
	)
	dataflowTester.FlushTabler(&models.GithubRelease{})
	dataflowTester.FlushRawTable(rawGraphqlReleasesTable)
	created := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	appendRawRow(t, dataflowTester, rawGraphqlReleasesTable, rawRelease(updatedRelease, "v1.1.0-rc1", true, created))
	appendRawRow(t, dataflowTester, rawGraphqlReleasesTable, rawRelease(unchangedRelease, "v1.0.0", false, created))

	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractReleasesMeta, taskData, false)
	firstRun := githubReleases(t, dataflowTester)
	require.Len(t, firstRun, 2)
	assert.True(t, firstRun[updatedRelease].IsPrerelease)

	// the collector adds a raw row for the release that was promoted after the first run
	appendRawRow(t, dataflowTester, rawGraphqlReleasesTable, rawRelease(updatedRelease, "v1.1.0", false, created.Add(time.Hour)))
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractReleasesMeta, taskData, false)
	incrementalRun := githubReleases(t, dataflowTester)
	require.Len(t, incrementalRun, 2)
	assert.Equal(t, "v1.1.0", incrementalRun[updatedRelease].TagName)
	assert.False(t, incrementalRun[updatedRelease].IsPrerelease)

	// `updated_at` holds the release's own date, so a release that is missing in the tool layer
	// shows what a run read: it comes back with a full sync, not with an incremental run
	require.NoError(t, dataflowTester.Db.Where("id = ?", unchangedRelease).Delete(&models.GithubRelease{}).Error)
	appendRawRow(t, dataflowTester, rawGraphqlReleasesTable, rawRelease(updatedRelease, "v1.1.1", false, created.Add(2*time.Hour)))
	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractReleasesMeta, taskData, false)
	secondIncrementalRun := githubReleases(t, dataflowTester)
	assert.Equal(t, "v1.1.1", secondIncrementalRun[updatedRelease].TagName)
	assert.NotContains(t, secondIncrementalRun, unchangedRelease, "an incremental run must not read old raw rows again")

	runSubtaskKeepingState(t, dataflowTester, githubGraphQLTasks.ExtractReleasesMeta, taskData, true)
	fullSyncRun := githubReleases(t, dataflowTester)
	require.Len(t, fullSyncRun, 2)
	assert.Equal(t, "v1.1.1", fullSyncRun[updatedRelease].TagName, "the newest raw row must win")
}

func newIncrementalExtractorTester(t *testing.T, scopeConfig *models.GithubScopeConfig) (*e2ehelper.DataFlowTester, *tasks.GithubTaskData) {
	t.Helper()
	var githubGraphql impl.GithubGraphql
	dataflowTester := e2ehelper.NewDataFlowTester(t, "github_graphql", githubGraphql)
	dataflowTester.FlushTabler(&coreModels.SubtaskState{})
	dataflowTester.FlushTabler(&coreModels.CollectorLatestState{})
	return dataflowTester, &tasks.GithubTaskData{
		Options: &tasks.GithubOptions{
			ConnectionId: 1,
			Name:         "apache/devlake",
			GithubId:     384111310,
			ScopeConfig:  scopeConfig,
		},
	}
}

// runSubtaskKeepingState runs a subtask and keeps its state, which DataFlowTester.Subtask does
// not: it removes the state and forces a full sync on every call.
func runSubtaskKeepingState(
	t *testing.T, dataflowTester *e2ehelper.DataFlowTester, subtaskMeta plugin.SubTaskMeta, taskData *tasks.GithubTaskData, fullSync bool,
) {
	t.Helper()
	subtaskCtx := contextimpl.NewStandaloneSubTaskContext(
		context.Background(),
		runner.CreateBasicRes(dataflowTester.Cfg, dataflowTester.Log, dataflowTester.Db),
		subtaskMeta.Name,
		taskData,
		dataflowTester.Name,
		&coreModels.SyncPolicy{TriggerSyncPolicy: coreModels.TriggerSyncPolicy{FullSync: fullSync}},
	)
	require.NoError(t, subtaskMeta.EntryPoint(subtaskCtx))
}

// appendRawRow adds a raw row, created now, the way a collector does. A run reads the raw rows
// created between the start of the previous run and its own start, so the pauses keep the row
// clear of both.
func appendRawRow(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, rawTable string, body any) {
	t.Helper()
	data, err := json.Marshal(body)
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	require.NoError(t, dataflowTester.Db.Table(rawTable).Create(&helper.RawData{
		Params:    `{"ConnectionId":1,"Name":"apache/devlake"}`,
		Data:      data,
		CreatedAt: time.Now(),
	}).Error)
	time.Sleep(20 * time.Millisecond)
}

func rawPr(databaseId int, state string, updatedAt time.Time, labels ...string) *githubGraphQLTasks.GraphqlQueryPr {
	pr := &githubGraphQLTasks.GraphqlQueryPr{
		DatabaseId: databaseId,
		Number:     databaseId - 1000,
		State:      state,
		Title:      "pull request",
		CreatedAt:  time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC),
		UpdatedAt:  updatedAt,
	}
	for _, label := range labels {
		pr.Labels.Nodes = append(pr.Labels.Nodes, struct {
			Id   string
			Name string
		}{Name: label})
	}
	return pr
}

func rawIssue(databaseId int, state string, updatedAt time.Time, assignees []int, labels ...string) *githubGraphQLTasks.GraphqlQueryIssue {
	issue := &githubGraphQLTasks.GraphqlQueryIssue{
		DatabaseId: databaseId,
		Number:     databaseId - 2000,
		State:      state,
		Title:      "issue",
		CreatedAt:  time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC),
		UpdatedAt:  updatedAt,
	}
	for _, assignee := range assignees {
		account := githubGraphQLTasks.GraphqlInlineAccountQuery{}
		account.Id = assignee
		account.Login = "assignee"
		issue.AssigneeList.Assignees = append(issue.AssigneeList.Assignees, account)
	}
	for _, label := range labels {
		issue.Labels.Nodes = append(issue.Labels.Nodes, struct {
			Id   string
			Name string
		}{Name: label})
	}
	return issue
}

func rawDeployment(id string, state string, updatedAt time.Time) *githubGraphQLTasks.GraphqlQueryDeploymentDeployment {
	deployment := &githubGraphQLTasks.GraphqlQueryDeploymentDeployment{
		Id:          id,
		Environment: "production",
		State:       state,
		CreatedAt:   time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC),
		UpdatedAt:   updatedAt,
	}
	deployment.Repository.Url = "https://github.com/apache/devlake"
	return deployment
}

func rawRelease(id string, tagName string, isPrerelease bool, updatedAt time.Time) *githubGraphQLTasks.GraphqlQueryRelease {
	return &githubGraphQLTasks.GraphqlQueryRelease{
		Id:           id,
		TagName:      tagName,
		Name:         tagName,
		IsPrerelease: isPrerelease,
		CreatedAt:    time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC),
		UpdatedAt:    updatedAt,
	}
}

func githubPrs(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) map[int]models.GithubPullRequest {
	t.Helper()
	var prs []models.GithubPullRequest
	require.NoError(t, dataflowTester.Db.Find(&prs).Error)
	prsById := make(map[int]models.GithubPullRequest, len(prs))
	for _, pr := range prs {
		prsById[pr.GithubId] = pr
	}
	return prsById
}

func githubPrLabels(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, pullId int) []string {
	t.Helper()
	var labels []string
	require.NoError(t, dataflowTester.Db.Model(&models.GithubPrLabel{}).Where("pull_id = ?", pullId).Pluck("label_name", &labels).Error)
	sort.Strings(labels)
	return labels
}

func githubIssues(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) map[int]models.GithubIssue {
	t.Helper()
	var issues []models.GithubIssue
	require.NoError(t, dataflowTester.Db.Find(&issues).Error)
	issuesById := make(map[int]models.GithubIssue, len(issues))
	for _, issue := range issues {
		issuesById[issue.GithubId] = issue
	}
	return issuesById
}

func githubIssueLabels(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, issueId int) []string {
	t.Helper()
	var labels []string
	require.NoError(t, dataflowTester.Db.Model(&models.GithubIssueLabel{}).Where("issue_id = ?", issueId).Pluck("label_name", &labels).Error)
	sort.Strings(labels)
	return labels
}

func githubIssueAssignees(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, issueId int) []int {
	t.Helper()
	var assignees []int
	require.NoError(t, dataflowTester.Db.Model(&models.GithubIssueAssignee{}).Where("issue_id = ?", issueId).Pluck("assignee_id", &assignees).Error)
	sort.Ints(assignees)
	return assignees
}

func githubDeployments(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) map[string]models.GithubDeployment {
	t.Helper()
	var deployments []models.GithubDeployment
	require.NoError(t, dataflowTester.Db.Find(&deployments).Error)
	deploymentsById := make(map[string]models.GithubDeployment, len(deployments))
	for _, deployment := range deployments {
		deploymentsById[deployment.Id] = deployment
	}
	return deploymentsById
}

func githubReleases(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) map[string]models.GithubRelease {
	t.Helper()
	var releases []models.GithubRelease
	require.NoError(t, dataflowTester.Db.Find(&releases).Error)
	releasesById := make(map[string]models.GithubRelease, len(releases))
	for _, release := range releases {
		releasesById[release.Id] = release
	}
	return releasesById
}
