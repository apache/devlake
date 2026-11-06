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
	"net/url"
	"path/filepath"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/models"
)

var ConvertIssuesMeta = plugin.SubTaskMeta{
	Name:             "convertIssues",
	EntryPoint:       ConvertIssues,
	EnabledByDefault: true,
	Description:      "convert Jira issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

// convertIssuesInputClauses selects the tool-layer issues that the convertor
// turns into domain issues/board_issues for one board.
//
// On a board SHARED by several DevLake projects, each project collected its own
// slice of the board (its ExtraJQL result) into _tool_jira_board_issues rows
// tagged with that project's name. The convertor therefore MUST filter by
// project_name as well as connection_id + board_id; otherwise a project's run
// would read every project's rows for the board and stamp them all with its own
// name, leaking issues across projects.
//
// The empty-project_name rows are always included so that pre-feature data
// (collected before project_name existed) and no-project pipelines keep working.
func convertIssuesInputClauses(connectionId uint64, boardId uint64, projectName string) []dal.Clause {
	return []dal.Clause{
		dal.Select("_tool_jira_issues.*"),
		dal.From("_tool_jira_issues"),
		dal.Join(`left join _tool_jira_board_issues
			on _tool_jira_board_issues.issue_id = _tool_jira_issues.issue_id
			and _tool_jira_board_issues.connection_id = _tool_jira_issues.connection_id`),
		dal.Where(
			`_tool_jira_board_issues.connection_id = ? AND _tool_jira_board_issues.board_id = ?
				AND (
					_tool_jira_board_issues.project_name = ?
					OR NOT EXISTS (
						SELECT 1 FROM _tool_jira_board_issues b2
						WHERE b2.connection_id = _tool_jira_board_issues.connection_id
						  AND b2.board_id = _tool_jira_board_issues.board_id
						  AND b2.project_name IS NOT NULL AND b2.project_name <> ''
					)
				)`,
			connectionId,
			boardId,
			projectName,
		),
	}
}

func ConvertIssues(subtaskCtx plugin.SubTaskContext) errors.Error {
	logger := subtaskCtx.GetLogger()
	data := subtaskCtx.GetData().(*JiraTaskData)
	db := subtaskCtx.GetDal()
	mappings, err := getTypeMappings(data, db)
	if err != nil {
		return err
	}

	issueIdGen := didgen.NewDomainIdGenerator(&models.JiraIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.JiraAccount{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.JiraBoard{})
	boardId := boardIdGen.Generate(data.Options.ConnectionId, data.Options.BoardId)

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[models.JiraIssue]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          RAW_ISSUE_TABLE,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
				ProjectName:  data.Options.ProjectName,
			},
			SubtaskConfig: mappings,
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := convertIssuesInputClauses(data.Options.ConnectionId, data.Options.BoardId, data.Options.ProjectName)
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_jira_issues.updated_at >= ? ", since))
				}
			}
			return db.Cursor(clauses...)
		},
		// not needed for now due to jira assignee and label are converted in FullSync(Delete+Insert) manner
		// BeforeConvert: func(jiraIssue *models.JiraIssue, stateManager *api.SubtaskStateManager) errors.Error {
		// 	issueId := issueIdGen.Generate(data.Options.ConnectionId, jiraIssue.IssueId)
		// 	if err := db.Delete(&ticket.IssueAssignee{}, dal.Where("issue_id = ?", issueId)); err != nil {
		// 		return err
		// 	}
		// 	if err := db.Delete(&ticket.IssueLabel{}, dal.Where("issue_id = ?", issueId)); err != nil {
		// 		return err
		// 	}
		// 	return nil
		// },
		Convert: func(jiraIssue *models.JiraIssue) ([]interface{}, errors.Error) {
			var result []interface{}
			issue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: issueIdGen.Generate(jiraIssue.ConnectionId, jiraIssue.IssueId),
				},
				Url:                     convertURL(jiraIssue.Self, jiraIssue.IssueKey),
				IconURL:                 jiraIssue.IconURL,
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
			boardIssue := &ticket.BoardIssue{
				BoardId:     boardId,
				IssueId:     issue.Id,
				ProjectName: data.ProjectName,
			}
			result = append(result, boardIssue)
			return result, nil
		},
	})

	if err != nil {
		return err
	}

	if !converter.IsIncremental() {
		logger.Debug("deleting outdated records for board_issues, issue_assignees and issues")
		dalWhere := dal.Where("_raw_data_table in ? AND _raw_data_params = ?",
			[]string{"_raw_jira_api_issues", "_raw_jira_api_epics"},
			converter.GetRawDataParams(),
		)
		if err := db.Delete(ticket.Issue{}, dalWhere); err != nil {
			logger.Error(err, "delete issues")
			return err
		}
		if err := db.Delete(ticket.IssueAssignee{}, dalWhere); err != nil {
			logger.Error(err, "delete issue_assignees")
			return err
		}
		if err := db.Delete(ticket.BoardIssue{}, dalWhere); err != nil {
			logger.Error(err, "delete board_issues")
			return err
		}
	}

	return converter.Execute()
}

func convertURL(api, issueKey string) string {
	u, err := url.Parse(api)
	if err != nil {
		return api
	}
	before, _, found := strings.Cut(u.Path, "/rest/agile/1.0/issue")
	if !found {
		before, _, _ = strings.Cut(u.Path, "/rest/api/2/issue")
	}
	u.Path = filepath.Join(before, "browse", issueKey)
	return u.String()
}
