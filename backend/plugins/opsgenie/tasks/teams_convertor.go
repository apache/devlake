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
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/opsgenie/models"
)

var ConvertTeamsMeta = plugin.SubTaskMeta{
	Name:             "convertTeams",
	EntryPoint:       ConvertTeams,
	EnabledByDefault: true,
	Description:      "convert Jira teams",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}

func ConvertTeams(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*OpsgenieTaskData)
	connectionId := data.Options.ConnectionId
	db := taskCtx.GetDal()
	clauses := []dal.Clause{
		dal.Select("teams.*"),
		dal.From("_tool_opsgenie_teams teams"),
		dal.Where("teams.connection_id = ?", connectionId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	accountIdGen := didgen.NewDomainIdGenerator(&models.Team{})
	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_TEAMS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.Team{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			team := inputRow.(*models.Team)
			u := &crossdomain.Team{
				DomainEntity: domainlayer.DomainEntity{
					Id: accountIdGen.Generate(connectionId, team.Id),
				},
				Name: team.Name,
			}
			return []interface{}{u}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}
