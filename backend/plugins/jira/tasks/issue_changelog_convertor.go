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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/jira/models"
)

var validID = regexp.MustCompile(`[0-9]+`)

var ConvertIssueChangelogsMeta = plugin.SubTaskMeta{
	Name:             "convertIssueChangelogs",
	EntryPoint:       ConvertIssueChangelogs,
	EnabledByDefault: true,
	Description:      "convert Jira Issue change logs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
}

type IssueChangelogItemResult struct {
	models.JiraIssueChangelogItems
	IssueId           uint64 `gorm:"index"`
	AuthorAccountId   string
	AuthorDisplayName string
	Created           time.Time
}

func ConvertIssueChangelogs(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()
	connectionId := data.Options.ConnectionId
	boardId := data.Options.BoardId

	var allStatus []models.JiraStatus
	err := db.All(&allStatus, dal.Where("connection_id = ?", connectionId))
	if err != nil {
		return err
	}
	statusMap := make(map[string]models.JiraStatus)
	for _, v := range allStatus {
		statusMap[v.ID] = v
	}

	issueFieldMap, err := getIssueFieldMap(db, connectionId, logger)
	if err != nil {
		return err
	}

	issueIdGenerator := didgen.NewDomainIdGenerator(&models.JiraIssue{})
	sprintIdGenerator := didgen.NewDomainIdGenerator(&models.JiraSprint{})
	changelogIdGenerator := didgen.NewDomainIdGenerator(&models.JiraIssueChangelogItems{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.JiraAccount{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[IssueChangelogItemResult]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          RAW_ISSUE_TABLE,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.Select("_tool_jira_issue_changelog_items.*, _tool_jira_issue_changelogs.issue_id, author_account_id, author_display_name, created"),
				dal.From("_tool_jira_issue_changelog_items"),
				dal.Join(`left join _tool_jira_issue_changelogs on (
					_tool_jira_issue_changelogs.connection_id = _tool_jira_issue_changelog_items.connection_id
					AND _tool_jira_issue_changelogs.changelog_id = _tool_jira_issue_changelog_items.changelog_id
				)`),
				dal.Join(`inner join _tool_jira_board_issues on (
				    _tool_jira_board_issues.connection_id = _tool_jira_issue_changelogs.connection_id
				    AND _tool_jira_board_issues.issue_id = _tool_jira_issue_changelogs.issue_id
				    AND _tool_jira_board_issues.board_id = ?
				)`, boardId),
				dal.Where("_tool_jira_issue_changelog_items.connection_id = ?", connectionId),			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_jira_issue_changelogs.created >= ?", since))

				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(row *IssueChangelogItemResult) ([]interface{}, errors.Error) {
			changelog := &ticket.IssueChangelogs{
				DomainEntity: domainlayer.DomainEntity{Id: changelogIdGenerator.Generate(
					row.ConnectionId,
					row.ChangelogId,
					row.Field,
				)},
				IssueId:           issueIdGenerator.Generate(row.ConnectionId, row.IssueId),
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
				changelog.OriginalFromValue, err = convertIds(row.FromValue, connectionId, sprintIdGenerator)
				if err != nil {
					return nil, err
				}
				changelog.OriginalToValue, err = convertIds(row.ToValue, connectionId, sprintIdGenerator)
				if err != nil {
					return nil, err
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
			default:
				if v, ok := issueFieldMap[row.Field]; ok && v.SchemaType == "user" {
					if row.FromValue != "" {
						changelog.OriginalFromValue = accountIdGen.Generate(connectionId, row.FromValue)
					}
					if row.ToValue != "" {
						changelog.OriginalToValue = accountIdGen.Generate(connectionId, row.ToValue)
					}
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

func convertIds(ids string, connectionId uint64, sprintIdGenerator *didgen.DomainIdGenerator) (string, errors.Error) {
	ss := strings.Split(ids, ",")
	var resultSlice []string
	for _, item := range ss {
		item = strings.TrimSpace(item)
		item := validID.FindString(item)
		if item != "" {
			id, err := strconv.ParseUint(item, 10, 64)
			if err != nil {
				return "", errors.Convert(err)
			}
			resultSlice = append(resultSlice, sprintIdGenerator.Generate(connectionId, id))
		}
	}
	return strings.Join(resultSlice, ","), nil
}
