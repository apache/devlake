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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/github/impl"
	"github.com/apache/devlake/plugins/github/models"
	"github.com/apache/devlake/plugins/github/tasks"
	graphqlTasks "github.com/apache/devlake/plugins/github_graphql/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountDataFlow(t *testing.T) {
	var plugin impl.Github
	dataflowTester := e2ehelper.NewDataFlowTester(t, "github", plugin)

	taskData := &tasks.GithubTaskData{
		Options: &tasks.GithubOptions{
			ConnectionId: 1,
			Name:         "panjf2000/ants",
			GithubId:     134018330,
		},
	}

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_github_api_accounts.csv", "_raw_github_api_accounts")

	// verify extraction
	dataflowTester.FlushTabler(&models.GithubAccount{})
	dataflowTester.Subtask(tasks.ExtractAccountsMeta, taskData)
	dataflowTester.VerifyTable(
		models.GithubAccount{},
		"./snapshot_tables/_tool_github_account.csv",
		[]string{
			"connection_id",
			"id",
			"login",
			"name",
			"company",
			"email",
			"avatar_url",
			"url",
			"html_url",
			"type",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_github_api_account_orgs.csv", "_raw_github_api_account_orgs")
	// verify extraction
	dataflowTester.FlushTabler(&models.GithubAccountOrg{})
	dataflowTester.Subtask(tasks.ExtractAccountOrgMeta, taskData)
	dataflowTester.VerifyTable(
		models.GithubAccountOrg{},
		"./snapshot_tables/_tool_github_account_orgs.csv",
		[]string{
			"connection_id",
			"account_id",
			"org_id",
			"org_login",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)

	// ConvertAccountsMeta only convert the account in this repo
	dataflowTester.ImportCsvIntoTabler("./raw_tables/_tool_github_repo_accounts.csv", &models.GithubRepoAccount{})

	// verify converter
	dataflowTester.FlushTabler(&crossdomain.Account{})
	dataflowTester.Subtask(tasks.ConvertAccountsMeta, taskData)
	dataflowTester.VerifyTable(
		crossdomain.Account{},
		"./snapshot_tables/account.csv",
		[]string{
			"id",
			"email",
			"full_name",
			"user_name",
			"avatar_url",
			"organization",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)

	// Referential-integrity invariant (#8886): every account the repo references in
	// _tool_github_repo_accounts must have a domain `accounts` row, so issues.creator_id /
	// pull_requests.author_id / merged_by_id never point at a missing account. We generate
	// the domain id with the SAME generator the issue/PR convertors use, so this is a
	// faithful proxy for the FK join the issue reported as broken. It also fails loudly if a
	// future change shrinks ConvertAccounts' coverage or diverges the id generation.
	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})
	var repoAccounts []models.GithubRepoAccount
	require.NoError(t, dataflowTester.Dal.All(&repoAccounts,
		dal.Where("repo_github_id = ? AND connection_id = ? AND account_id > 0",
			taskData.Options.GithubId, taskData.Options.ConnectionId),
	))
	require.NotEmpty(t, repoAccounts, "fixture must reference at least one account")
	sawOrphanCase := false
	for _, ra := range repoAccounts {
		if ra.Login == "milichev" {
			sawOrphanCase = true // the non-committer author from the issue repro
		}
		domainId := accountIdGen.Generate(taskData.Options.ConnectionId, ra.AccountId)
		count, err := dataflowTester.Dal.Count(
			dal.From(&crossdomain.Account{}),
			dal.Where("id = ?", domainId),
		)
		require.NoError(t, err)
		assert.Equalf(t, int64(1), count,
			"orphan FK: repo account %q (id=%d) has no domain accounts row %q", ra.Login, ra.AccountId, domainId)
	}
	assert.True(t, sawOrphanCase, "fixture should include the non-committer orphan case (milichev)")
}

// Without profile enrichment, bot classification must survive every repository's
// conversion of the shared account, including REST aliases of a GraphQL actor.
func TestAccountDataFlowBotWithoutProfile(t *testing.T) {
	t.Setenv("GITHUB_PR_EXCLUDELIST", "")
	tasks.ResetExcludedUsernamesForTest()
	t.Cleanup(tasks.ResetExcludedUsernamesForTest)
	df := e2ehelper.NewDataFlowTester(t, "github", impl.Github{})
	for _, model := range []interface{ TableName() string }{
		&models.GithubAccount{}, &models.GithubAccountOrg{}, &models.GithubRepoAccount{},
		&crossdomain.Account{}, &models.GithubPullRequest{}, &models.GithubPrReview{},
		&models.GithubReviewer{}, &models.GithubPrLabel{}, &models.GithubPrCommit{},
		&models.GithubCommit{}, &models.GithubPrComment{},
	} {
		df.FlushTabler(model)
	}
	graphqlTable := "_raw_" + graphqlTasks.RAW_PRS_TABLE
	restTable := "_raw_" + tasks.RAW_PR_REVIEW_COMMENTS_TABLE
	df.FlushRawTable(graphqlTable)
	df.FlushRawTable(restTable)

	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})
	for step, run := range []struct {
		repoID  int
		graphql bool
	}{
		{101, true}, {102, false}, {101, false}, {102, true},
	} {
		t.Run(fmt.Sprintf("%d/repo-%d/graphql-%t", step, run.repoID, run.graphql), func(t *testing.T) {
			name := fmt.Sprintf("owner/repo-%d", run.repoID)
			data := &tasks.GithubTaskData{Options: &tasks.GithubOptions{
				ConnectionId: 1, GithubId: run.repoID, Name: name,
			}}
			params, err := json.Marshal(tasks.GithubApiParams{ConnectionId: 1, Name: name})
			require.NoError(t, err)
			wantBotLogin := "Copilot[bot]"
			if run.graphql {
				// Match the real decoder's overlapping fragment fields and the
				// collector's JSON serialization, rather than a Bot-only fragment.
				raw := fmt.Sprintf(`{
					"DatabaseId":%d, "Number":1, "State":"OPEN",
					"CreatedAt":"2026-01-01T00:00:00Z", "UpdatedAt":"2026-01-02T00:00:00Z",
					"Author":{
						"TypeName":"User", "Login":"octocat", "Id":583231,
						"Bot":{"Login":"octocat", "Id":583231}
					},
					"Reviews":{"Nodes":[{
						"databaseId":%d, "state":"COMMENTED",
						"Author":{
							"TypeName":"Bot", "Login":"copilot-pull-request-reviewer", "Id":175728472,
							"Bot":{"Login":"copilot-pull-request-reviewer", "Id":175728472}
						}
					}]}
				}`, run.repoID, run.repoID)
				var pr graphqlTasks.GraphqlQueryPr
				require.NoError(t, json.Unmarshal([]byte(raw), &pr))
				serialized, err := json.Marshal(pr)
				require.NoError(t, err)
				require.NoError(t, df.Db.Table(graphqlTable).Create(&api.RawData{
					Params: string(params), Data: serialized, CreatedAt: time.Now().UTC(),
				}).Error)
				df.Subtask(graphqlTasks.ExtractPrsMeta, data)
				wantBotLogin = "copilot-pull-request-reviewer[bot]"
			} else {
				for i, user := range []string{
					`{"login":"Copilot","id":175728472,"type":"Bot"}`,
					`{"login":"octocat","id":583231,"type":"User"}`,
				} {
					raw := fmt.Sprintf(`{"id":%d,"body":"review","user":%s,"pull_request_url":"https://api.github.com/repos/%s/pulls/1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z"}`, run.repoID*10+i, user, name)
					require.NoError(t, df.Db.Table(restTable).Create(&api.RawData{
						Params: string(params), Data: []byte(raw), CreatedAt: time.Now().UTC(),
					}).Error)
				}
				df.Subtask(tasks.ExtractApiPrReviewCommentsMeta, data)
			}
			df.Subtask(tasks.ConvertAccountsMeta, data)
			for _, expected := range []struct {
				id    int
				login string
				bot   bool
			}{
				{175728472, wantBotLogin, true}, {583231, "octocat", false},
			} {
				var mapping models.GithubRepoAccount
				require.NoError(t, df.Dal.First(&mapping, dal.Where("connection_id = ? AND repo_github_id = ? AND account_id = ?", 1, run.repoID, expected.id)))
				assert.Equal(t, expected.login, mapping.Login)
				var account crossdomain.Account
				require.NoError(t, df.Dal.First(&account, dal.Where("id = ?", accountIdGen.Generate(uint64(1), expected.id))))
				assert.Equal(t, expected.login, account.UserName)
				assert.Equal(t, expected.bot, account.IsBot)
			}
		})
	}
	count, err := df.Dal.Count(dal.From(&models.GithubAccount{}))
	require.NoError(t, err)
	assert.Zero(t, count, "profile enrichment must remain absent throughout the test")
	count, err = df.Dal.Count(dal.From(&crossdomain.Account{}))
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "repositories must share the same numeric account identities")
}
