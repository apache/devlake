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
	"time"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/api"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/plugins/youtrack/models/migrationscripts"
	"github.com/apache/devlake/plugins/youtrack/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginSource
	plugin.PluginMigration
	plugin.DataSourcePluginBlueprintV200
} = (*Youtrack)(nil)

type Youtrack struct{}

func (p Youtrack) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Youtrack) Description() string {
	return "To collect and enrich data from YouTrack"
}

func (p Youtrack) Name() string {
	return "youtrack"
}

func (p Youtrack) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/youtrack"
}

func (p Youtrack) Connection() dal.Tabler {
	return &models.YoutrackConnection{}
}

func (p Youtrack) Scope() plugin.ToolLayerScope {
	return &models.YoutrackProject{}
}

func (p Youtrack) ScopeConfig() dal.Tabler {
	return &models.YoutrackScopeConfig{}
}

func (p Youtrack) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

// GetTablesInfo lists every tool-layer table of the plugin. The init
// migration consumes the archived snapshots in exactly this order.
func (p Youtrack) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.YoutrackConnection{},
		&models.YoutrackProject{},
		&models.YoutrackScopeConfig{},
		&models.YoutrackAccount{},
		&models.YoutrackIssue{},
		&models.YoutrackIssueComment{},
		&models.YoutrackIssueLabel{},
		&models.YoutrackWorkflowState{},
		&models.YoutrackIssueChangelog{},
	}
}

// SubTaskMetas declares every subtask in execution order: this list's order
// is the contract (Linear idiom) — no Dependencies fields anywhere
// in the plugin.
func (p Youtrack) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectWorkflowStatesMeta,
		tasks.ExtractWorkflowStatesMeta,
		tasks.CollectIssuesMeta,
		tasks.ExtractIssuesMeta,
		tasks.CollectCommentsMeta,
		tasks.ExtractCommentsMeta,
		tasks.ConvertProjectsMeta,
		tasks.ConvertAccountsMeta,
		tasks.ConvertIssuesMeta,
		tasks.ConvertIssueLabelsMeta,
		tasks.ConvertCommentsMeta,
	}
}

func (p Youtrack) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.YoutrackOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, errors.Default.Wrap(err, "could not decode YouTrack options")
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New("youtrack connectionId is invalid")
	}
	if op.ProjectId == "" {
		return nil, errors.BadInput.New("youtrack projectId is required")
	}

	connection := &models.YoutrackConnection{}
	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, errors.Default.Wrap(err, "error getting connection for YouTrack plugin")
	}

	// Resolve the scope config (field-name slots + type/status mappings).
	// Default to an empty config when none is set so subtasks can rely on it
	// being non-nil.
	scopeConfig := &models.YoutrackScopeConfig{}
	if op.ScopeConfigId != 0 {
		if err := taskCtx.GetDal().First(scopeConfig, dal.Where("id = ?", op.ScopeConfigId)); err != nil {
			return nil, errors.Default.Wrap(err, "error getting scope config for YouTrack plugin")
		}
	}
	// An empty selection means "all domain types", exactly as the blueprint
	// planner reads it (srvhelper setDefaultEntities / MakePipelinePlanSubtasks):
	// otherwise the planner schedules the CROSS subtasks while the extractors,
	// seeing no CROSS here, derive no accounts for them to convert.
	if len(scopeConfig.Entities) == 0 {
		scopeConfig.Entities = plugin.DOMAIN_TYPES
	}

	// The scope row carries the shortName needed for the `project: {}` query
	// and for Issue.OriginalProject.
	project := &models.YoutrackProject{}
	if err := taskCtx.GetDal().First(project, dal.Where("connection_id = ? AND id = ?", op.ConnectionId, op.ProjectId)); err != nil {
		return nil, errors.Default.Wrap(err, "error getting project scope for YouTrack plugin")
	}

	apiClient, err := tasks.NewYoutrackApiClient(taskCtx, connection)
	if err != nil {
		return nil, err
	}

	taskData := &tasks.YoutrackTaskData{
		Options:     &op,
		Connection:  connection,
		ScopeConfig: scopeConfig,
		Project:     project,
		ApiClient:   apiClient,
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

func (p Youtrack) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
		},
		"connections/:connectionId/proxy/rest/*path": {
			"GET": api.Proxy,
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
		"connections/:connectionId/scopes": {
			"GET": api.GetScopeList,
			"PUT": api.PutScopes,
		},
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		// the config-ui's attached-projects lookup. Singular `scope-config`,
		// NOT nested under `connections/:connectionId` — the asymmetry is
		// Linear's, copied verbatim or the UI call 404s.
		"scope-config/:scopeConfigId/projects": {
			"GET": api.GetProjectsByScopeConfig,
		},
	}
}

func (p Youtrack) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}
