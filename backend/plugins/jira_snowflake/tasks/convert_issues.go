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

// Adapted from plugins/jira/tasks/issue_convertor.go.
// Key adaptation: full-sync deletion uses board scope (connection_id + board_id) instead
// of _raw_data_table matching, because jira_snowflake has no raw-table layer.
// TODO: long-term, extract shared convertor logic to helpers/jiraconvertors/.

import (
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var ConvertIssuesMeta = plugin.SubTaskMeta{
	Name:             "convertIssues",
	EntryPoint:       ConvertIssues,
	EnabledByDefault: true,
	Description:      "Convert Jira issues from tool layer into domain layer ticket.Issue",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertIssues(subtaskCtx plugin.SubTaskContext) errors.Error {
	logger := subtaskCtx.GetLogger()
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()

	mappings, err := getTypeMappings(data, db)
	if err != nil {
		return err
	}

	issueIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraAccount{})
	boardIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraBoard{})
	boardId := boardIdGen.Generate(data.Options.ConnectionId, data.Options.BoardId)

	converter, err := helper.NewStatefulDataConverter(&helper.StatefulDataConverterArgs[jiramodels.JiraIssue]{
		SubtaskCommonArgs: &helper.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          "jira_api_issues",
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
			SubtaskConfig: mappings,
		},
		Input: func(stateManager *helper.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.Select("_tool_jira_issues.*"),
				dal.From("_tool_jira_issues"),
				dal.Join(`LEFT JOIN _tool_jira_board_issues
					ON _tool_jira_board_issues.issue_id = _tool_jira_issues.issue_id
					AND _tool_jira_board_issues.connection_id = _tool_jira_issues.connection_id`),
				dal.Where(
					"_tool_jira_board_issues.connection_id = ? AND _tool_jira_board_issues.board_id = ?",
					data.Options.ConnectionId,
					data.Options.BoardId,
				),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_jira_issues.updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(jiraIssue *jiramodels.JiraIssue) ([]interface{}, errors.Error) {
			var result []interface{}
			issue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: issueIdGen.Generate(jiraIssue.ConnectionId, jiraIssue.IssueId),
				},
				Url:                     issueURL(jiraIssue.IssueKey),
				IssueKey:                jiraIssue.IssueKey,
				Title:                   jiraIssue.Summary,
				Description:             jiraIssue.Description,
				EpicKey:                 jiraIssue.EpicKey,
				Type:                    jiraIssue.StdType,
				OriginalType:            jiraIssue.Type,
				Status:                  jiraIssue.StdStatus,
				OriginalStatus:          jiraIssue.StatusName,
				StoryPoint:              jiraIssue.StoryPoint,
				OriginalEstimateMinutes: jiraIssue.OriginalEstimateMinutes,
				ResolutionDate:          jiraIssue.ResolutionDate,
				Priority:                jiraIssue.PriorityName,
				CreatedDate:             &jiraIssue.Created,
				UpdatedDate:             &jiraIssue.Updated,
				LeadTimeMinutes:         jiraIssue.LeadTimeMinutes,
				TimeSpentMinutes:        jiraIssue.SpentMinutes,
				TimeRemainingMinutes:    &jiraIssue.RemainingEstimateMinutes,
				OriginalProject:         jiraIssue.ProjectName,
				Component:               jiraIssue.Components,
				IsSubtask:               jiraIssue.Subtask,
				DueDate:                 jiraIssue.DueDate,
				FixVersions:             jiraIssue.FixVersions,
			}
			if jiraIssue.CreatorAccountId != "" {
				issue.CreatorId = accountIdGen.Generate(data.Options.ConnectionId, jiraIssue.CreatorAccountId)
			}
			if jiraIssue.CreatorDisplayName != "" {
				issue.CreatorName = jiraIssue.CreatorDisplayName
			}
			if jiraIssue.AssigneeDisplayName != "" {
				issue.AssigneeName = jiraIssue.AssigneeDisplayName
			}
			if jiraIssue.ParentId != 0 {
				issue.ParentIssueId = issueIdGen.Generate(data.Options.ConnectionId, jiraIssue.ParentId)
			}
			// only set type to subtask if no type mapping is set
			mapped, ok := mappings.StdTypeMappings[jiraIssue.Type]
			if !(ok && mapped != "") && jiraIssue.Subtask {
				issue.Type = ticket.SUBTASK
			}
			result = append(result, issue)
			if jiraIssue.AssigneeAccountId != "" {
				issue.AssigneeId = accountIdGen.Generate(data.Options.ConnectionId, jiraIssue.AssigneeAccountId)
				issueAssignee := &ticket.IssueAssignee{
					IssueId:      issue.Id,
					AssigneeId:   issue.AssigneeId,
					AssigneeName: issue.AssigneeName,
				}
				result = append(result, issueAssignee)
			}
			result = append(result, &ticket.BoardIssue{
				BoardId: boardId,
				IssueId: issue.Id,
			})
			return result, nil
		},
	})
	if err != nil {
		return err
	}

	// Full-sync deletion: since there is no raw-table layer, we cannot use
	// the _raw_data_table-based deletion from the upstream jira convertor.
	// Instead we delete domain rows scoped to this board's domain ID.
	//
	// Trade-off: deletion happens before conversion rather than after. If the
	// converter fails partway through, previously synced domain data is lost
	// until the next successful full sync. The alternative (delete after) is
	// unsafe here because converter.Execute() upserts rows with the same
	// _raw_data_params scope, so a post-convert delete would erase the freshly
	// written data. Acceptable given that full-sync is an admin-triggered operation
	// and re-running it is straightforward.
	if !converter.IsIncremental() {
		logger.Debug("deleting outdated domain rows for board %d", data.Options.BoardId)
		if dbErr := db.Delete(&ticket.Issue{}, dal.Where("_raw_data_params = ?", converter.GetRawDataParams())); dbErr != nil {
			logger.Error(dbErr, "delete issues")
			return dbErr
		}
		if dbErr := db.Delete(&ticket.IssueAssignee{}, dal.Where("_raw_data_params = ?", converter.GetRawDataParams())); dbErr != nil {
			logger.Error(dbErr, "delete issue_assignees")
			return dbErr
		}
		if dbErr := db.Delete(&ticket.BoardIssue{}, dal.Where("board_id = ?", boardId)); dbErr != nil {
			logger.Error(dbErr, "delete board_issues")
			return dbErr
		}
	}

	return converter.Execute()
}

// issueURL returns the issue key as the URL placeholder.
// The Jira base URL is PII-stripped in Snowflake so we cannot construct a full URL.
func issueURL(issueKey string) string {
	return issueKey
}
