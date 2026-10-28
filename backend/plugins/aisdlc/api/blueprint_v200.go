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
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/plugin"
	helperapi "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/srvhelper"
	"github.com/apache/devlake/plugins/aisdlc/models"
	"github.com/apache/devlake/plugins/aisdlc/tasks"
)

func MakeDataSourcePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	connection, err := dsHelper.ConnSrv.FindByPk(connectionId)
	if err != nil {
		return nil, nil, err
	}
	scopeDetails, err := dsHelper.ScopeSrv.MapScopeDetails(connectionId, bpScopes)
	if err != nil {
		return nil, nil, err
	}
	plan, err := makePipelinePlan(subtaskMetas, scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}
	return plan, makeScopes(scopeDetails), nil
}

func makePipelinePlan(
	subtaskMetas []plugin.SubTaskMeta,
	scopeDetails []*srvhelper.ScopeDetail[models.AisdlcScope, models.AisdlcScopeConfig],
	connection *models.AisdlcConnection,
) (coreModels.PipelinePlan, errors.Error) {
	plan := make(coreModels.PipelinePlan, len(scopeDetails))
	for i, scopeDetail := range scopeDetails {
		scope, scopeConfig := scopeDetail.Scope, scopeDetail.ScopeConfig
		entities := []string{plugin.DOMAIN_TYPE_CODE}
		if scopeConfig != nil && len(scopeConfig.Entities) > 0 {
			entities = scopeConfig.Entities
		}
		task, err := helperapi.MakePipelinePlanTask(
			"aisdlc",
			subtaskMetas,
			entities,
			tasks.AisdlcOptions{
				ConnectionId: connection.ID,
				FullName:     scope.FullName,
				ScopeConfig:  scopeConfig,
			},
		)
		if err != nil {
			return nil, err
		}
		plan[i] = coreModels.PipelineStage{task}
	}
	return plan, nil
}

func makeScopes(
	scopeDetails []*srvhelper.ScopeDetail[models.AisdlcScope, models.AisdlcScopeConfig],
) []plugin.Scope {
	scopes := make([]plugin.Scope, 0, len(scopeDetails))
	for _, scopeDetail := range scopeDetails {
		scope := scopeDetail.Scope
		scopes = append(scopes, &code.Repo{
			DomainEntity: domainlayer.DomainEntity{Id: scope.FullName},
			Name:         scope.ScopeName(),
		})
	}
	return scopes
}
