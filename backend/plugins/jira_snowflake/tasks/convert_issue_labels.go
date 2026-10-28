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

// Adapted from plugins/jira/tasks/issue_label_convertor.go.
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

var ConvertIssueLabelsMeta = plugin.SubTaskMeta{
	Name:             "convertIssueLabels",
	EntryPoint:       ConvertIssueLabels,
	EnabledByDefault: false,
	Description:      "Convert Jira issue labels into domain layer ticket.IssueLabel",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertIssueLabels(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*JiraSnowflakeTaskData)

	cursor, err := db.Cursor(
		dal.Select("jil.*"),
		dal.From("_tool_jira_issue_labels jil"),
		dal.Join(`LEFT JOIN _tool_jira_board_issues jbi
              ON jil.connection_id = jbi.connection_id AND jil.issue_id = jbi.issue_id`),
		dal.Where("jil.connection_id = ? AND jbi.board_id = ?", data.Options.ConnectionId, data.Options.BoardId),
		dal.Orderby("issue_id ASC"),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	issueIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraIssue{})
	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
			Table: "jira_api_issues",
		},
		InputRowType: reflect.TypeOf(jiramodels.JiraIssueLabel{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			issueLabel := inputRow.(*jiramodels.JiraIssueLabel)
			return []interface{}{
				&ticket.IssueLabel{
					IssueId:   issueIdGen.Generate(data.Options.ConnectionId, issueLabel.IssueId),
					LabelName: issueLabel.LabelName,
				},
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
