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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/apache/incubator-devlake/plugins/gitlab/tasks"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/utils"

	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	plugin "github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/gitlab/models"
)

func MakePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	// load connection, scope and scopeConfig from the db
	connection, err := dsHelper.ConnSrv.FindByPk(connectionId)
	if err != nil {
		return nil, nil, err
	}
	scopeDetails, err := dsHelper.ScopeApi.MapScopeDetails(connectionId, bpScopes)
	if err != nil {
		return nil, nil, err
	}

	sc, err := makeScopeV200(connectionId, scopeDetails)
	if err != nil {
		return nil, nil, err
	}

	pp, err := makePipelinePlanV200(subtaskMetas, connection, scopeDetails)
	if err != nil {
		return nil, nil, err
	}

	return pp, sc, nil
}

func makeScopeV200(
	connectionId uint64,
	scopeDetails []*srvhelper.ScopeDetail[models.GitlabProject, models.GitlabScopeConfig],
) ([]plugin.Scope, errors.Error) {
	sc := make([]plugin.Scope, 0, 3*len(scopeDetails))

	for _, scope := range scopeDetails {
		gitlabProject, scopeConfig := scope.Scope, scope.ScopeConfig
		id := didgen.NewDomainIdGenerator(&models.GitlabProject{}).Generate(connectionId, gitlabProject.GitlabId)

		// if no entities specified, use all entities enabled by default
		if len(scopeConfig.Entities) == 0 {
			scopeConfig.Entities = plugin.DOMAIN_TYPES
		}

		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE_REVIEW) ||
			utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) ||
			utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CROSS) {
			// if we don't need to collect gitex, we need to add repo to scopes here
			scopeRepo := code.NewRepo(id, gitlabProject.PathWithNamespace)

			if gitlabProject.ForkedFromProjectWebUrl != "" {
				scopeRepo.ForkedFrom = gitlabProject.ForkedFromProjectWebUrl
			}
			sc = append(sc, scopeRepo)
		}

		// add cicd_scope to scopes
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CICD) {
			scopeCICD := devops.NewCicdScope(id, gitlabProject.PathWithNamespace)
			sc = append(sc, scopeCICD)
		}

		// add board to scopes
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_TICKET) {
			scopeTicket := ticket.NewBoard(id, gitlabProject.PathWithNamespace)
			sc = append(sc, scopeTicket)
		}
	}

	return sc, nil
}

func makePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connection *models.GitlabConnection,
	scopeDetails []*srvhelper.ScopeDetail[models.GitlabProject, models.GitlabScopeConfig],
) (coreModels.PipelinePlan, errors.Error) {
	plans := make(coreModels.PipelinePlan, 0, 3*len(scopeDetails))
	for _, scope := range scopeDetails {
		gitlabProject, scopeConfig := scope.Scope, scope.ScopeConfig
		var stage coreModels.PipelineStage
		// construct subtasks
		task, err := helper.MakePipelinePlanTask(pluginName, subtaskMetas, scopeConfig.Entities, map[string]interface{}{
			"connectionId": connection.ID,
			"projectId":    gitlabProject.GitlabId,
			"fullName":     gitlabProject.PathWithNamespace,
		})
		if err != nil {
			return nil, err
		}
		stage = append(stage, task)

		// collect git data by gitextractor if CODE was requested
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) || len(scopeConfig.Entities) == 0 {
			cloneUrl, err := errors.Convert01(url.Parse(gitlabProject.HttpUrlToRepo))
			if err != nil {
				return nil, err
			}
			cloneUrl.User = url.UserPassword("git", connection.Token)
			gitextOpts := map[string]interface{}{
				"url":          cloneUrl.String(),
				"name":         gitlabProject.Name,
				"fullName":     gitlabProject.PathWithNamespace,
				"repoId":       didgen.NewDomainIdGenerator(&models.GitlabProject{}).Generate(connection.ID, gitlabProject.GitlabId),
				"proxy":        connection.Proxy,
				"connectionId": gitlabProject.ConnectionId,
				"pluginName":   "gitlab",
			}
			if len(scopeConfig.PrSizeExcludedFileExtensions) > 0 {
				// pass excluded file extensions to gitextractor to support PR Size exclusion
				gitextOpts["excludeFileExtensions"] = scopeConfig.PrSizeExcludedFileExtensions
			}
			stage = append(stage, &coreModels.PipelineTask{
				Plugin:  "gitextractor",
				Options: gitextOpts,
			})
		}

		plans = append(plans, stage)

		// refdiff part
		if scopeConfig.Refdiff != nil {
			task := &coreModels.PipelineTask{
				Plugin:  "refdiff",
				Options: scopeConfig.Refdiff,
			}
			plans = append(plans, coreModels.PipelineStage{task})
		}
	}
	return plans, nil
}

func GetApiProject(
	op *tasks.GitlabOptions,
	apiClient plugin.ApiClient,
) (*models.GitlabApiProject, errors.Error) {
	repoRes := &models.GitlabApiProject{}
	res, err := apiClient.Get(fmt.Sprintf("projects/%d", op.ProjectId), nil, nil)
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
