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
	"fmt"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

type GithubApiRepo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	GithubId    int    `json:"id"`
	HTMLUrl     string `json:"html_url"`
	Language    string `json:"language"`
	Description string `json:"description"`
	Owner       *GithubAccountResponse
	Parent      *GithubApiRepo      `json:"parent"`
	CreatedAt   common.Iso8601Time  `json:"created_at"`
	UpdatedAt   *common.Iso8601Time `json:"updated_at"`
	CloneUrl    string              `json:"clone_url"`
}

func init() {
	RegisterSubtaskMeta(&ConvertRepoMeta)
}

var ConvertRepoMeta = plugin.SubTaskMeta{
	Name:             "Convert Repos",
	EntryPoint:       ConvertRepo,
	EnabledByDefault: true,
	Description:      "Convert tool layer table github_repos into domain layer table repos and boards",
	DomainTypes: []string{
		plugin.DOMAIN_TYPE_CODE,
		plugin.DOMAIN_TYPE_TICKET,
		plugin.DOMAIN_TYPE_CICD,
		plugin.DOMAIN_TYPE_CODE_REVIEW,
		plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{
		//models.GithubRepo{}.TableName(), // config will not regard as dependency
		//RAW_REPOSITORIES_TABLE,
	},
	ProductTables: []string{
		code.Repo{}.TableName(),
		ticket.Board{}.TableName(),
		crossdomain.BoardRepo{}.TableName(),
		devops.CicdScope{}.TableName()},
}

func ConvertRepo(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)
	repoId := data.Options.GithubId

	repoIdGen := didgen.NewDomainIdGenerator(&models.GithubRepo{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.GithubRepo]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          models.GithubRepo{}.TableName(),
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From(&models.GithubRepo{}),
				dal.Where("github_id = ? and connection_id = ?", repoId, data.Options.ConnectionId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(repository *models.GithubRepo) ([]interface{}, errors.Error) {
			domainRepository := &code.Repo{
				DomainEntity: domainlayer.DomainEntity{
					Id: repoIdGen.Generate(data.Options.ConnectionId, repository.GithubId),
				},
				Name:        repository.FullName,
				Url:         repository.HTMLUrl,
				Description: repository.Description,
				ForkedFrom:  repository.ParentHTMLUrl,
				Language:    repository.Language,
				CreatedDate: repository.CreatedDate,
				UpdatedDate: repository.UpdatedDate,
			}
			domainBoard := &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{
					Id: repoIdGen.Generate(data.Options.ConnectionId, repository.GithubId),
				},
				Name:        repository.FullName,
				Url:         fmt.Sprintf("%s/%s", repository.HTMLUrl, "issues"),
				Description: repository.Description,
				CreatedDate: repository.CreatedDate,
			}
			domainBoardRepo := &crossdomain.BoardRepo{
				BoardId: repoIdGen.Generate(data.Options.ConnectionId, repository.GithubId),
				RepoId:  repoIdGen.Generate(data.Options.ConnectionId, repository.GithubId),
			}
			domainCicdScope := &devops.CicdScope{
				DomainEntity: domainlayer.DomainEntity{
					Id: repoIdGen.Generate(data.Options.ConnectionId, repository.GithubId),
				},
				Name:        repository.FullName,
				Url:         repository.HTMLUrl,
				Description: repository.Description,
				CreatedDate: repository.CreatedDate,
				UpdatedDate: repository.UpdatedDate,
			}
			return []interface{}{
				domainRepository,
				domainBoard,
				domainBoardRepo,
				domainCicdScope,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
