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
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertReleasesMeta)
}

const (
	RAW_RELEASE_TABLE = "github_graphql_release"
)

var ConvertReleasesMeta = plugin.SubTaskMeta{
	Name:             "Convert Releases",
	EntryPoint:       ConvertRelease,
	EnabledByDefault: true,
	Description:      "Convert tool layer table github_releases into domain layer table releases",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{models.GithubRelease{}.TableName()},
	ProductTables:    []string{devops.CicdRelease{}.TableName()},
}

func ConvertRelease(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	params := GithubApiParams{
		ConnectionId: data.Options.ConnectionId,
		Name:         data.Options.Name,
	}

	releaseIdGen := didgen.NewDomainIdGenerator(&models.GithubRelease{})
	releaseScopeIdGen := didgen.NewDomainIdGenerator(&models.GithubRepo{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubRelease]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_RELEASE_TABLE,
			Params:         params,
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From(&models.GithubRelease{}),
				dal.Where(
					"published_at IS NOT NULL AND connection_id = ? AND github_id = ?",
					data.Options.ConnectionId, data.Options.GithubId,
				),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubRelease *models.GithubRelease) ([]interface{}, errors.Error) {
			release := &devops.CicdRelease{
				DomainEntity: domainlayer.DomainEntity{
					Id: releaseIdGen.Generate(githubRelease.ConnectionId, githubRelease.Id),
				},
				PublishedAt:  *githubRelease.PublishedAt,
				CicdScopeId:  releaseScopeIdGen.Generate(githubRelease.ConnectionId, githubRelease.GithubId),
				Name:         githubRelease.Name,
				DisplayTitle: githubRelease.Name,
				Description:  githubRelease.Description,
				URL:          githubRelease.URL,
				IsDraft:      githubRelease.IsDraft,
				IsLatest:     githubRelease.IsLatest,
				IsPrerelease: githubRelease.IsPrerelease,
				TagName:      githubRelease.TagName,
				CommitSha:    githubRelease.CommitSha,
				AuthorID:     githubRelease.AuthorID,
				RepoId:       releaseScopeIdGen.Generate(githubRelease.ConnectionId, githubRelease.GithubId),
			}
			return []interface{}{release}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
