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

// Adapted from plugins/jira/tasks/sprint_issues_convertor.go.
// TODO: long-term, extract shared convertor logic to helpers/jiraconvertors/.

import (
	"reflect"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var ConvertSprintIssuesMeta = plugin.SubTaskMeta{
	Name:             "convertSprintIssues",
	EntryPoint:       ConvertSprintIssues,
	EnabledByDefault: false,
	Description:      "Convert Jira sprint-issue memberships into domain layer ticket.SprintIssue",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertSprintIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*JiraSnowflakeTaskData)

	clauses := []dal.Clause{
		dal.Select("*"),
		dal.From(&jiramodels.JiraSprintIssue{}),
		dal.Where("_tool_jira_sprint_issues.connection_id = ?", data.Options.ConnectionId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	issueIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraIssue{})
	sprintIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraSprint{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		InputRowType: reflect.TypeOf(jiramodels.JiraSprintIssue{}),
		Input:        cursor,
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
			Table: "jira_api_issues",
		},
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			jiraSprintIssue := inputRow.(*jiramodels.JiraSprintIssue)
			return []interface{}{
				&ticket.SprintIssue{
					SprintId: sprintIdGen.Generate(data.Options.ConnectionId, jiraSprintIssue.SprintId),
					IssueId:  issueIdGen.Generate(data.Options.ConnectionId, jiraSprintIssue.IssueId),
				},
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
