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

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/asana/api"
	"github.com/apache/incubator-devlake/plugins/asana/models"
	"github.com/apache/incubator-devlake/plugins/asana/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/asana/tasks"
)

var _ interface {
	plugin.PluginTask
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginMigration
	plugin.PluginSource
	plugin.CloseablePluginTask
	plugin.DataSourcePluginBlueprintV200
} = (*Asana)(nil)

type Asana struct{}

func (p Asana) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Asana) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.AsanaConnection{},
		&models.AsanaProject{},
		&models.AsanaScopeConfig{},
		&models.AsanaTask{},
		&models.AsanaSection{},
		&models.AsanaUser{},
		&models.AsanaWorkspace{},
		&models.AsanaTeam{},
		&models.AsanaStory{},
		&models.AsanaTag{},
		&models.AsanaTaskTag{},
		&models.AsanaCustomField{},
		&models.AsanaTaskCustomFieldValue{},
		&models.AsanaProjectMembership{},
		&models.AsanaTeamMembership{},
	}
}

func (p Asana) Description() string {
	return "To collect and enrich data from Asana"
}

func (p Asana) Name() string {
	return "asana"
}

func (p Asana) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		// Collect and extract in hierarchical order
		// 1. Project (scope)
		tasks.CollectProjectMeta,
		tasks.ExtractProjectMeta,
		// 2. Users (project members)
		tasks.CollectUserMeta,
		tasks.ExtractUserMeta,
		// 3. Sections
		tasks.CollectSectionMeta,
		tasks.ExtractSectionMeta,
		// 4. Tasks
		tasks.CollectTaskMeta,
		tasks.ExtractTaskMeta,
		// 5. Subtasks (children of tasks)
		tasks.CollectSubtaskMeta,
		tasks.ExtractSubtaskMeta,
		// 6. Stories (comments on tasks)
		tasks.CollectStoryMeta,
		tasks.ExtractStoryMeta,
		// 7. Tags (on tasks)
		tasks.CollectTagMeta,
		tasks.ExtractTagMeta,
		// Convert to domain layer
		tasks.ConvertProjectMeta,
		tasks.ConvertUserMeta,
		tasks.ConvertTaskMeta,
		tasks.ConvertStoryMeta,
	}
}

func (p Asana) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.AsanaOptions
	err := helper.Decode(options, &op, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "Asana plugin could not decode options")
	}
	if op.ProjectId == "" {
		return nil, errors.BadInput.New("asana projectId is required")
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New("asana connectionId is invalid")
	}
	connection := &models.AsanaConnection{}
	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error getting connection for Asana plugin")
	}
	apiClient, err := tasks.CreateApiClient(taskCtx, connection)
	if err != nil {
		return nil, err
	}
	return &tasks.AsanaTaskData{
		Options:   &op,
		ApiClient: apiClient,
	}, nil
}

func (p Asana) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/asana"
}

func (p Asana) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Asana) Connection() dal.Tabler {
	return &models.AsanaConnection{}
}

func (p Asana) Scope() plugin.ToolLayerScope {
	return &models.AsanaProject{}
}

func (p Asana) ScopeConfig() dal.Tabler {
	return &models.AsanaScopeConfig{}
}

func (p Asana) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/proxy/rest/*path": {
			"GET": api.Proxy,
		},
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
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
		"scope-config/:scopeConfigId/projects": {
			"GET": api.GetProjectsByScopeConfig,
		},
	}
}

func (p Asana) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Asana) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.AsanaTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	data.ApiClient.Release()
	return nil
}
