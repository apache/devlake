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
	"github.com/apache/incubator-devlake/plugins/taiga/api"
	"github.com/apache/incubator-devlake/plugins/taiga/models"
	"github.com/apache/incubator-devlake/plugins/taiga/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/taiga/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginMigration
	plugin.DataSourcePluginBlueprintV200
	plugin.CloseablePluginTask
	plugin.PluginSource
} = (*Taiga)(nil)

type Taiga struct {
}

func (p Taiga) Connection() dal.Tabler {
	return &models.TaigaConnection{}
}

func (p Taiga) Scope() plugin.ToolLayerScope {
	return &models.TaigaProject{}
}

func (p Taiga) ScopeConfig() dal.Tabler {
	return &models.TaigaScopeConfig{}
}

func (p Taiga) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Taiga) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.TaigaConnection{},
		&models.TaigaProject{},
		&models.TaigaUserStory{},
		&models.TaigaTask{},
		&models.TaigaIssue{},
		&models.TaigaEpic{},
		&models.TaigaScopeConfig{},
	}
}

func (p Taiga) Description() string {
	return "To collect and enrich data from Taiga"
}

func (p Taiga) Name() string {
	return "taiga"
}

func (p Taiga) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectProjectsMeta,
		tasks.ExtractProjectsMeta,
		tasks.CollectUserStoriesMeta,
		tasks.ExtractUserStoriesMeta,
		tasks.CollectTasksMeta,
		tasks.ExtractTasksMeta,
		tasks.CollectIssuesMeta,
		tasks.ExtractIssuesMeta,
		tasks.CollectEpicsMeta,
		tasks.ExtractEpicsMeta,
		tasks.ConvertProjectsMeta,
		tasks.ConvertUserStoriesMeta,
		tasks.ConvertTasksMeta,
		tasks.ConvertIssuesMeta,
		tasks.ConvertEpicsMeta,
	}
}

func (p Taiga) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.TaigaOptions
	var err errors.Error
	logger := taskCtx.GetLogger()
	logger.Debug("%v", options)

	err = helper.Decode(options, &op, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "could not decode Taiga options")
	}

	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New("taiga connectionId is invalid")
	}

	connection := &models.TaigaConnection{}
	connectionHelper := helper.NewConnectionHelper(
		taskCtx,
		nil,
		p.Name(),
	)
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get Taiga connection")
	}

	taigaApiClient, err := tasks.NewTaigaApiClient(taskCtx, connection)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create taiga api client")
	}

	if op.ProjectId != 0 {
		var scope *models.TaigaProject
		db := taskCtx.GetDal()
		err = db.First(&scope, dal.Where("connection_id = ? AND project_id = ?", op.ConnectionId, op.ProjectId))
		if err != nil && db.IsErrorNotFound(err) {
			// Fetch from remote and save
			// TODO: Implement fetching from Taiga API
			return nil, errors.Default.Wrap(err, fmt.Sprintf("fail to find project: %d", op.ProjectId))
		}
		if err != nil {
			return nil, errors.Default.Wrap(err, fmt.Sprintf("fail to find project: %d", op.ProjectId))
		}
		if op.ScopeConfigId == 0 && scope.ScopeConfigId != 0 {
			op.ScopeConfigId = scope.ScopeConfigId
		}
	}

	if op.ScopeConfig == nil && op.ScopeConfigId != 0 {
		var scopeConfig models.TaigaScopeConfig
		db := taskCtx.GetDal()
		err = taskCtx.GetDal().First(&scopeConfig, dal.Where("id = ?", op.ScopeConfigId))
		if err != nil && db.IsErrorNotFound(err) {
			return nil, errors.BadInput.Wrap(err, "fail to get scopeConfig")
		}
		op.ScopeConfig = &scopeConfig
		if err != nil {
			return nil, errors.BadInput.Wrap(err, "fail to make scopeConfig")
		}
	}

	if op.ScopeConfig == nil && op.ScopeConfigId == 0 {
		op.ScopeConfig = new(models.TaigaScopeConfig)
	}

	// Set default page size
	if op.PageSize <= 0 || op.PageSize > 100 {
		op.PageSize = 100
	}

	taskData := &tasks.TaigaTaskData{
		Options:   &op,
		ApiClient: taigaApiClient,
	}

	return taskData, nil
}

func (p Taiga) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Taiga) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/taiga"
}

func (p Taiga) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Taiga) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"scope-config/:scopeConfigId/projects": {
			"GET": api.GetProjectsByScopeConfig,
		},
	}
}

func (p Taiga) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.TaigaTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	data.ApiClient.Release()
	return nil
}
