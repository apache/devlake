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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/utils"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/github/models"
	"github.com/apache/incubator-devlake/plugins/github/tasks"
)

func MakeDataSourcePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	// load connection, scope and scopeConfig from the db
	connection, err := dsHelper.ConnSrv.FindByPk(connectionId)
	if err != nil {
		return nil, nil, err
	}
	scopeDetails, err := dsHelper.ScopeSrv.MapScopeDetails(connectionId, bpScopes)
	if err != nil {
		return nil, nil, err
	}

	// needed for the connection to populate its access tokens
	// if AppKey authentication method is selected
	_, err = helper.NewApiClientFromConnection(context.TODO(), basicRes, connection)
	if err != nil {
		return nil, nil, err
	}

	plan, err := makeDataSourcePipelinePlanV200(subtaskMetas, scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}
	scopes, err := makeScopesV200(scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}

	return plan, scopes, nil
}

func makeDataSourcePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	scopeDetails []*srvhelper.ScopeDetail[models.GithubRepo, models.GithubScopeConfig],
	connection *models.GithubConnection,
) (coreModels.PipelinePlan, errors.Error) {
	plan := make(coreModels.PipelinePlan, 0, len(scopeDetails)+1)
	for i, scopeDetail := range scopeDetails {
		if i == len(plan) {
			plan = append(plan, nil)
		}
		githubRepo, scopeConfig := scopeDetail.Scope, scopeDetail.ScopeConfig
		stage := plan[i]
		if stage == nil {
			stage = coreModels.PipelineStage{}
		}
		// refdiff
		if scopeConfig != nil && scopeConfig.Refdiff != nil {
			// add a new task to next stage
			j := i + 1
			if j == len(plan) {
				plan = append(plan, nil)
			}
			refdiffOp := scopeConfig.Refdiff
			refdiffOp["repoId"] = didgen.NewDomainIdGenerator(&models.GithubRepo{}).Generate(connection.ID, githubRepo.GithubId)
			plan[j] = coreModels.PipelineStage{
				{
					Plugin:  "refdiff",
					Options: refdiffOp,
				},
			}
			scopeConfig.Refdiff = nil
		}

		// construct task options for github
		op := &tasks.GithubOptions{
			ConnectionId: githubRepo.ConnectionId,
			GithubId:     githubRepo.GithubId,
			Name:         githubRepo.FullName,
			FullName:     githubRepo.FullName,
		}
		stage, err := addGithub(subtaskMetas, connection, scopeConfig.Entities, stage, op)
		if err != nil {
			return nil, err
		}

		// add gitex stage
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) || len(scopeConfig.Entities) == 0 {
			cloneUrl, err := errors.Convert01(url.Parse(githubRepo.CloneUrl))
			if err != nil {
				return nil, err
			}
			token := strings.Split(connection.Token, ",")[0]
			cloneUrl.User = url.UserPassword("git", token)
			stage = append(stage, &coreModels.PipelineTask{
				Plugin: "gitextractor",
				Options: map[string]interface{}{
					"url":                   cloneUrl.String(),
					"name":                  githubRepo.FullName,
					"fullName":              githubRepo.FullName,
					"repoId":                didgen.NewDomainIdGenerator(&models.GithubRepo{}).Generate(connection.ID, githubRepo.GithubId),
					"proxy":                 connection.Proxy,
					"connectionId":          githubRepo.ConnectionId,
					"pluginName":            "github",
					"excludeFileExtensions": scopeConfig.PrSizeExcludedFileExtensions,
				},
			})

		}
		plan[i] = stage
	}
	return plan, nil
}

