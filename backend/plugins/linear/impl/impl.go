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

package impl

import (
	"fmt"
	"time"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/apache/incubator-devlake/plugins/linear/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/linear/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginSource
	plugin.PluginMigration
	plugin.CloseablePluginTask
	plugin.DataSourcePluginBlueprintV200
} = (*Linear)(nil)

type Linear struct{}

func (p Linear) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Linear) Description() string {
	return "To collect and enrich data from Linear"
}

func (p Linear) Name() string {
	return "linear"
}

func (p Linear) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/linear"
}

func (p Linear) Connection() dal.Tabler {
	return &models.LinearConnection{}
}

func (p Linear) Scope() plugin.ToolLayerScope {
	return &models.LinearTeam{}
}

func (p Linear) ScopeConfig() dal.Tabler {
	return &models.LinearScopeConfig{}
}

func (p Linear) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Linear) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.LinearConnection{},
		&models.LinearTeam{},
		&models.LinearScopeConfig{},
		&models.LinearAccount{},
		&models.LinearIssue{},
		&models.LinearComment{},
		&models.LinearIssueLabel{},
		&models.LinearWorkflowState{},
		&models.LinearCycle{},
		&models.LinearIssueHistory{},
	}
}

func (p Linear) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectAccountsMeta,
		tasks.ExtractAccountsMeta,
		tasks.ConvertAccountsMeta,
	}
}

func (p Linear) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.LinearOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, errors.Default.Wrap(err, "could not decode Linear options")
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New("linear connectionId is invalid")
	}
	if op.TeamId == "" {
		return nil, errors.BadInput.New("linear teamId is required")
	}

	connection := &models.LinearConnection{}
	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, errors.Default.Wrap(err, "error getting connection for Linear plugin")
	}

	graphqlClient, err := tasks.NewLinearGraphqlClient(taskCtx, connection)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to create Linear GraphQL client")
	}

	taskData := &tasks.LinearTaskData{
		Options:       &op,
		GraphqlClient: graphqlClient,
	}
	if op.TimeAfter != "" {
		timeAfter, errConv := errors.Convert01(time.Parse(time.RFC3339, op.TimeAfter))
		if errConv != nil {
			return nil, errors.BadInput.Wrap(errConv, "invalid timeAfter")
		}
		taskData.TimeAfter = &timeAfter
	}
	return taskData, nil
}

func (p Linear) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"test": {
			"POST": api.TestConnection,
		},
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.ListConnections,
		},
		"connections/:connectionId": {
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
			"GET":    api.GetConnection,
		},
		"connections/:connectionId/test": {
			"POST": api.TestExistingConnection,
		},
		"connections/:connectionId/scope-configs": {
			"POST": api.PostScopeConfig,
			"GET":  api.GetScopeConfigList,
		},
		"connections/:connectionId/scope-configs/:scopeConfigId": {
			"PATCH":  api.PatchScopeConfig,
			"GET":    api.GetScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopeList,
			"PUT": api.PutScopes,
		},
	}
}

func (p Linear) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Linear) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.LinearTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	if data.GraphqlClient != nil {
		data.GraphqlClient.Release()
	}
	return nil
}
