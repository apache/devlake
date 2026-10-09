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
	gocontext "context"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/api"
	"github.com/apache/devlake/plugins/cursor/models"
	"github.com/apache/devlake/plugins/cursor/models/migrationscripts"
	"github.com/apache/devlake/plugins/cursor/service"
	"github.com/apache/devlake/plugins/cursor/tasks"
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
} = (*Cursor)(nil)

// Cursor is the plugin entrypoint implementing DevLake interfaces.
type Cursor struct{}

func (p Cursor) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Cursor) Description() string {
	return "Collect Cursor team usage events and billing metrics from the Admin API"
}

func (p Cursor) Name() string {
	return "cursor"
}

func (p Cursor) Connection() dal.Tabler {
	return &models.CursorConnection{}
}

func (p Cursor) Scope() plugin.ToolLayerScope {
	return &models.CursorScope{}
}

func (p Cursor) ScopeConfig() dal.Tabler {
	return &models.CursorScopeConfig{}
}

func (p Cursor) GetTablesInfo() []dal.Tabler {
	return models.GetTablesInfo()
}

func (p Cursor) SubTaskMetas() []plugin.SubTaskMeta {
	return tasks.GetSubTaskMetas()
}

func (p Cursor) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.CursorOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, err
	}

	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())
	connection := &models.CursorConnection{}
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, err
	}

	NormalizeConnection(connection)

	if err := service.RefreshAndPersistOptionalPermissions(gocontext.Background(), taskCtx, connection); err != nil {
		taskCtx.GetLogger().Warn(err, "cursor: optional permission refresh failed; using stored connection capabilities")
	}

	return &tasks.CursorTaskData{
		Options:    &op,
		Connection: connection,
	}, nil
}

func (p Cursor) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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

func (p Cursor) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Cursor) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/cursor"
}

func (p Cursor) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Cursor) Close(taskCtx plugin.TaskContext) errors.Error {
	return nil
}
