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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/github/impl"
	"github.com/apache/incubator-devlake/plugins/github/models"
	"github.com/apache/incubator-devlake/plugins/github/tasks"
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
