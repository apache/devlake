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

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/grafana_irm/api"
	"github.com/apache/devlake/plugins/grafana_irm/models"
	"github.com/apache/devlake/plugins/grafana_irm/models/migrationscripts"
	"github.com/apache/devlake/plugins/grafana_irm/tasks"
)

// make sure interface is implemented

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.DataSourcePluginBlueprintV200
	plugin.CloseablePluginTask
	plugin.PluginSource
} = (*GrafanaIrm)(nil)

type GrafanaIrm struct{}

func (p GrafanaIrm) Description() string {
	return "collect Grafana IRM incident data"
}

func (p GrafanaIrm) Name() string {
	return "grafana_irm"
}

func (p GrafanaIrm) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p GrafanaIrm) Connection() dal.Tabler {
	return &models.GrafanaIrmConnection{}
}

func (p GrafanaIrm) Scope() plugin.ToolLayerScope {
	return &models.GrafanaIrmScope{}
}

func (p GrafanaIrm) ScopeConfig() dal.Tabler {
	return &models.GrafanaIrmScopeConfig{}
}

func (p GrafanaIrm) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectIncidentsMeta,
		tasks.ExtractIncidentsMeta,
		tasks.ConvertIncidentsMeta,
	}
}

func (p GrafanaIrm) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.GrafanaIrmConnection{},
		&models.GrafanaIrmScope{},
		&models.GrafanaIrmScopeConfig{},
		&models.Incident{},
		&models.IncidentLabel{},
		&models.IncidentAssignment{},
	}
}

func (p GrafanaIrm) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}
	connectionHelper := helper.NewConnectionHelper(
		taskCtx,
		nil,
		p.Name(),
	)
	connection := &models.GrafanaIrmConnection{}
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get Grafana IRM connection by the given connection ID")
	}

	if err := loadScopeConfig(taskCtx, op); err != nil {
		return nil, err
	}

	client, err := helper.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}
	asyncClient, err := helper.CreateAsyncApiClient(taskCtx, client, nil)
	if err != nil {
		return nil, err
	}
	return &tasks.GrafanaIrmTaskData{
		Options:    op,
		Client:     asyncClient,
		Connection: connection,
	}, nil
}

// loadScopeConfig resolves op.ScopeConfig, which decides which incidents this
// scope covers (see models.GrafanaIrmScopeConfig). Options may carry it
// inline (advanced mode), or only a scope config id, or neither — in which
// case it's read from the scope row. A scope with no config at all is left
// with a zero-value config, which means "no label filter": the scope covers
// every real incident on the connection.
func loadScopeConfig(taskCtx plugin.TaskContext, op *tasks.GrafanaIrmOptions) errors.Error {
	if op.ScopeConfig != nil {
		return nil
	}
	db := taskCtx.GetDal()
	if op.ScopeConfigId == 0 && op.ScopeId != "" {
		scope := &models.GrafanaIrmScope{}
		err := db.First(scope, dal.Where("connection_id = ? AND id = ?", op.ConnectionId, op.ScopeId))
		if err != nil && !db.IsErrorNotFound(err) {
			return errors.Default.Wrap(err, "unable to get Grafana IRM scope")
		}
		if err == nil {
			op.ScopeConfigId = scope.ScopeConfigId
		}
	}
	scopeConfig := &models.GrafanaIrmScopeConfig{}
	if op.ScopeConfigId != 0 {
		err := db.First(scopeConfig, dal.Where("id = ?", op.ScopeConfigId))
		if err != nil && !db.IsErrorNotFound(err) {
			return errors.Default.Wrap(err, "unable to get Grafana IRM scope config")
		}
	}
	op.ScopeConfig = scopeConfig
	return nil
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p GrafanaIrm) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/grafana_irm"
}

func (p GrafanaIrm) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p GrafanaIrm) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/search-remote-scopes": {
			"GET": api.SearchRemoteScopes,
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
	}
}

func (p GrafanaIrm) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p GrafanaIrm) Close(taskCtx plugin.TaskContext) errors.Error {
	_, ok := taskCtx.GetData().(*tasks.GrafanaIrmTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	return nil
}
