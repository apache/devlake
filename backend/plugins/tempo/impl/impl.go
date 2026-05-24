/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements. See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License. You may obtain a copy of the License at
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
	"github.com/apache/incubator-devlake/plugins/tempo/api"
	"github.com/apache/incubator-devlake/plugins/tempo/models"
	"github.com/apache/incubator-devlake/plugins/tempo/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/tempo/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMigration
	plugin.DataSourcePluginBlueprintV200
	plugin.CloseablePluginTask
	plugin.PluginSource
} = (*Tempo)(nil)

type Tempo struct {
}

func (p Tempo) Connection() dal.Tabler {
	return &models.TempoConnection{}
}

func (p Tempo) Scope() plugin.ToolLayerScope {
	return &models.TempoTeam{}
}

func (p Tempo) ScopeConfig() dal.Tabler {
	return &models.TempoScopeConfig{}
}

func (p Tempo) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Tempo) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.TempoConnection{},
		&models.TempoTeam{},
		&models.TempoWorklog{},
		&models.TempoScopeConfig{},
	}
}

func (p Tempo) Description() string {
	return "Collect worklogs from Jira Tempo"
}

func (p Tempo) Name() string {
	return "tempo"
}

func (p Tempo) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectTeamsMeta,
		tasks.ExtractTeamsMeta,
		tasks.CollectWorklogsMeta,
		tasks.ExtractWorklogsMeta,
		tasks.ConvertWorklogsMeta,
	}
}

func (p Tempo) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.TempoOptions
	var err errors.Error

	logger := taskCtx.GetLogger()
	logger.Debug("%v", options)

	err = helper.Decode(options, &op, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "could not decode Tempo options")
	}

	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New("tempo connectionId is invalid")
	}

	connection := &models.TempoConnection{}
	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())

	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get Tempo connection")
	}

	tempoApiClient, err := tasks.NewTempoApiClient(taskCtx, connection)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create Tempo api client")
	}

	taskData := &tasks.TempoTaskData{
		Options:    &op,
		ApiClient:  tempoApiClient,
		Connection: connection,
	}

	return taskData, nil
}

func (p Tempo) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Tempo) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/tempo"
}

func (p Tempo) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Tempo) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/proxy/*path": {
			"GET": api.Proxy,
		},
		"connections/:connectionId/teams": {
			"GET": api.GetTeams,
		},
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
		},
		"connections/:connectionId/search-remote-scopes": {
			"GET": api.SearchRemoteScopes,
		},
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.UpdateScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopeList,
			"PUT": api.PutScope,
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
	}
}

func (p Tempo) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.TempoTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	data.ApiClient.Release()
	return nil
}
