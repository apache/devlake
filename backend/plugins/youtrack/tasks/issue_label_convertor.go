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
	"reflect"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

var ConvertIssueLabelsMeta = plugin.SubTaskMeta{
	Name:             "Convert Issue Labels",
	EntryPoint:       ConvertIssueLabels,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_youtrack_issue_labels into domain layer table issue_labels",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.YoutrackIssueLabel{}.TableName(), models.YoutrackIssue{}.TableName(), RAW_ISSUES_TABLE},
	ProductTables:    []string{ticket.IssueLabel{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertIssueLabels

func ConvertIssueLabels(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*YoutrackTaskData)
	connectionId := data.Options.ConnectionId
	issueIdGen := didgen.NewDomainIdGenerator(&models.YoutrackIssue{})

	cursor, err := db.Cursor(
		dal.Select("l.*"),
		dal.From("_tool_youtrack_issue_labels l"),
		dal.Join("LEFT JOIN _tool_youtrack_issues i ON (i.connection_id = l.connection_id AND i.id = l.issue_id)"),
		dal.Where("l.connection_id = ? AND i.project_id = ?", connectionId, data.Options.ProjectId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Options: models.YoutrackApiParams{
				ConnectionId: connectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_ISSUES_TABLE,
		},
		InputRowType: reflect.TypeOf(models.YoutrackIssueLabel{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			label := inputRow.(*models.YoutrackIssueLabel)
			domainLabel := &ticket.IssueLabel{
				IssueId:   issueIdGen.Generate(connectionId, label.IssueId),
				LabelName: label.LabelName,
			}
			return []interface{}{domainLabel}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
