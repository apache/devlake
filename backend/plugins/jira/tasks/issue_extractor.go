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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/utils"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks/apiv2models"
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
	sprintFieldId, err := getSprintFieldId(db, connectionId)
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
				"epicKeyField":    data.Options.ScopeConfig.EpicKeyField,
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

func extractIssues(data *JiraTaskData, mappings *typeMappings, apiIssue *apiv2models.Issue, row *api.RawData, userFieldMaps map[string]struct{}, sprintFieldId string) ([]interface{}, errors.Error) {
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
	// Issues collected through the search API (#8931) no longer carry
	// fields.sprint/fields.closedSprints; sprint membership only remains in the
	// Sprint custom field, so merge the ids parsed from it.
	if sprintFieldId != "" {
		sprints = mergeSprintIds(sprints, parseSprintCustomField(apiIssue.Fields.AllFields[sprintFieldId]))
	}
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
	// Honor the configured epic/parent-link custom field. Jira Cloud and
	// company-managed projects often store the epic key there instead of the
	// legacy fields.epic object. Leave EpicKey unchanged when the field is unset
	// so the standard epic mapping from toToolLayer still applies.
	if data.Options.ScopeConfig != nil && data.Options.ScopeConfig.EpicKeyField != "" {
		unknownEpicKey := apiIssue.Fields.AllFields[data.Options.ScopeConfig.EpicKeyField]
		switch ek := unknownEpicKey.(type) {
		case string:
			issue.EpicKey = ek
		case map[string]interface{}:
			if key, ok := ek["key"].(string); ok {
				issue.EpicKey = key
			}
		case nil:
			// Field is not set; keep EpicKey from the standard epic field.
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

// ghSprintSchemaCustom is the schema_custom value of the GreenHopper Sprint
// custom field. It stays stable across Jira instances even when the field is
// renamed, so it is the reliable way to locate the field in
// _tool_jira_issue_fields.
const ghSprintSchemaCustom = "com.pyxis.greenhopper.jira:gh-sprint"

// getSprintFieldId resolves the id (e.g. "customfield_10004") of the Sprint
// custom field for the connection. It returns "" when the field has not been
// collected, in which case sprint extraction keeps relying on
// fields.sprint/fields.closedSprints only.
func getSprintFieldId(db dal.Dal, connectionId uint64) (string, errors.Error) {
	var sprintFields []models.JiraIssueField
	err := db.All(&sprintFields, dal.Where("connection_id = ? AND schema_custom = ?", connectionId, ghSprintSchemaCustom))
	if err != nil {
		return "", err
	}
	if len(sprintFields) == 0 {
		return "", nil
	}
	return sprintFields[0].ID, nil
}

// sprintLegacyIdPattern matches the id inside a Jira Server/DC legacy sprint
// string such as
// "com.atlassian.greenhopper.service.sprint.Sprint@14e33bf4[...,id=2424,...]".
var sprintLegacyIdPattern = regexp.MustCompile(`(?:^|[,\[])id=(\d+)(?:[,\]])`)

// parseSprintCustomField extracts sprint ids from the Sprint custom field
// value found in Fields.AllFields. Jira Cloud returns an array of objects
// with an "id" property; Jira Server/DC returns an array of legacy strings.
func parseSprintCustomField(value interface{}) []uint64 {
	switch v := value.(type) {
	case nil:
		return nil
	case []interface{}:
		var ids []uint64
		for _, item := range v {
			if id, ok := sprintIdFromValue(item); ok {
				ids = append(ids, id)
			}
		}
		return ids
	default:
		if id, ok := sprintIdFromValue(v); ok {
			return []uint64{id}
		}
		return nil
	}
}

func sprintIdFromValue(item interface{}) (uint64, bool) {
	switch s := item.(type) {
	case map[string]interface{}:
		return toUint64(s["id"])
	case string:
		m := sprintLegacyIdPattern.FindStringSubmatch(s)
		if m == nil {
			return 0, false
		}
		id, err := strconv.ParseUint(m[1], 10, 64)
		return id, err == nil
	}
	return 0, false
}

func toUint64(value interface{}) (uint64, bool) {
	switch n := value.(type) {
	case float64:
		return uint64(n), true
	case string:
		id, err := strconv.ParseUint(n, 10, 64)
		return id, err == nil
	}
	return 0, false
}

// mergeSprintIds unions two sprint id lists, keeping the first occurrence
// order and dropping duplicates.
func mergeSprintIds(base []uint64, extra []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(base)+len(extra))
	merged := make([]uint64, 0, len(base)+len(extra))
	for _, id := range base {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			merged = append(merged, id)
		}
	}
	for _, id := range extra {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			merged = append(merged, id)
		}
	}
	return merged
}
