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

// Adapted from plugins/jira/tasks/issue_changelog_convertor.go.
// Simplification: user-field type detection (getIssueFieldMap) is omitted because
// _tool_jira_issue_fields is not populated in jira_snowflake (no CollectIssueFieldsMeta).
// TODO: long-term, extract shared convertor logic to helpers/jiraconvertors/.

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var validID = regexp.MustCompile(`[0-9]+`)

var ConvertChangelogsMeta = plugin.SubTaskMeta{
	Name:             "convertChangelogs",
	EntryPoint:       ConvertChangelogs,
	EnabledByDefault: true,
	Description:      "Convert Jira issue changelogs into domain layer ticket.IssueChangelogs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
}

type issueChangelogItemResult struct {
	jiramodels.JiraIssueChangelogItems
	IssueId           uint64 `gorm:"index"`
	AuthorAccountId   string
	AuthorDisplayName string
	Created           time.Time
}

func ConvertChangelogs(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	connectionId := data.Options.ConnectionId
	boardId := data.Options.BoardId

	var allStatus []jiramodels.JiraStatus
	if err := db.All(&allStatus, dal.Where("connection_id = ?", connectionId)); err != nil {
		return err
	}
	statusMap := make(map[string]jiramodels.JiraStatus)
	for _, v := range allStatus {
		statusMap[v.ID] = v
	}

	issueIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraIssue{})
	sprintIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraSprint{})
	changelogIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraIssueChangelogItems{})
	accountIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraAccount{})

	converter, err := helper.NewStatefulDataConverter(&helper.StatefulDataConverterArgs[issueChangelogItemResult]{
		SubtaskCommonArgs: &helper.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          "jira_api_issues",
			Params: JiraApiParams{
				ConnectionId: connectionId,
				BoardId:      boardId,
			},
		},
		Input: func(stateManager *helper.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.Select("_tool_jira_issue_changelog_items.*, _tool_jira_issue_changelogs.issue_id, author_account_id, author_display_name, created"),
				dal.From("_tool_jira_issue_changelog_items"),
				dal.Join(`LEFT JOIN _tool_jira_issue_changelogs ON (
					_tool_jira_issue_changelogs.connection_id = _tool_jira_issue_changelog_items.connection_id
					AND _tool_jira_issue_changelogs.changelog_id = _tool_jira_issue_changelog_items.changelog_id
				)`),
				dal.Join(`LEFT JOIN _tool_jira_board_issues ON (
					_tool_jira_board_issues.connection_id = _tool_jira_issue_changelogs.connection_id
					AND _tool_jira_board_issues.issue_id = _tool_jira_issue_changelogs.issue_id
				)`),
				dal.Where("_tool_jira_issue_changelog_items.connection_id = ? AND _tool_jira_board_issues.board_id = ?", connectionId, boardId),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_jira_issue_changelog_items.created_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(row *issueChangelogItemResult) ([]interface{}, errors.Error) {
			changelog := &ticket.IssueChangelogs{
				DomainEntity: domainlayer.DomainEntity{
					Id: changelogIdGen.Generate(row.ConnectionId, row.ChangelogId, row.Field),
				},
				IssueId:           issueIdGen.Generate(row.ConnectionId, row.IssueId),
				AuthorId:          accountIdGen.Generate(connectionId, row.AuthorAccountId),
				AuthorName:        row.AuthorDisplayName,
				FieldId:           row.FieldId,
				FieldName:         row.Field,
				OriginalFromValue: row.FromString,
				OriginalToValue:   row.ToString,
				CreatedDate:       row.Created,
			}
			switch row.Field {
			case "assignee", "reporter":
				if row.FromValue != "" {
					changelog.OriginalFromValue = accountIdGen.Generate(connectionId, row.FromValue)
				}
				if row.ToValue != "" {
					changelog.OriginalToValue = accountIdGen.Generate(connectionId, row.ToValue)
				}
			case "Sprint":
				var e errors.Error
				changelog.OriginalFromValue, e = convertSprintIds(row.FromValue, connectionId, sprintIdGen)
				if e != nil {
					return nil, e
				}
				changelog.OriginalToValue, e = convertSprintIds(row.ToValue, connectionId, sprintIdGen)
				if e != nil {
					return nil, e
				}
			case "status":
				if fromStatus, ok := statusMap[row.FromValue]; ok {
					changelog.OriginalFromValue = fromStatus.Name
					changelog.FromValue = getStdStatus(fromStatus.StatusCategory)
				}
				if toStatus, ok := statusMap[row.ToValue]; ok {
					changelog.OriginalToValue = toStatus.Name
					changelog.ToValue = getStdStatus(toStatus.StatusCategory)
				}
			}
			return []interface{}{changelog}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

func convertSprintIds(ids string, connectionId uint64, gen *didgen.DomainIdGenerator) (string, errors.Error) {
	var result []string
	for _, item := range strings.Split(ids, ",") {
		item = strings.TrimSpace(item)
		item = validID.FindString(item)
		if item != "" {
			id, parseErr := strconv.ParseUint(item, 10, 64)
			if parseErr != nil {
				return "", errors.Convert(parseErr)
			}
			result = append(result, gen.Generate(connectionId, id))
		}
	}
	return strings.Join(result, ","), nil
}
