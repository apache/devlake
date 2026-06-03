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
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/claude_code/api"
	"github.com/apache/incubator-devlake/plugins/claude_code/models"
	"github.com/apache/incubator-devlake/plugins/claude_code/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/claude_code/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginSource
	plugin.DataSourcePluginBlueprintV200
	plugin.PluginMigration
	plugin.CloseablePluginTask
} = (*ClaudeCode)(nil)

// ClaudeCode is the plugin entrypoint implementing DevLake interfaces.
type ClaudeCode struct{}

func (p ClaudeCode) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p ClaudeCode) Description() string {
	return "Collect Claude Code usage analytics and productivity metrics"
}

func (p ClaudeCode) Name() string {
	return "claude_code"
}

func (p ClaudeCode) Connection() dal.Tabler {
	return &models.ClaudeCodeConnection{}
}

func (p ClaudeCode) Scope() plugin.ToolLayerScope {
	return &models.ClaudeCodeScope{}
}

func (p ClaudeCode) ScopeConfig() dal.Tabler {
	return &models.ClaudeCodeScopeConfig{}
}

func (p ClaudeCode) GetTablesInfo() []dal.Tabler {
	return models.GetTablesInfo()
}

func (p ClaudeCode) SubTaskMetas() []plugin.SubTaskMeta {
	return tasks.GetSubTaskMetas()
}

func (p ClaudeCode) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.ClaudeCodeOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, err
	}

	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())
	connection := &models.ClaudeCodeConnection{}
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, err
	}

	NormalizeConnection(connection)

	taskData := &tasks.ClaudeCodeTaskData{
		Options:    &op,
		Connection: connection,
	}

	return taskData, nil
}

func (p ClaudeCode) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"test": {
			"POST": api.TestConnection,
		},
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.ListConnections,
		},
		"connections/:connectionId": {
			"GET":    api.GetConnection,
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
		},
		"connections/:connectionId/test": {
			"POST": api.TestExistingConnection,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopeList,
			"PUT": api.PutScopes,
		},
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes/:scopeId/latest-sync-state": {
			"GET": api.GetScopeLatestSyncState,
		},
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
		},
		"connections/:connectionId/search-remote-scopes": {
			"GET": api.SearchRemoteScopes,
		},
		"connections/:connectionId/scope-configs": {
			"POST": api.PostScopeConfig,
			"GET":  api.GetScopeConfigList,
		},
		"connections/:connectionId/scope-configs/:scopeConfigId": {
			"GET":    api.GetScopeConfig,
			"PATCH":  api.PatchScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
		"scope-config/:scopeConfigId/projects": {
			"GET": api.GetProjectsByScopeConfig,
		},
	}
}

func (p ClaudeCode) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p ClaudeCode) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/claude_code"
}

func (p ClaudeCode) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p ClaudeCode) Close(taskCtx plugin.TaskContext) errors.Error {
	return nil
}
