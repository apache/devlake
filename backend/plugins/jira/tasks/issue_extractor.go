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
	"strconv"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/utils"
	"github.com/apache/incubator-devlake/plugins/jira/models"
	"github.com/apache/incubator-devlake/plugins/jira/tasks/apiv2models"
)

var _ plugin.SubTaskEntryPoint = ExtractIssues

var ExtractIssuesMeta = plugin.SubTaskMeta{
	Name:             "extractIssues",
	EntryPoint:       ExtractIssues,
	EnabledByDefault: true,
	Description:      "extract Jira issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
}

type typeMappings struct {
	TypeIdMappings         map[string]string
	StdTypeMappings        map[string]string
	StandardStatusMappings map[string]models.StatusMappings
}

func ExtractIssues(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraTaskData)
	db := subtaskCtx.GetDal()
	connectionId := data.Options.ConnectionId
	boardId := data.Options.BoardId
	logger := subtaskCtx.GetLogger()
	logger.Info("extract Issues, connection_id=%d, board_id=%d", connectionId, boardId)
	mappings, err := getTypeMappings(data, db)
	if err != nil {
		return err
	}
	userFieldMap, err := getUserFieldMap(db, connectionId, logger)
	if err != nil {
		return err
	}
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[apiv2models.Issue]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          RAW_ISSUE_TABLE,
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
			return extractIssues(data, mappings, apiIssue, row, userFieldMap)
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func extractIssues(data *JiraTaskData, mappings *typeMappings, apiIssue *apiv2models.Issue, row *api.RawData, userFieldMaps map[string]struct{}) ([]interface{}, errors.Error) {
	err := apiIssue.SetAllFields(row.Data)
	if err != nil {
		return nil, err
	}
	var results []interface{}
	// if the field `created` is nil, ignore it
	if apiIssue.Fields.Created == nil {
		return results, nil
	}
	sprints, issue, comments, worklogs, changelogs, changelogItems, users := apiIssue.ExtractEntities(data.Options.ConnectionId, userFieldMaps)
	for _, sprintId := range sprints {
		sprintIssue := &models.JiraSprintIssue{
			ConnectionId:     data.Options.ConnectionId,
			SprintId:         sprintId,
			IssueId:          issue.IssueId,
			IssueCreatedDate: &issue.Created,
			ResolutionDate:   issue.ResolutionDate,
		}
		results = append(results, sprintIssue)
	}
	if issue.ResolutionDate != nil {
		temp := uint(issue.ResolutionDate.Unix()-issue.Created.Unix()) / 60
		issue.LeadTimeMinutes = &temp
	}
	if data.Options.ScopeConfig != nil && data.Options.ScopeConfig.StoryPointField != "" {
		unknownStoryPoint := apiIssue.Fields.AllFields[data.Options.ScopeConfig.StoryPointField]
		switch sp := unknownStoryPoint.(type) {
		case string:
			// string, try to parse
			temp, _ := strconv.ParseFloat(sp, 32)
			issue.StoryPoint = &temp
		case nil:
		default:
			// not string, convert to float64, ignore it if failed
			temp, _ := unknownStoryPoint.(float64)
			issue.StoryPoint = &temp
		}

	}
	// default due date field is "duedate"
	dueDateField := "duedate"
	if data.Options.ScopeConfig != nil && data.Options.ScopeConfig.DueDateField != "" {
		dueDateField = data.Options.ScopeConfig.DueDateField
	}
	// using location of issues.Created
	loc := issue.Created.Location()
	issue.DueDate, _ = utils.GetTimeFieldFromMap(apiIssue.Fields.AllFields, dueDateField, loc)
	// code in next line will set issue.Type to issueType.Name
	issue.Type = mappings.TypeIdMappings[issue.Type]
	issue.StdType = mappings.StdTypeMappings[issue.Type]
	if issue.StdType == "" {
		issue.StdType = strings.ToUpper(issue.Type)
	}
	issue.StdStatus = getStdStatus(issue.StatusKey)
	if value, ok := mappings.StandardStatusMappings[issue.Type][issue.StatusKey]; ok {
		issue.StdStatus = value.StandardStatus
	}
	// issue comments
	results = append(results, issue)
	for _, comment := range comments {
		results = append(results, comment)
	}
	// worklogs
	for _, worklog := range worklogs {
		results = append(results, worklog)
	}
	var issueUpdated *time.Time
	// likely this issue has more changelogs to be collected
	if len(changelogs) == 100 {
		issueUpdated = nil
	} else {
		issueUpdated = &issue.Updated
	}
	// changelogs
	for _, changelog := range changelogs {
		changelog.IssueUpdated = issueUpdated
		results = append(results, changelog)
	}
	// changelog items
	for _, changelogItem := range changelogItems {
		results = append(results, changelogItem)
	}
	// users
	for _, user := range users {
		if user.AccountId != "" {
			results = append(results, user)
		}
	}
	results = append(results, &models.JiraBoardIssue{
		ConnectionId: data.Options.ConnectionId,
		BoardId:      data.Options.BoardId,
		IssueId:      issue.IssueId,
	})
	// labels
	labels := apiIssue.Fields.Labels
	for _, v := range labels {
		issueLabel := &models.JiraIssueLabel{
			IssueId:      issue.IssueId,
			LabelName:    v,
			ConnectionId: data.Options.ConnectionId,
		}
		results = append(results, issueLabel)
	}
	// components
	components := apiIssue.Fields.Components
	var componentNames []string
	for _, v := range components {
		componentNames = append(componentNames, v.Name)
	}
	issue.Components = strings.Join(componentNames, ",")

	// fix versions
	fixVersions := apiIssue.Fields.FixVersions
	var fixVersionsNames []string
	for _, v := range fixVersions {
		fixVersionsNames = append(fixVersionsNames, v.Name)
	}
	issue.FixVersions = strings.Join(fixVersionsNames, ",")

	// issuelinks
	issuelinks := apiIssue.Fields.Issuelinks
	for _, v := range issuelinks {
		issueLink := &models.JiraIssueRelationship{
			ConnectionId:    data.Options.ConnectionId,
			IssueId:         issue.IssueId,
			IssueKey:        issue.IssueKey,
			TypeId:          v.Type.ID,          // Extracting the TypeId from the issuelink
			TypeName:        v.Type.Name,        // Extracting the TypeName from the issuelink
			Inward:          v.Type.Inward,      // Extracting the Inward from the issuelink
			Outward:         v.Type.Outward,     // Extracting the Outward from the issuelink
			InwardIssueId:   v.InwardIssue.ID,   // Extracting the InwardIssueId from the issuelink
			InwardIssueKey:  v.InwardIssue.Key,  // Extracting the InwardIssueKey from the issuelink
			OutwardIssueId:  v.OutwardIssue.ID,  // Extracting the OutwardIssueId from the issuelink
			OutwardIssueKey: v.OutwardIssue.Key, // Extracting the OutwardIssueKey from the issuelink
		}
		results = append(results, issueLink)
	}

	// is subtask
	issue.Subtask = apiIssue.Fields.Issuetype.Subtask

	return results, nil
}

func getTypeMappings(data *JiraTaskData, db dal.Dal) (*typeMappings, errors.Error) {
	typeIdMapping := make(map[string]string)
	issueTypes := make([]models.JiraIssueType, 0)
	clauses := []dal.Clause{
		dal.From(&models.JiraIssueType{}),
		dal.Where("connection_id = ?", data.Options.ConnectionId),
	}
	err := db.All(&issueTypes, clauses...)
	if err != nil {
		return nil, err
	}
	for _, issueType := range issueTypes {
		typeIdMapping[issueType.Id] = issueType.Name
	}
	stdTypeMappings := make(map[string]string)
	standardStatusMappings := make(map[string]models.StatusMappings)
	if data.Options.ScopeConfig != nil {
		for userType, stdType := range data.Options.ScopeConfig.TypeMappings {
			stdTypeMappings[userType] = strings.ToUpper(stdType.StandardType)
			standardStatusMappings[userType] = stdType.StatusMappings
		}
	}
	return &typeMappings{
		TypeIdMappings:         typeIdMapping,
		StdTypeMappings:        stdTypeMappings,
		StandardStatusMappings: standardStatusMappings,
	}, nil
}
