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
	"fmt"
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

var ConvertCyclesMeta = plugin.SubTaskMeta{
	Name:             "Convert Cycles",
	EntryPoint:       ConvertCycles,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_linear_cycles into domain layer tables sprints and board_sprints",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.LinearCycle{}.TableName(), RAW_CYCLES_TABLE},
	ProductTables:    []string{ticket.Sprint{}.TableName(), ticket.BoardSprint{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertCycles

func ConvertCycles(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)
	connectionId := data.Options.ConnectionId

	cycleIdGen := didgen.NewDomainIdGenerator(&models.LinearCycle{})
	boardIdGen := didgen.NewDomainIdGenerator(&models.LinearTeam{})
	boardId := boardIdGen.Generate(connectionId, data.Options.TeamId)

	cursor, err := db.Cursor(
		dal.From(&models.LinearCycle{}),
		dal.Where("connection_id = ? AND team_id = ?", connectionId, data.Options.TeamId),
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
			Table: RAW_CYCLES_TABLE,
		},
		InputRowType: reflect.TypeOf(models.LinearCycle{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			cycle := inputRow.(*models.LinearCycle)
			sprintId := cycleIdGen.Generate(connectionId, cycle.Id)
			name := cycle.Name
			if name == "" {
				name = fmt.Sprintf("Cycle %d", cycle.Number)
			}
			status := "ACTIVE"
			if cycle.CompletedAt != nil {
				status = "CLOSED"
			}
			sprint := &ticket.Sprint{
				DomainEntity:    domainlayer.DomainEntity{Id: sprintId},
				Name:            name,
				Status:          status,
				StartedDate:     cycle.StartsAt,
				EndedDate:       cycle.EndsAt,
				CompletedDate:   cycle.CompletedAt,
				OriginalBoardID: boardId,
			}
			boardSprint := &ticket.BoardSprint{
				BoardId:  boardId,
				SprintId: sprintId,
			}
			return []interface{}{sprint, boardSprint}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
