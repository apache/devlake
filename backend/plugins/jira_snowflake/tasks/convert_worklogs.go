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

// Adapted from plugins/jira/tasks/worklog_convertor.go.
// TODO: long-term, extract shared convertor logic to helpers/jiraconvertors/.

import (
	"reflect"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var ConvertWorklogsMeta = plugin.SubTaskMeta{
	Name:             "convertWorklogs",
	EntryPoint:       ConvertWorklogs,
	EnabledByDefault: false,
	Description:      "Convert Jira worklogs into domain layer ticket.IssueWorklog",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertWorklogs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JiraSnowflakeTaskData)
	db := taskCtx.GetDal()

	clauses := []dal.Clause{
		dal.From(&jiramodels.JiraWorklog{}),
		dal.Select("_tool_jira_worklogs.*"),
		dal.Join(`LEFT JOIN _tool_jira_board_issues
              ON _tool_jira_board_issues.connection_id = _tool_jira_worklogs.connection_id
              AND _tool_jira_board_issues.issue_id = _tool_jira_worklogs.issue_id`),
		dal.Where(
			"_tool_jira_board_issues.connection_id = ? AND _tool_jira_board_issues.board_id = ?",
			data.Options.ConnectionId, data.Options.BoardId,
		),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	worklogIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraWorklog{})
	accountIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraAccount{})
	issueIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraIssue{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
			Table: "jira_api_worklogs",
		},
		InputRowType: reflect.TypeOf(jiramodels.JiraWorklog{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			jiraWorklog := inputRow.(*jiramodels.JiraWorklog)
			worklog := &ticket.IssueWorklog{
				DomainEntity: domainlayer.DomainEntity{
					Id: worklogIdGen.Generate(jiraWorklog.ConnectionId, jiraWorklog.IssueId, jiraWorklog.WorklogId),
				},
				IssueId:          issueIdGen.Generate(jiraWorklog.ConnectionId, jiraWorklog.IssueId),
				TimeSpentMinutes: jiraWorklog.TimeSpentSeconds / 60,
				StartedDate:      &jiraWorklog.Started,
				LoggedDate:       &jiraWorklog.Updated,
			}
			if jiraWorklog.AuthorId != "" {
				worklog.AuthorId = accountIdGen.Generate(data.Options.ConnectionId, jiraWorklog.AuthorId)
			}
			return []interface{}{worklog}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
