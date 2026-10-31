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
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/models"
	"reflect"
)

var ConvertSprintIssuesMeta = plugin.SubTaskMeta{
	Name:             "convertSprintIssues",
	EntryPoint:       ConvertSprintIssues,
	EnabledByDefault: true,
	Description:      "convert Jira sprint_issues from the board's Sprint Reports",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

// ConvertSprintIssues builds the domain sprint_issues links from
// _tool_jira_sprint_reports instead of _tool_jira_sprint_issues.
//
// _tool_jira_sprint_issues is populated by extractIssues from the issue's
// `sprint` / `closedSprints` fields, which the search API used by the issue
// collector does not return, so no new links were being produced. The
// Sprint Report is collected for every board, is Jira's own record of which
// issues were in each sprint, and is board-scoped, so this converter only
// reads (and only rewrites) the current board's links.
//
// The report's primary key is (connection_id, board_id, sprint_id, issue_id),
// so filtering by connection and board yields at most one row per
// (sprint_id, issue_id), matching the domain table's primary key.
//
// Sprint Reports only exist for closed sprints, so active and future sprints
// get no links from here.
func ConvertSprintIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*JiraTaskData)

	clauses := []dal.Clause{
		dal.Select("*"),
		dal.From(&models.JiraSprintReport{}),
		dal.Where("connection_id = ? AND board_id = ?", data.Options.ConnectionId, data.Options.BoardId),
		dal.Orderby("sprint_id, issue_id"),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	issueIdGen := didgen.NewDomainIdGenerator(&models.JiraIssue{})
	sprintIdGen := didgen.NewDomainIdGenerator(&models.JiraSprint{})

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		InputRowType: reflect.TypeOf(models.JiraSprintReport{}),
		Input:        cursor,
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
			Table: RAW_SPRINT_REPORT_TABLE,
		},
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			sprintReport := inputRow.(*models.JiraSprintReport)
			sprintIssue := &ticket.SprintIssue{
				SprintId: sprintIdGen.Generate(data.Options.ConnectionId, sprintReport.SprintId),
				IssueId:  issueIdGen.Generate(data.Options.ConnectionId, sprintReport.IssueId),
			}
			return []interface{}{sprintIssue}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
