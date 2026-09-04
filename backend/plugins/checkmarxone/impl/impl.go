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
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/checkmarxone/api"
	"github.com/apache/incubator-devlake/plugins/checkmarxone/models"
	"github.com/apache/incubator-devlake/plugins/checkmarxone/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/checkmarxone/tasks"
)

// compile-time interface assertion
var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginMigration
	plugin.PluginSource
	plugin.DataSourcePluginBlueprintV200
	plugin.CloseablePluginTask
} = (*CheckmarxOne)(nil)

type CheckmarxOne struct{}

func (p CheckmarxOne) Name() string        { return "checkmarxone" }
func (p CheckmarxOne) Description() string { return "collect security findings from CheckmarxOne" }
func (p CheckmarxOne) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/checkmarxone"
}

func (p CheckmarxOne) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p CheckmarxOne) Connection() dal.Tabler      { return &models.CheckmarxoneConnection{} }
func (p CheckmarxOne) Scope() plugin.ToolLayerScope { return &models.CheckmarxoneProject{} }
func (p CheckmarxOne) ScopeConfig() dal.Tabler     { return nil }

func (p CheckmarxOne) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.CheckmarxoneConnection{},
		&models.CheckmarxoneProject{},
		&models.CheckmarxoneFinding{},
	}
}

func (p CheckmarxOne) SubTaskMetas() []plugin.SubTaskMeta {
	return tasks.CollectDataTaskMetas()
}

func (p CheckmarxOne) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.CheckmarxoneOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid options")
	}

	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())
	connection := &models.CheckmarxoneConnection{}
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, errors.Default.Wrap(err, "connection not found")
	}

	logger := taskCtx.GetLogger()
	apiClient, err := tasks.NewCheckmarxoneApiClient(logger, connection)
	if err != nil {
		return nil, err
	}

	return &tasks.CheckmarxoneTaskData{
		Options:    &op,
		ApiClient:  apiClient,
		Connection: connection,
	}, nil
}

func (p CheckmarxOne) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p CheckmarxOne) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.ListConnections,
		},
		"connections/:connectionId": {
			"GET":    api.GetConnection,
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
		},
	}
}

func (p CheckmarxOne) Close(taskCtx plugin.TaskContext) errors.Error {
	data, _ := taskCtx.GetData().(*tasks.CheckmarxoneTaskData)
	if data != nil && data.ApiClient != nil {
		data.ApiClient.Close()
	}
	return nil
}

func (p CheckmarxOne) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []plugin.Scope,
	syncPolicy plugin.BlueprintSyncPolicy,
) (plugin.PipelinePlan, errors.Error) {
	plan := plugin.PipelinePlan{}
	for _, scope := range scopes {
		scopeItem, ok := scope.(*models.CheckmarxoneProject)
		if !ok {
			return nil, errors.BadInput.New("invalid scope item")
		}
		stage := plugin.PipelineStage{}
		for _, task := range p.SubTaskMetas() {
			stage = append(stage, &plugin.PipelineTask{
				Plugin:   p.Name(),
				Subtasks: []string{task.Name},
				Options: map[string]interface{}{
					"connectionId": connectionId,
					"projectId":    scopeItem.ProjectId,
				},
			})
		}
		plan = append(plan, stage)
	}
	return plan, nil
}