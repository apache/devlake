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
	pluginhelper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/aisdlc/api"
	"github.com/apache/devlake/plugins/aisdlc/models"
	"github.com/apache/devlake/plugins/aisdlc/models/migrationscripts"
	"github.com/apache/devlake/plugins/aisdlc/tasks"
	"github.com/apache/devlake/plugins/aisdlc/tasks/parse"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginMigration
	plugin.PluginTask
	plugin.DataSourcePluginBlueprintV200
} = (*Aisdlc)(nil)

// Aisdlc collects station artifacts from GitLab.
type Aisdlc struct{}

func (p Aisdlc) Description() string {
	return "Collect AISDLC station artifacts from GitLab and store shared observations"
}

func (p Aisdlc) Name() string {
	return "aisdlc"
}

func (p Aisdlc) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Aisdlc) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/aisdlc"
}

func (p Aisdlc) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.AisdlcConnection{},
		&models.AisdlcScope{},
		&models.AisdlcScopeConfig{},
		&models.AisdlcRawFile{},
		&models.AisdlcCheckpoint{},
		&models.AisdlcExecution{},
		&models.AisdlcWorkItem{},
		&models.AisdlcObservation{},
		&models.AisdlcEvidence{},
		&models.AisdlcItemLink{},
		&models.AisdlcUnknown{},
		&models.AisdlcTime{},
	}
}

func (p Aisdlc) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectFilesMeta,
		tasks.ExtractObservationsMeta,
		tasks.LinkWorkItemsMeta,
	}
}

func (p Aisdlc) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]any) (any, errors.Error) {
	var op tasks.AisdlcOptions
	if err := pluginhelper.Decode(options, &op, nil); err != nil {
		return nil, err
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New("connectionId is required")
	}
	if op.FullName == "" {
		return nil, errors.BadInput.New("fullName is required")
	}
	connectionHelper := pluginhelper.NewConnectionHelper(taskCtx, nil, p.Name())
	connection := &models.AisdlcConnection{}
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, err
	}
	if op.ScopeConfig == nil {
		scope := &models.AisdlcScope{}
		if err := taskCtx.GetDal().First(scope, dal.Where("connection_id = ? AND full_name = ?", op.ConnectionId, op.FullName)); err != nil {
			return nil, errors.Default.Wrap(err, "scope not found")
		}
		if scope.ScopeConfigId == 0 {
			return nil, errors.BadInput.New("scope has no scope config")
		}
		scopeConfig := &models.AisdlcScopeConfig{}
		if err := taskCtx.GetDal().First(scopeConfig, dal.Where("id = ?", scope.ScopeConfigId)); err != nil {
			return nil, errors.Default.Wrap(err, "scope config not found")
		}
		op.ScopeConfig = scopeConfig
	}
	if err := validateScopeConfig(op.ScopeConfig); err != nil {
		return nil, err
	}
	return &tasks.AisdlcTaskData{
		Options:    &op,
		Connection: connection,
	}, nil
}

func validateScopeConfig(cfg *models.AisdlcScopeConfig) errors.Error {
	if cfg == nil {
		return errors.BadInput.New("scope config is required")
	}
	if cfg.TenantId == "" || cfg.SourceId == "" || cfg.StationId == "" {
		return errors.BadInput.New("tenantId, sourceId, and stationId are required")
	}
	if cfg.Parser != parse.ParserRFE && cfg.Parser != parse.ParserStrat {
		return errors.BadInput.New(fmt.Sprintf("parser must be %s or %s", parse.ParserRFE, parse.ParserStrat))
	}
	if cfg.DisplayName == "" {
		cfg.DisplayName = cfg.StationId
	}
	return nil
}

func (p Aisdlc) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, bpScopes)
}

func (p Aisdlc) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Aisdlc) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
	}
}