func makeScopesV200(
	scopeDetails []*srvhelper.ScopeDetail[models.GithubRepo, models.GithubScopeConfig],
	connection *models.GithubConnection,
) ([]plugin.Scope, errors.Error) {
	scopes := make([]plugin.Scope, 0)
	for _, scopeDetail := range scopeDetails {
		githubRepo, scopeConfig := scopeDetail.Scope, scopeDetail.ScopeConfig
		// if no entities specified, use all entities enabled by default
		if len(scopeConfig.Entities) == 0 {
			scopeConfig.Entities = plugin.DOMAIN_TYPES
		}
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE_REVIEW) ||
			utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) ||
			utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CROSS) {
			// if we don't need to collect gitex, we need to add repo to scopes here
			scopeRepo := &code.Repo{
				DomainEntity: domainlayer.DomainEntity{
					Id: didgen.NewDomainIdGenerator(&models.GithubRepo{}).Generate(connection.ID, githubRepo.GithubId),
				},
				Name: githubRepo.FullName,
			}
			if githubRepo.ParentHTMLUrl != "" {
				scopeRepo.ForkedFrom = githubRepo.ParentHTMLUrl
			}
			scopes = append(scopes, scopeRepo)
		}
		// add cicd_scope to scopes
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CICD) {
			scopeCICD := &devops.CicdScope{
				DomainEntity: domainlayer.DomainEntity{
					Id: didgen.NewDomainIdGenerator(&models.GithubRepo{}).Generate(connection.ID, githubRepo.GithubId),
				},
				Name: githubRepo.FullName,
			}
			scopes = append(scopes, scopeCICD)
		}
		// add board to scopes
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_TICKET) {
			scopeTicket := &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{
					Id: didgen.NewDomainIdGenerator(&models.GithubRepo{}).Generate(connection.ID, githubRepo.GithubId),
				},
				Name: githubRepo.FullName,
			}
			scopes = append(scopes, scopeTicket)
		}
	}
	return scopes, nil
}

func addGithub(
	subtaskMetas []plugin.SubTaskMeta,
	connection *models.GithubConnection,
	entities []string,
	stage coreModels.PipelineStage,
	options *tasks.GithubOptions,
) (coreModels.PipelineStage, errors.Error) {
	// construct github(graphql) task
	if connection.EnableGraphql {
		// FIXME this need fix when 2 plugins merged
		p, err := plugin.GetPlugin(`github_graphql`)
		if err != nil {
			return nil, err
		}
		if pluginGq, ok := p.(plugin.PluginTask); ok {
			task, err := helper.MakePipelinePlanTask("github_graphql", pluginGq.SubTaskMetas(), entities, options)
			if err != nil {
				return nil, err
			}
			stage = append(stage, task)
		} else {
			return nil, errors.BadInput.New("plugin github_graphql does not support SubTaskMetas")
		}
	} else {
		task, err := helper.MakePipelinePlanTask("github", subtaskMetas, entities, options)
		if err != nil {
			return nil, err
		}
		stage = append(stage, task)
	}
	return stage, nil
}

func getApiRepo(
	op *tasks.GithubOptions,
	apiClient plugin.ApiClient,
) (*tasks.GithubApiRepo, errors.Error) {
	repoRes := &tasks.GithubApiRepo{}
	res, err := apiClient.Get(fmt.Sprintf("repos/%s", op.Name), nil, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.HttpStatus(res.StatusCode).New(fmt.Sprintf("unexpected status code when requesting repo detail from %s", res.Request.URL.String()))
	}
	body, err := errors.Convert01(io.ReadAll(res.Body))
	if err != nil {
		return nil, err
	}
	err = errors.Convert(json.Unmarshal(body, repoRes))
	if err != nil {
		return nil, err
	}
	return repoRes, nil
}

func MemorizedGetApiRepo(
	repo *tasks.GithubApiRepo,
	op *tasks.GithubOptions, apiClient plugin.ApiClient,
) (*tasks.GithubApiRepo, errors.Error) {
	if repo == nil {
		var err errors.Error
		repo, err = getApiRepo(op, apiClient)
		if err != nil {
			return nil, err
		}
	}
	return repo, nil
}
