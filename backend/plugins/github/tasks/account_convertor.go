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

package tasks

import (
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

// repoAccountForConvert is the row projected by ConvertAccounts' query: every
// account referenced by the repo (from _tool_github_repo_accounts), enriched
// with profile detail from _tool_github_accounts when it was collected. The
// embedded NoPKModel carries the RawDataOrigin across to the domain row.
type repoAccountForConvert struct {
	Id        int
	Login     string
	Name      string
	Email     string
	AvatarUrl string
	common.NoPKModel
}

func init() {
	RegisterSubtaskMeta(&ConvertAccountsMeta)
}

var ConvertAccountsMeta = plugin.SubTaskMeta{
	Name:             "Convert Users",
	EntryPoint:       ConvertAccounts,
	EnabledByDefault: true,
	Description:      "Convert every account referenced by the repo (tool layer repo_accounts, enriched by github_accounts) into domain layer table accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{
		models.GithubRepoAccount{}.TableName(), // cursor (every user referenced by the repo)
		models.GithubAccount{}.TableName(),     // left-join enrichment (profile detail, optional)
		models.GithubAccountOrg{}.TableName()}, // org pluck
	ProductTables: []string{crossdomain.Account{}.TableName()},
}

func ConvertAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)

	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[repoAccountForConvert]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_ACCOUNT_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		// Source every account referenced by this repo from _tool_github_repo_accounts
		// (which the issue/PR/commit extractors populate for any author, assignee, or
		// merged-by user), and LEFT JOIN _tool_github_accounts for profile detail when it
		// was collected. This guarantees a domain `accounts` row for every CreatorId /
		// AuthorId the other convertors emit, instead of only for users who committed.
		// SQL is kept DB-agnostic (no backtick quoting, COALESCE not IFNULL) so it runs on
		// both MySQL and PostgreSQL.
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.Select(`_tool_github_repo_accounts.account_id AS id,
					_tool_github_repo_accounts.login AS login,
					COALESCE(ga.name, '') AS name,
					COALESCE(ga.email, '') AS email,
					COALESCE(ga.avatar_url, '') AS avatar_url,
					_tool_github_repo_accounts._raw_data_params AS _raw_data_params,
					_tool_github_repo_accounts._raw_data_table AS _raw_data_table,
					_tool_github_repo_accounts._raw_data_id AS _raw_data_id,
					_tool_github_repo_accounts._raw_data_remark AS _raw_data_remark`),
				dal.From(&models.GithubRepoAccount{}),
				dal.Join(`left join _tool_github_accounts ga on (
					ga.connection_id = _tool_github_repo_accounts.connection_id
					AND ga.id = _tool_github_repo_accounts.account_id
				)`),
				dal.Where(
					`_tool_github_repo_accounts.repo_github_id = ?
						AND _tool_github_repo_accounts.connection_id = ?
						AND _tool_github_repo_accounts.account_id > 0`,
					data.Options.GithubId,
					data.Options.ConnectionId,
				),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					// Incremental cursor intentionally tracks _tool_github_repo_accounts.updated_at
					// (repo membership), not _tool_github_accounts.updated_at (profile freshness):
					// account-detail re-enrichment is reconciled on the next full sync. Do not switch
					// this back to _tool_github_accounts — that is what left issue/PR-only authors
					// orphaned (#8886).
					clauses = append(clauses, dal.Where("_tool_github_repo_accounts.updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubUser *repoAccountForConvert) ([]interface{}, errors.Error) {
			// query related orgs
			var orgs []string
			err := db.Pluck(`org_login`, &orgs,
				dal.From(&models.GithubAccountOrg{}),
				dal.Where(`account_id = ? and connection_id = ?`, githubUser.Id, data.Options.ConnectionId),
			)
			if err != nil {
				return nil, err
			}
			var orgStr string
			if len(orgs) == 0 {
				orgStr = ``
			} else {
				orgStr = strings.Join(orgs, `,`)
				if len(orgStr) > 255 {
					orgStr = orgStr[:255]
				}
			}

			domainUser := &crossdomain.Account{
				DomainEntity: domainlayer.DomainEntity{Id: accountIdGen.Generate(data.Options.ConnectionId, githubUser.Id)},
				Email:        githubUser.Email,
				FullName:     githubUser.Name,
				UserName:     githubUser.Login,
				AvatarUrl:    githubUser.AvatarUrl,
				Organization: orgStr,
			}
			return []interface{}{
				domainUser,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
