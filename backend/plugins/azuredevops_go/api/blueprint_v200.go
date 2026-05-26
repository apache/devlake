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
	"net/url"

	"golang.org/x/exp/slices"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/utils"

	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/azuredevops_go/models"
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
	scopeDetails []*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig],
) ([]plugin.Scope, errors.Error) {
	sc := make([]plugin.Scope, 0, 3*len(scopeDetails))

	for _, scope := range scopeDetails {
		azuredevopsRepo, scopeConfig := scope.Scope, scope.ScopeConfig
		if azuredevopsRepo.Type != models.RepositoryTypeADO {
			continue
		}
		id := didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{}).Generate(connectionId, azuredevopsRepo.Id)

		// if no entities specified, use all entities enabled by default
		if len(scopeConfig.Entities) == 0 {
			scopeConfig.Entities = plugin.DOMAIN_TYPES
		}

		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE_REVIEW) ||
			utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) ||
			utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CROSS) {
			// if we don't need to collect gitex, we need to add repo to scopes here
			scopeRepo := code.NewRepo(id, azuredevopsRepo.Name)
			sc = append(sc, scopeRepo)
		}

		// add cicd_scope to scopes
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CICD) {
			scopeCICD := devops.NewCicdScope(id, azuredevopsRepo.Name)
			sc = append(sc, scopeCICD)
		}

		// add board to scopes
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_TICKET) {
			scopeTicket := ticket.NewBoard(id, azuredevopsRepo.Name)
			sc = append(sc, scopeTicket)
		}
	}

	for _, scope := range scopeDetails {
		azuredevopsRepo, scopeConfig := scope.Scope, scope.ScopeConfig
		if azuredevopsRepo.Type == models.RepositoryTypeADO {
			continue
		}
		id := didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{}).Generate(connectionId, azuredevopsRepo.Id)

		// Azure DevOps Pipeline can be used with remote repositories such as GitHub and Bitbucket
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CICD) {
			scopeCICD := devops.NewCicdScope(id, azuredevopsRepo.Name)
			sc = append(sc, scopeCICD)
		}

		// DOMAIN_TYPE_CODE (i.e. gitextractor, rediff) only works if the repository is public
		if !azuredevopsRepo.IsPrivate && utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) {
			scopeRepo := code.NewRepo(id, azuredevopsRepo.Name)
			sc = append(sc, scopeRepo)
		}
	}

	return sc, nil
}

func makePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connection *models.AzuredevopsConnection,
	scopeDetails []*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig],
) (coreModels.PipelinePlan, errors.Error) {
	plans := make(coreModels.PipelinePlan, 0, 3*len(scopeDetails))
	for _, scope := range scopeDetails {
		azuredevopsRepo, scopeConfig := scope.Scope, scope.ScopeConfig
		var stage coreModels.PipelineStage
		var err errors.Error

		options := make(map[string]interface{})
		options["name"] = azuredevopsRepo.Name // this is solely for the FE to display the repo name of a task

		options["connectionId"] = connection.ID
		options["organizationId"] = azuredevopsRepo.OrganizationId
		options["projectId"] = azuredevopsRepo.ProjectId
		options["externalId"] = azuredevopsRepo.ExternalId
		options["repositoryId"] = azuredevopsRepo.Id
		options["repositoryType"] = azuredevopsRepo.Type

		// construct subtasks
		var entities []string
		if scope.Scope.Type == models.RepositoryTypeADO {
			entities = append(entities, scopeConfig.Entities...)
		} else {
			if i := slices.Index(scopeConfig.Entities, plugin.DOMAIN_TYPE_CICD); i >= 0 {
				entities = append(entities, scopeConfig.Entities[i])
			}

			if i := slices.Index(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE); i >= 0 && !scope.Scope.IsPrivate {
				entities = append(entities, scopeConfig.Entities[i])
			}
		}

		subtasks, err := helper.MakePipelinePlanSubtasks(subtaskMetas, entities)
		if err != nil {
			return nil, err
		}

		stage = append(stage, &coreModels.PipelineTask{
			Plugin:   "azuredevops_go",
			Subtasks: subtasks,
			Options:  options,
		})

		// collect git data by gitextractor if CODE was requested
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) && !scope.Scope.IsPrivate || len(scopeConfig.Entities) == 0 {
			cloneUrl, err := errors.Convert01(url.Parse(azuredevopsRepo.RemoteUrl))
			if err != nil {
				return nil, err
			}

			if scope.Scope.Type == models.RepositoryTypeADO {
				cloneUrl.User = url.UserPassword("git", connection.Token)
			}
			stage = append(stage, &coreModels.PipelineTask{
				Plugin: "gitextractor",
				Options: map[string]interface{}{
					"url":            cloneUrl.String(),
					"name":           azuredevopsRepo.Name,
					"repoId":         didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{}).Generate(connection.ID, azuredevopsRepo.Id),
					"proxy":          connection.Proxy,
					"noShallowClone": true,
				},
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
