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

// Adapted from plugins/jira/tasks/sprint_convertor.go.
// TODO: long-term, extract shared convertor logic to helpers/jiraconvertors/.

import (
	"reflect"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var ConvertSprintsMeta = plugin.SubTaskMeta{
	Name:             "convertSprints",
	EntryPoint:       ConvertSprints,
	EnabledByDefault: false,
	Description:      "Convert Jira sprints into domain layer ticket.Sprint",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertSprints(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*JiraSnowflakeTaskData)
	connectionId := data.Options.ConnectionId
	boardId := data.Options.BoardId
	db := taskCtx.GetDal()

	clauses := []dal.Clause{
		dal.Select("tjs.*"),
		dal.From("_tool_jira_sprints tjs"),
		dal.Join(`LEFT JOIN _tool_jira_board_sprints tjbs
              ON tjbs.sprint_id = tjs.sprint_id AND tjbs.connection_id = tjs.connection_id`),
		dal.Where("tjs.connection_id = ? AND tjbs.board_id = ?", connectionId, boardId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	domainBoardId := didgen.NewDomainIdGenerator(&jiramodels.JiraBoard{}).Generate(connectionId, boardId)
	sprintIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraSprint{})
	boardIdGen := didgen.NewDomainIdGenerator(&jiramodels.JiraBoard{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: JiraApiParams{
				ConnectionId: connectionId,
				BoardId:      boardId,
			},
			Table: "jira_api_sprints",
		},
		InputRowType: reflect.TypeOf(jiramodels.JiraSprint{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			jiraSprint := inputRow.(*jiramodels.JiraSprint)
			sprint := &ticket.Sprint{
				DomainEntity:    domainlayer.DomainEntity{Id: sprintIdGen.Generate(connectionId, jiraSprint.SprintId)},
				Url:             jiraSprint.Self,
				Status:          strings.ToUpper(jiraSprint.State),
				Name:            jiraSprint.Name,
				StartedDate:     jiraSprint.StartDate,
				EndedDate:       jiraSprint.EndDate,
				CompletedDate:   jiraSprint.CompleteDate,
				OriginalBoardID: boardIdGen.Generate(connectionId, jiraSprint.OriginBoardID),
			}
			return []interface{}{
				sprint,
				&ticket.BoardSprint{
					BoardId:  domainBoardId,
					SprintId: sprint.Id,
				},
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
