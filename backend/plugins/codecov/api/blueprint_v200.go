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
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/srvhelper"
	"github.com/apache/devlake/plugins/codecov/models"
	"github.com/apache/devlake/plugins/codecov/tasks"
)

// MakeDataSourcePipelinePlanV200 creates a pipeline plan for Codecov
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
		// Provide a more helpful error message if scope is not found
		if lakeErr := errors.AsLakeErrorType(err); lakeErr != nil && lakeErr.As(errors.NotFound) != nil {
			return nil, nil, errors.BadInput.Wrap(err, "One or more scopes were not found in the database. Please save the scopes to the connection first before adding them to a project.")
		}
		return nil, nil, err
	}

	plan, err := makePipelinePlanV200(subtaskMetas, scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}

	// Return scopes for project mapping
	scopes, err := makeScopesV200(scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}

	return plan, scopes, nil
}

func makePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	scopeDetails []*srvhelper.ScopeDetail[models.CodecovRepo, models.CodecovScopeConfig],
	connection *models.CodecovConnection,
) (coreModels.PipelinePlan, errors.Error) {
	plan := make(coreModels.PipelinePlan, len(scopeDetails))
	for i, scopeDetail := range scopeDetails {
		stage := plan[i]
		if stage == nil {
			stage = coreModels.PipelineStage{}
		}

		scope, scopeConfig := scopeDetail.Scope, scopeDetail.ScopeConfig
		// construct task options for codecov
		task, err := helper.MakePipelinePlanTask(
			"codecov",
			subtaskMetas,
			scopeConfig.Entities,
			tasks.CodecovOptions{
				ConnectionId: connection.ID,
				FullName:     scope.FullName,
				ScopeConfig:  scopeConfig,
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
	scopeDetails []*srvhelper.ScopeDetail[models.CodecovRepo, models.CodecovScopeConfig],
	connection *models.CodecovConnection,
) ([]plugin.Scope, errors.Error) {
	scopes := make([]plugin.Scope, 0, len(scopeDetails))

	for _, scopeDetail := range scopeDetails {
		scope := scopeDetail.Scope
		// Return the CodecovRepo itself as it implements plugin.Scope
		scopes = append(scopes, scope)
	}

	return scopes, nil
}
