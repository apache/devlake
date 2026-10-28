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
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	pluginhelper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/agentready/api"
	"github.com/apache/devlake/plugins/agentready/models"
	"github.com/apache/devlake/plugins/agentready/models/migrationscripts"
	"github.com/apache/devlake/plugins/agentready/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginMigration
	plugin.PluginTask
	plugin.DataSourcePluginBlueprintV200
} = (*AgentReady)(nil)

type AgentReady struct{}

func (p AgentReady) Description() string {
	return "Collect and analyze AI readiness assessments from submissions repositories"
}

func (p AgentReady) Name() string {
	return "agentready"
}

func (p AgentReady) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p AgentReady) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/agentready"
}

func (p AgentReady) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.AgentReadyConnection{},
		&models.AgentReadyScope{},
		&models.AgentReadyScopeConfig{},
		&models.AgentReadyAssessment{},
		&models.AgentReadyFinding{},
		&models.AgentReadyMetric{},
	}
}

func (p AgentReady) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectSubmissionsMeta,
		tasks.ExtractAssessmentsMeta,
		tasks.CalculateMetricsMeta,
	}
}

func (p AgentReady) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]any) (any, errors.Error) {
	var op tasks.AgentReadyOptions
	if err := pluginhelper.Decode(options, &op, nil); err != nil {
		return nil, err
	}

	connectionHelper := pluginhelper.NewConnectionHelper(taskCtx, nil, p.Name())
	connection := &models.AgentReadyConnection{}
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, err
	}

	return &tasks.AgentReadyTaskData{
		Options:    &op,
		Connection: connection,
	}, nil
}

func (p AgentReady) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, bpScopes)
}

func (p AgentReady) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p AgentReady) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
		},
		"connections/:connectionId/scopes/*scopeId": {
			"GET":    api.GetScopeDispatcher,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopeList,
			"PUT": api.PutScopes,
		},
		"connections/:connectionId/scope-configs": {
			"POST": api.CreateScopeConfig,
			"GET":  api.GetScopeConfigList,
		},
		"connections/:connectionId/scope-configs/:scopeConfigId": {
			"PATCH":  api.UpdateScopeConfig,
			"GET":    api.GetScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
		"scope-config/:scopeConfigId/projects": {
			"GET": api.GetProjectsByScopeConfig,
		},
		"connections/:connectionId/assessments": {
			"GET": api.GetAssessments,
		},
		"connections/:connectionId/assessments/:id": {
			"GET": api.GetAssessment,
		},
		"connections/:connectionId/assessments/:id/findings": {
			"GET": api.GetAssessmentFindings,
		},
		"connections/:connectionId/stats": {
			"GET": api.GetStats,
		},
	}
}
