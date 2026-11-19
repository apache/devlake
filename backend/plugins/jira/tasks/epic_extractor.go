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

package tasks

import (
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks/apiv2models"
)

var _ plugin.SubTaskEntryPoint = ExtractEpics

var ExtractEpicsMeta = plugin.SubTaskMeta{
	Name:             "extractEpics",
	EntryPoint:       ExtractEpics,
	EnabledByDefault: true,
	Description:      "extract Jira epics from all boards",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
}

func ExtractEpics(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraTaskData)
	db := subtaskCtx.GetDal()
	connectionId := data.Options.ConnectionId
	boardId := data.Options.BoardId
	logger := subtaskCtx.GetLogger()
	logger.Info("extract external epic Issues, connection_id=%d, board_id=%d", connectionId, boardId)
	mappings, err := getTypeMappings(data, db)
	if err != nil {
		return err
	}
	userFieldMap, err := getUserFieldMap(db, connectionId, logger)
	if err != nil {
		return err
	}
	sprintFieldId, err := getSprintFieldId(db, connectionId)
	if err != nil {
		return err
	}

	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[apiv2models.Issue]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          RAW_EPIC_TABLE,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
			SubtaskConfig: map[string]any{
				"typeMappings":    mappings,
				"storyPointField": data.Options.ScopeConfig.StoryPointField,
				"dueDateField":    data.Options.ScopeConfig.DueDateField,
			},
		},
		BeforeExtract: func(apiIssue *apiv2models.Issue, stateManager *api.SubtaskStateManager) errors.Error {
			if stateManager.IsIncremental() {
				err := db.Delete(
					&models.JiraIssueLabel{},
					dal.Where("connection_id = ? AND issue_id = ?", data.Options.ConnectionId, apiIssue.ID),
				)
				if err != nil {
					return err
				}
				err = db.Delete(
					&models.JiraIssueRelationship{},
					dal.Where("connection_id = ? AND issue_id = ?", data.Options.ConnectionId, apiIssue.ID),
				)
				if err != nil {
					return err
				}
			}
			return nil
		},
		Extract: func(apiIssue *apiv2models.Issue, row *api.RawData) ([]interface{}, errors.Error) {
			return extractIssues(data, mappings, apiIssue, row, userFieldMap, sprintFieldId)
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func getIssueFieldMap(db dal.Dal, connectionId uint64, logger log.Logger) (map[string]models.JiraIssueField, errors.Error) {
	var allIssueFields []models.JiraIssueField
	if err := db.All(&allIssueFields, dal.Where("connection_id = ?", connectionId)); err != nil {
		return nil, err
	}
	issueFieldMap := make(map[string]models.JiraIssueField)
	for _, v := range allIssueFields {
		if existing, ok := issueFieldMap[v.Name]; ok {
			// Same field can appear on multiple boards; only warn on a real name collision.
			if existing.ID != v.ID {
				logger.Warn(nil, "field name %s is duplicated (IDs: %s, %s)", v.Name, existing.ID, v.ID)
			}
			if v.SchemaType == "user" {
				issueFieldMap[v.Name] = v
			}
		} else {
			issueFieldMap[v.Name] = v
		}
	}
	return issueFieldMap, nil
}

func getUserFieldMap(db dal.Dal, connectionId uint64, logger log.Logger) (map[string]struct{}, errors.Error) {
	userFieldMap := make(map[string]struct{})
	issueFieldMap, err := getIssueFieldMap(db, connectionId, logger)
	if err != nil {
		return nil, err
	}
	for filedName, issueField := range issueFieldMap {
		if issueField.SchemaType == "user" {
			userFieldMap[filedName] = struct{}{}
		}
	}
	return userFieldMap, nil
}
