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
	"net/http"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/api"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/models/migrationscripts"
	"github.com/apache/devlake/plugins/jira/tasks"
	"github.com/apache/devlake/plugins/jira/tasks/apiv2models"
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
} = (*Jira)(nil)

type Jira struct {
}

func (p Jira) Connection() dal.Tabler {
	return &models.JiraConnection{}
}

func (p Jira) Scope() plugin.ToolLayerScope {
	return &models.JiraBoard{}
}

func (p Jira) ScopeConfig() dal.Tabler {
	return &models.JiraScopeConfig{}
}

func (p Jira) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)

	return nil
}

func (p Jira) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.ApiMyselfResponse{},
		&models.JiraAccount{},
		&models.JiraBoard{},
		&models.JiraBoardIssue{},
		&models.JiraBoardSprint{},
		&models.JiraConnection{},
		&models.JiraIssue{},
		&models.JiraIssueChangelogItems{},
		&models.JiraIssueChangelogs{},
		&models.JiraIssueCommit{},
		&models.JiraIssueLabel{},
		&models.JiraIssueType{},
		&models.JiraProject{},
		&models.JiraRemotelink{},
		&models.JiraServerInfo{},
		&models.JiraSprint{},
		&models.JiraSprintIssue{},
		&models.JiraSprintReport{},
		&models.JiraStatus{},
		&models.JiraWorklog{},
		&models.JiraIssueComment{},
		&models.JiraIssueRelationship{},
		&models.JiraScopeConfig{},
		&models.JiraIssueField{},
	}
}

func (p Jira) Description() string {
	return "To collect and enrich data from JIRA"
}

func (p Jira) Name() string {
	return "jira"
}

func (p Jira) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectIssueFieldsMeta,
		tasks.ExtractIssueFieldsMeta,

		tasks.CollectBoardFilterBeginMeta,

		tasks.CollectStatusMeta,
		tasks.ExtractStatusMeta,

		tasks.CollectProjectsMeta,
		tasks.ExtractProjectsMeta,

		tasks.CollectIssueTypesMeta,
		tasks.ExtractIssueTypesMeta,

		tasks.CollectIssuesMeta,
		tasks.ExtractIssuesMeta,

		tasks.CollectParentIssuesMeta,

		tasks.ConvertIssueLabelsMeta,

		tasks.CollectIssueCommentsMeta,
		tasks.ExtractIssueCommentsMeta,

		tasks.CollectIssueChangelogsMeta,
		tasks.ExtractIssueChangelogsMeta,

		tasks.CollectWorklogsMeta,
		tasks.ExtractWorklogsMeta,

		tasks.CollectRemotelinksMeta,
		tasks.ExtractRemotelinksMeta,

		tasks.CollectSprintsMeta,
		tasks.ExtractSprintsMeta,

		tasks.CollectSprintReportMeta,
		tasks.ExtractSprintReportMeta,

		tasks.CollectEpicsMeta,
		tasks.ExtractEpicsMeta,
		tasks.CleanupStaleBoardIssuesMeta,

		tasks.CollectAccountsMeta,

		tasks.ConvertBoardMeta,

		tasks.ConvertIssuesMeta,
		tasks.ConvertIssueCommentsMeta,
		tasks.ConvertWorklogsMeta,
		tasks.ConvertIssueChangelogsMeta,
		tasks.ConvertIssueRelationshipsMeta,

		tasks.ConvertSprintsMeta,
		tasks.ConvertSprintIssuesMeta,
		tasks.ConvertSprintReportMeta,

		tasks.CollectDevelopmentPanelMeta,
		tasks.ExtractDevelopmentPanelMeta,

		tasks.ConvertIssueCommitsMeta,
		tasks.ConvertIssueRepoCommitsMeta,

		tasks.ExtractAccountsMeta,
		tasks.ConvertAccountsMeta,

		tasks.CollectBoardFilterEndMeta,
	}
}

func (p Jira) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	var op tasks.JiraOptions
	var err errors.Error
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	logger.Debug("%v", options)
	err = helper.Decode(options, &op, nil)
	if err != nil {
		return nil, errors.Default.Wrap(err, "could not decode Jira options")
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New("jira connectionId is invalid")
	}
	connection := &models.JiraConnection{}
	connectionHelper := helper.NewConnectionHelper(
		taskCtx,
		nil,
		p.Name(),
	)
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get Jira connection")
	}
	jiraApiClient, err := tasks.NewJiraApiClient(taskCtx, connection)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create jira api client")
	}

	var scope *models.JiraBoard
	if op.BoardId != 0 {
		// support v100 & advance mode
		// If we still cannot find the record in db, we have to request from remote server and save it to db
		db := taskCtx.GetDal()
		err = db.First(&scope, dal.Where("connection_id = ? AND board_id = ?", op.ConnectionId, op.BoardId))
		if err != nil && db.IsErrorNotFound(err) {
			var board *apiv2models.Board
			board, err = api.GetApiJira(&op, jiraApiClient)
			if err != nil {
				return nil, err
			}
			logger.Debug(fmt.Sprintf("Current project: %d", board.ID))
			scope = board.ToToolLayer(connection.ID)
			err = db.CreateIfNotExist(&scope)
			if err != nil {
				return nil, err
			}
		}
		if err != nil {
			return nil, errors.Default.Wrap(err, fmt.Sprintf("fail to find board: %d", op.BoardId))
		}
		if op.ScopeConfigId == 0 && scope.ScopeConfigId != 0 {
			op.ScopeConfigId = scope.ScopeConfigId
		}
	}
	if op.ScopeConfig == nil && op.ScopeConfigId != 0 {
		var scopeConfig models.JiraScopeConfig
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
		op.ScopeConfig = new(models.JiraScopeConfig)
	}

	// set default page size
	if op.PageSize <= 0 || op.PageSize > 100 {
		op.PageSize = 100
	}

	info, code, err := tasks.GetJiraServerInfo(jiraApiClient)
	if err != nil || code != http.StatusOK || info == nil {
		return nil, errors.HttpStatus(code).Wrap(err, "fail to get Jira server info")
	}
	taskData := &tasks.JiraTaskData{
		Options:        &op,
		ApiClient:      jiraApiClient,
		JiraServerInfo: *info,
		Board:          scope,
		ProjectName:    op.ProjectName,
	}

	return taskData, nil
}

func (p Jira) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (pp coreModels.PipelinePlan, sc []plugin.Scope, err errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}

func (p Jira) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/jira"
}

func (p Jira) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Jira) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"test": {
			"POST": api.TestConnection,
		},
		"echo": {
			"POST": func(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
				return &plugin.ApiResourceOutput{Body: input.Body}, nil
			},
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
		"connections/:connectionId/proxy/rest/*path": {
			"GET": api.Proxy,
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
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.UpdateScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes/:scopeId/latest-sync-state": {
			"GET": api.GetScopeLatestSyncState,
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
		"connections/:connectionId/application-types": {
			"GET": api.GetApplicationTypes,
		},
		"connections/:connectionId/dev-panel-commits": {
			"GET": api.GetCommitsURLs,
		},
		"scope-config/:scopeConfigId/projects": {
			"GET": api.GetProjectsByScopeConfig,
		},
		"generate-regex": {
			"POST": api.GenRegex,
		},
		"apply-regex": {
			"POST": api.ApplyRegex,
		},
	}
}

func (p Jira) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.JiraTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	data.ApiClient.Release()
	return nil
}
