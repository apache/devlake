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

// RAW_TEAMS_TABLE labels the raw-data lineage for the team-scope-derived board.
// Teams are added as scopes (no collector), so this is a logical tag only.
const RAW_TEAMS_TABLE = "linear_teams"

var ConvertTeamsMeta = plugin.SubTaskMeta{
	Name:             "Convert Teams",
	EntryPoint:       ConvertTeams,
	EnabledByDefault: true,
	Description:      "Convert the Linear team scope (_tool_linear_teams) into the domain layer table boards",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.LinearTeam{}.TableName()},
	ProductTables:    []string{ticket.Board{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertTeams

func ConvertTeams(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)
	connectionId := data.Options.ConnectionId

	// boardId must be generated identically to the issue/sprint convertors so the
	// board joins to the board_issues/sprint_issues that reference it.
	boardIdGen := didgen.NewDomainIdGenerator(&models.LinearTeam{})

	cursor, err := db.Cursor(
		dal.From(&models.LinearTeam{}),
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
			Table: RAW_TEAMS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.LinearTeam{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			team := inputRow.(*models.LinearTeam)
			board := &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{Id: boardIdGen.Generate(connectionId, team.TeamId)},
				Name:         team.Name,
				Description:  team.Description,
				Type:         "linear",
			}
			return []interface{}{board}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
