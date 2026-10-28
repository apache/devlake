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
	coremodels "github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira_snowflake/api"
	"github.com/apache/devlake/plugins/jira_snowflake/models"
	"github.com/apache/devlake/plugins/jira_snowflake/models/migrationscripts"
	"github.com/apache/devlake/plugins/jira_snowflake/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMigration
	plugin.PluginApi
	plugin.PluginSource
} = (*JiraSnowflake)(nil)

// jiraPluginStub is a minimal plugin.PluginMeta that lets didgen resolve
// types from plugins/jira/models (JiraIssue, JiraSprint, etc.) without
// requiring the full jira plugin to be loaded alongside jira_snowflake.
type jiraPluginStub struct{}

func (jiraPluginStub) Name() string        { return "jira" }
func (jiraPluginStub) RootPkgPath() string { return "github.com/apache/devlake/plugins/jira" }
func (jiraPluginStub) Description() string { return "" }

// JiraSnowflake is the plugin implementation struct.
type JiraSnowflake struct{}

func (p JiraSnowflake) Connection() dal.Tabler {
	return &models.SnowflakeJiraConnection{}
}

func (p JiraSnowflake) Scope() plugin.ToolLayerScope {
	return &jiramodels.JiraBoard{}
}

func (p JiraSnowflake) ScopeConfig() dal.Tabler {
	return &jiramodels.JiraScopeConfig{}
}

func (p JiraSnowflake) Init(basicRes context.BasicRes) errors.Error {
	// Only register the jira stub when the real jira plugin is absent.
	// If both plugins are deployed in the same instance, the real registration
	// must take precedence — overwriting it with this stub would break jira
	// API routing, task execution, and didgen resolution for all jira connections.
	if _, err := plugin.GetPlugin("jira"); err != nil {
		_ = plugin.RegisterPlugin("jira", jiraPluginStub{})
	}
	api.Init(basicRes, p)
	return nil
}

func (p JiraSnowflake) Name() string {
	return "jira_snowflake"
}

func (p JiraSnowflake) Description() string {
	return "Ingest Jira data from a Snowflake replica (Fivetran) instead of the Jira REST API"
}

func (p JiraSnowflake) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/jira_snowflake"
}

func (p JiraSnowflake) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		// Sync tasks: Snowflake → _tool_jira_* tables
		tasks.SyncIssuesMeta,
		tasks.SyncSprintsMeta,
		tasks.SyncSprintIssuesMeta,
		tasks.SyncChangelogsMeta,
		tasks.SyncWorklogsMeta,
		tasks.SyncLabelsMeta,
		tasks.SyncIssueLinksMeta,
		// Convertor tasks: _tool_jira_* → domain layer
		// Owned copies of jira/tasks convertors, adapted for the no-raw-table context.
		tasks.ConvertBoardMeta,
		tasks.ConvertIssuesMeta,
		tasks.ConvertIssueLabelsMeta,
		tasks.ConvertWorklogsMeta,
		tasks.ConvertChangelogsMeta,
		tasks.ConvertIssueRelationshipsMeta,
		tasks.ConvertSprintsMeta,
		tasks.ConvertSprintIssuesMeta,
	}
}

func (p JiraSnowflake) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}

	// Load the Snowflake connection record
	connection := &models.SnowflakeJiraConnection{}
	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, errors.Default.Wrap(err, "unable to get jira_snowflake connection")
	}

	// Ensure a JiraBoard scope record exists so convertors can find it.
	// Load the board before resolving scope config so we can inherit
	// board.scope_config_id (same precedence as the jira plugin).
	board := &jiramodels.JiraBoard{}
	dbErr := taskCtx.GetDal().First(board, dal.Where("connection_id = ? AND board_id = ?", op.ConnectionId, op.BoardId))
	if dbErr != nil && taskCtx.GetDal().IsErrorNotFound(dbErr) {
		board = &jiramodels.JiraBoard{
			Scope:   coremodels.Scope{ConnectionId: op.ConnectionId},
			BoardId: op.BoardId,
			Name:    "Snowflake Board",
		}
		if createErr := taskCtx.GetDal().CreateIfNotExist(board); createErr != nil {
			return nil, errors.Default.Wrap(createErr, "failed to create JiraBoard scope record")
		}
	} else if dbErr != nil {
		return nil, errors.Default.Wrap(dbErr, "failed to look up JiraBoard scope record")
	}

	// Scope config precedence: inline options.scopeConfig > options.scopeConfigId >
	// board.scope_config_id > empty config.
	if op.ScopeConfigId == 0 && board.ScopeConfigId != 0 {
		op.ScopeConfigId = board.ScopeConfigId
	}
	if op.ScopeConfig == nil && op.ScopeConfigId != 0 {
		var scopeConfig jiramodels.JiraScopeConfig
		if loadErr := taskCtx.GetDal().First(&scopeConfig, dal.Where("id = ?", op.ScopeConfigId)); loadErr != nil {
			return nil, errors.BadInput.Wrap(loadErr, "failed to load scope config")
		}
		op.ScopeConfig = &scopeConfig
	}
	if op.ScopeConfig == nil {
		op.ScopeConfig = new(jiramodels.JiraScopeConfig)
	}

	// Open the Snowflake SQL connection (closed in Close())
	snowDB, openErr := tasks.OpenSnowflakeDB(
		connection.Account,
		connection.User,
		connection.AuthType,
		connection.PrivateKey,
		connection.Database,
		connection.Schema,
		connection.Warehouse,
		connection.Role,
	)
	if openErr != nil {
		return nil, openErr
	}

	return &tasks.JiraSnowflakeTaskData{
		Options:     op,
		SnowflakeDB: snowDB,
	}, nil
}

// Close is called after all subtasks complete; it closes the Snowflake connection.
func (p JiraSnowflake) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.JiraSnowflakeTaskData)
	if ok && data != nil && data.SnowflakeDB != nil {
		if err := data.SnowflakeDB.Close(); err != nil {
			return errors.Default.Wrap(err, "failed to close Snowflake connection")
		}
	}
	return nil
}

func (p JiraSnowflake) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.SnowflakeJiraConnection{},
	}
}

func (p JiraSnowflake) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p JiraSnowflake) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.GetConnections,
		},
		"connections/:connectionId": {
			"GET":    api.GetConnection,
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
		},
	}
}
