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

// Adapted from plugins/jira/tasks/board_convertor.go.
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

var ConvertBoardMeta = plugin.SubTaskMeta{
	Name:             "convertBoard",
	EntryPoint:       ConvertBoard,
	EnabledByDefault: true,
	Description:      "Convert Jira board into domain layer ticket.Board",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertBoard(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JiraSnowflakeTaskData)
	logger := taskCtx.GetLogger()
	db := taskCtx.GetDal()
	logger.Info("convert board:%d", data.Options.BoardId)

	idGen := didgen.NewDomainIdGenerator(&jiramodels.JiraBoard{})
	clauses := []dal.Clause{
		dal.Select("*"),
		dal.From(&jiramodels.JiraBoard{}),
		dal.Where("connection_id = ? AND board_id = ?", data.Options.ConnectionId, data.Options.BoardId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
			Table: "jira_api_boards",
		},
		InputRowType: reflect.TypeOf(jiramodels.JiraBoard{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			board := inputRow.(*jiramodels.JiraBoard)
			domainBoard := &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{
					Id: idGen.Generate(data.Options.ConnectionId, data.Options.BoardId),
				},
				Name: board.Name,
				Url:  board.Self,
				Type: board.Type,
			}
			return []interface{}{domainBoard}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
