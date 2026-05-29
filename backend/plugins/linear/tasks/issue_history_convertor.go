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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

var ConvertIssueHistoryMeta = plugin.SubTaskMeta{
	Name:             "Convert Issue History",
	EntryPoint:       ConvertIssueHistory,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_linear_issue_history into domain layer table issue_changelogs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.LinearIssueHistory{}.TableName(), models.LinearIssue{}.TableName(), RAW_ISSUE_HISTORY_TABLE},
	ProductTables:    []string{ticket.IssueChangelogs{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertIssueHistory

func ConvertIssueHistory(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)
	connectionId := data.Options.ConnectionId

	issueIdGen := didgen.NewDomainIdGenerator(&models.LinearIssue{})
	historyIdGen := didgen.NewDomainIdGenerator(&models.LinearIssueHistory{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.LinearAccount{})

	cursor, err := db.Cursor(
		dal.Select("h.*"),
		dal.From("_tool_linear_issue_history h"),
		dal.Join("LEFT JOIN _tool_linear_issues i ON (i.connection_id = h.connection_id AND i.id = h.issue_id)"),
		dal.Where("h.connection_id = ? AND i.team_id = ?", connectionId, data.Options.TeamId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: connectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ISSUE_HISTORY_TABLE,
		},
		InputRowType: reflect.TypeOf(models.LinearIssueHistory{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			event := inputRow.(*models.LinearIssueHistory)
			changelog := &ticket.IssueChangelogs{
				DomainEntity:      domainlayer.DomainEntity{Id: historyIdGen.Generate(connectionId, event.Id)},
				IssueId:           issueIdGen.Generate(connectionId, event.IssueId),
				FieldId:           "state",
				FieldName:         "status",
				OriginalFromValue: event.FromStateName,
				OriginalToValue:   event.ToStateName,
				CreatedDate:       event.CreatedAt,
			}
			if event.FromStateType != "" {
				changelog.FromValue = StatusFromStateType(event.FromStateType)
			}
			if event.ToStateType != "" {
				changelog.ToValue = StatusFromStateType(event.ToStateType)
			}
			if event.ActorId != "" {
				changelog.AuthorId = accountIdGen.Generate(connectionId, event.ActorId)
			}
			return []interface{}{changelog}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
