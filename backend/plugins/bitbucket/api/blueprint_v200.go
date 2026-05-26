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
	"net/url"

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
	"github.com/apache/incubator-devlake/plugins/bitbucket/models"
	"github.com/apache/incubator-devlake/plugins/bitbucket/tasks"
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
	scopeDetails []*srvhelper.ScopeDetail[models.BitbucketRepo, models.BitbucketScopeConfig],
	connection *models.BitbucketConnection,
) (coreModels.PipelinePlan, errors.Error) {
	plan := make(coreModels.PipelinePlan, 0, len(scopeDetails)+1)
	for i, scopeDetail := range scopeDetails {
		if i == len(plan) {
			plan = append(plan, nil)
		}
		bitbucketRepo, scopeConfig := scopeDetail.Scope, scopeDetail.ScopeConfig
		stage := plan[i]
		if stage == nil {
			stage = coreModels.PipelineStage{}
		}
		task, err := helper.MakePipelinePlanTask(
			"bitbucket",
			subtaskMetas,
			scopeConfig.Entities,
			tasks.BitbucketOptions{
				ConnectionId: bitbucketRepo.ConnectionId,
				FullName:     bitbucketRepo.BitbucketId,
			},
		)
		if err != nil {
			return nil, err
		}

		stage = append(stage, task)

		// refdiff
		if scopeConfig != nil && scopeConfig.Refdiff != nil {
			// add a new task to next stage
			j := i + 1
			if j == len(plan) {
				plan = append(plan, nil)
			}
			refdiffOp := scopeConfig.Refdiff
			refdiffOp["repoId"] = didgen.NewDomainIdGenerator(&models.BitbucketRepo{}).Generate(connection.ID, bitbucketRepo.BitbucketId)
			plan[j] = coreModels.PipelineStage{
				{
					Plugin:  "refdiff",
					Options: refdiffOp,
				},
			}
			scopeConfig.Refdiff = nil
		}
		// add gitex stage
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) {
			cloneUrl, err := errors.Convert01(url.Parse(bitbucketRepo.CloneUrl))
			if err != nil {
				return nil, err
			}
			// For Bitbucket API tokens, use x-token-auth as username per Bitbucket docs
			// https://support.atlassian.com/bitbucket-cloud/docs/using-api-tokens/
			gitUsername := connection.Username
			if connection.UsesApiToken {
				gitUsername = "x-bitbucket-api-token-auth"
			}
			cloneUrl.User = url.UserPassword(gitUsername, connection.Password)
			stage = append(stage, &coreModels.PipelineTask{
				Plugin: "gitextractor",
				Options: map[string]interface{}{
					"url":      cloneUrl.String(),
					"name":     bitbucketRepo.BitbucketId,
					"fullName": bitbucketRepo.BitbucketId,
					"repoId":   didgen.NewDomainIdGenerator(&models.BitbucketRepo{}).Generate(connection.ID, bitbucketRepo.BitbucketId),
					"proxy":    connection.Proxy,
				},
			})

		}
		plan[i] = stage
	}
	return plan, nil
}

func makeScopesV200(
	scopeDetails []*srvhelper.ScopeDetail[models.BitbucketRepo, models.BitbucketScopeConfig],
	connection *models.BitbucketConnection,
) ([]plugin.Scope, errors.Error) {
	scopes := make([]plugin.Scope, 0)
	idgen := didgen.NewDomainIdGenerator(&models.BitbucketRepo{})
	for _, scopeDetail := range scopeDetails {
		scope, scopeConfig := scopeDetail.Scope, scopeDetail.ScopeConfig
		id := idgen.Generate(connection.ID, scope.BitbucketId)

		// if no entities specified, use all entities enabled by default
		if len(scopeConfig.Entities) == 0 {
			scopeConfig.Entities = plugin.DOMAIN_TYPES
		}

		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE_REVIEW) ||
			utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CODE) ||
			utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CROSS) {
			// if we don't need to collect gitex, we need to add repo to scopes here
			scopes = append(scopes, &code.Repo{
				DomainEntity: domainlayer.DomainEntity{
					Id: id,
				},
				Name: scope.BitbucketId,
			})
		}
		// add cicd_scope to scopes
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_CICD) {
			scopes = append(scopes, &devops.CicdScope{
				DomainEntity: domainlayer.DomainEntity{
					Id: id,
				},
				Name: scope.BitbucketId,
			})
		}
		// add board to scopes
		if utils.StringsContains(scopeConfig.Entities, plugin.DOMAIN_TYPE_TICKET) {
			scopes = append(scopes, &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{
					Id: id,
				},
				Name: scope.BitbucketId,
			})
		}
	}
	return scopes, nil
}
