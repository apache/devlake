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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/utils"

	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/qa"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/srvhelper"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
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

	sc, err := makeScopeV200(connection, scopeDetails)
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
	connection *models.AzuredevopsConnection,
	scopeDetails []*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig],
) ([]plugin.Scope, errors.Error) {
	sc := make([]plugin.Scope, 0, 3*len(scopeDetails))
	connectionId := uint64(0)
	endpoint := ""
	if connection != nil {
		connectionId = connection.ID
		endpoint = connection.GetEndpoint()
	}
	claimedProjects := map[string]struct{}{}

	for _, scope := range scopeDetails {
		azuredevopsRepo, scopeConfig := scope.Scope, scope.ScopeConfig
		entities := models.ClaimProjectEntities(
			models.ScopeEntities(azuredevopsRepo.Type, scopeConfig.Entities),
			azuredevopsRepo.ProjectKey(),
			claimedProjects,
		)
		repoID := didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{}).Generate(connectionId, azuredevopsRepo.Id)
		name := azuredevopsRepo.DomainName()

		if azuredevopsRepo.Type == models.RepositoryTypeADO {
			if utils.StringsContains(entities, plugin.DOMAIN_TYPE_CODE_REVIEW) ||
				utils.StringsContains(entities, plugin.DOMAIN_TYPE_CODE) ||
				utils.StringsContains(entities, plugin.DOMAIN_TYPE_CROSS) {
				sc = append(sc, code.NewRepo(repoID, name))
			}
			if utils.StringsContains(entities, plugin.DOMAIN_TYPE_CICD) {
				sc = append(sc, devops.NewCicdScope(repoID, name))
			}
		} else if azuredevopsRepo.Type != models.RepositoryTypeProject {
			if utils.StringsContains(entities, plugin.DOMAIN_TYPE_CICD) {
				sc = append(sc, devops.NewCicdScope(repoID, name))
			}
			if !azuredevopsRepo.IsPrivate && utils.StringsContains(entities, plugin.DOMAIN_TYPE_CODE) {
				sc = append(sc, code.NewRepo(repoID, name))
			}
		}
		sc = appendProjectScopes(sc, connectionId, endpoint, azuredevopsRepo, entities)
	}

	return sc, nil
}

func appendProjectScopes(sc []plugin.Scope, connectionId uint64, endpoint string, repo models.AzuredevopsRepo, entities []string) []plugin.Scope {
	if !models.HasProjectEntity(entities) {
		return sc
	}
	id := didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{}).Generate(connectionId, repo.ProjectDomainScopeID())
	projectName := repo.ProjectName()
	if utils.StringsContains(entities, plugin.DOMAIN_TYPE_TICKET) {
		board := ticket.NewBoard(id, projectName)
		board.Type = "Scrum"
		board.Url = models.ProjectBoardURL(endpoint, repo.OrganizationId, projectName)
		sc = append(sc, board)
	}
	if utils.StringsContains(entities, plugin.DOMAIN_TYPE_CODE_QUALITY) {
		sc = append(sc, &qa.QaProject{
			DomainEntityExtended: domainlayer.DomainEntityExtended{
				Id:        id,
				NoPKModel: common.NewNoPKModel(),
			},
			Name: projectName,
		})
	}
	return sc
}

func makePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connection *models.AzuredevopsConnection,
	scopeDetails []*srvhelper.ScopeDetail[models.AzuredevopsRepo, models.AzuredevopsScopeConfig],
) (coreModels.PipelinePlan, errors.Error) {
	plans := make(coreModels.PipelinePlan, 0, 3*len(scopeDetails))
	claimedProjects := map[string]struct{}{}
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

		entities := models.ClaimProjectEntities(
			models.ScopeEntities(azuredevopsRepo.Type, scopeConfig.Entities),
			azuredevopsRepo.ProjectKey(),
			claimedProjects,
		)
		if azuredevopsRepo.Type != models.RepositoryTypeADO && azuredevopsRepo.Type != models.RepositoryTypeProject && azuredevopsRepo.IsPrivate {
			filtered := make([]string, 0, len(entities))
			for _, entity := range entities {
				if entity != plugin.DOMAIN_TYPE_CODE {
					filtered = append(filtered, entity)
				}
			}
			entities = filtered
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
		if azuredevopsRepo.Type != models.RepositoryTypeProject && (utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) && !scope.Scope.IsPrivate || len(scopeConfig.Entities) == 0) {
			cloneUrl, err := errors.Convert01(url.Parse(azuredevopsRepo.RemoteUrl))
			if err != nil {
				return nil, err
			}

			if scope.Scope.Type == models.RepositoryTypeADO {
				username := connection.Username
				if username == "" {
					username = "git"
				}
				cloneUrl.User = url.UserPassword(username, connection.Token)
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
		if azuredevopsRepo.Type != models.RepositoryTypeProject && scopeConfig.Refdiff != nil {
			task := &coreModels.PipelineTask{
				Plugin:  "refdiff",
				Options: scopeConfig.Refdiff,
			}
			plans = append(plans, coreModels.PipelineStage{task})
		}
	}
	return plans, nil
}
