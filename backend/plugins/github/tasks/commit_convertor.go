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
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertCommitsMeta)
}

var ConvertCommitsMeta = plugin.SubTaskMeta{
	Name:             "Convert Commits",
	EntryPoint:       ConvertCommits,
	EnabledByDefault: false,
	Description:      "Convert tool layer table github_commits into  domain layer table commits",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	DependencyTables: []string{
		models.GithubCommit{}.TableName(),     // cursor
		models.GithubRepoCommit{}.TableName(), // cursor
		//models.GithubRepo{}.TableName(),       // id generator, but config not regard as dependency
		RAW_COMMIT_TABLE},
	ProductTables: []string{
		code.Commit{}.TableName(),
		code.RepoCommit{}.TableName()},
}

func ConvertCommits(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	repoDidGen := didgen.NewDomainIdGenerator(&models.GithubRepo{})
	domainRepoId := repoDidGen.Generate(data.Options.ConnectionId, repoId)

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubCommit]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_COMMIT_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From("_tool_github_commits gc"),
				dal.Join(`left join _tool_github_repo_commits grc on (
					grc.commit_sha = gc.sha
				)`),
				dal.Select("gc.*"),
				dal.Where("grc.repo_id = ? AND grc.connection_id = ?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("(gc.updated_at >= ? OR grc.updated_at >= ?)", since, since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubCommit *models.GithubCommit) ([]interface{}, errors.Error) {
			domainCommit := &code.Commit{
				Sha:            githubCommit.Sha,
				Message:        githubCommit.Message,
				Additions:      githubCommit.Additions,
				Deletions:      githubCommit.Deletions,
				AuthorId:       githubCommit.AuthorEmail,
				AuthorName:     githubCommit.AuthorName,
				AuthorEmail:    githubCommit.AuthorEmail,
				AuthoredDate:   githubCommit.AuthoredDate,
				CommitterName:  githubCommit.CommitterName,
				CommitterEmail: githubCommit.CommitterEmail,
				CommittedDate:  githubCommit.CommittedDate,
				CommitterId:    githubCommit.CommitterEmail,
			}
			repoCommit := &code.RepoCommit{
				RepoId:    domainRepoId,
				CommitSha: domainCommit.Sha,
			}
			return []interface{}{
				domainCommit,
				repoCommit,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
