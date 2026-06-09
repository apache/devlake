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

	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/tempo/models"
)

func MakeDataSourcePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	// Load connection, scope and scopeConfig from the db
	connection, err := dsHelper.ConnSrv.FindByPk(connectionId)
	if err != nil {
		return nil, nil, err
	}
	scopeDetails, err := dsHelper.ScopeSrv.MapScopeDetails(connectionId, bpScopes)
	if err != nil {
		return nil, nil, err
	}

	// Needed for the connection to populate its access tokens
	_, err = api.NewApiClientFromConnection(context.TODO(), basicRes, connection)
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
	scopeDetails []*srvhelper.ScopeDetail[models.TempoTeam, models.TempoScopeConfig],
	connection *models.TempoConnection,
) (coreModels.PipelinePlan, errors.Error) {
	plan := make(coreModels.PipelinePlan, len(scopeDetails))
	for i, scopeDetail := range scopeDetails {
		stage := plan[i]
		if stage == nil {
			stage = coreModels.PipelineStage{}
		}

		scope := scopeDetail.Scope
		// Construct task options for Tempo
		task, err := api.MakePipelinePlanTask(
			"tempo",
			subtaskMetas,
			nil, // No entities to select for Tempo
			map[string]interface{}{
				"connectionId": scope.ConnectionId,
				"teamId":       scope.TeamId,
			},
		)
		if err != nil {
			return nil, err
		}

		stage = append(stage, task)
		plan[i] = stage
	}

	return plan, nil
}

func makeScopesV200(
	scopeDetails []*srvhelper.ScopeDetail[models.TempoTeam, models.TempoScopeConfig],
	connection *models.TempoConnection,
) ([]plugin.Scope, errors.Error) {
	scopes := make([]plugin.Scope, 0)
	for _, scopeDetail := range scopeDetails {
		tempoTeam := scopeDetail.Scope
		// Add team to scopes
		domainTeam := &ticket.Board{
			DomainEntity: domainlayer.DomainEntity{
				Id: didgen.NewDomainIdGenerator(&models.TempoTeam{}).Generate(tempoTeam.ConnectionId, tempoTeam.TeamId),
			},
			Name: tempoTeam.Name,
			Url:  "", // Tempo doesn't provide a direct URL for teams
			Type: "team",
		}
		scopes = append(scopes, domainTeam)
	}
	return scopes, nil
}
