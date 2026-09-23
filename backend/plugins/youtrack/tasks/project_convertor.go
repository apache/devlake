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
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

// RAW_PROJECTS_TABLE labels the raw-data lineage for the scope-derived
// board. Projects are added as scopes (no collector), so this is a logical
// tag only (Linear's RAW_TEAMS_TABLE convention).
const RAW_PROJECTS_TABLE = "youtrack_projects"

var ConvertProjectsMeta = plugin.SubTaskMeta{
	Name:             "Convert Projects",
	EntryPoint:       ConvertProjects,
	EnabledByDefault: true,
	Description:      "Convert the YouTrack project scope (_tool_youtrack_projects) into the domain layer table boards",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.YoutrackProject{}.TableName()},
	ProductTables:    []string{ticket.Board{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertProjects

func ConvertProjects(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*YoutrackTaskData)
	connectionId := data.Options.ConnectionId
	boardIdGen := didgen.NewDomainIdGenerator(&models.YoutrackProject{})

	cursor, err := db.Cursor(
		dal.From(&models.YoutrackProject{}),
		dal.Where("connection_id = ? AND id = ?", connectionId, data.Options.ProjectId),
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
			Table: RAW_PROJECTS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.YoutrackProject{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			project := inputRow.(*models.YoutrackProject)
			board := &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{Id: boardIdGen.Generate(connectionId, project.Id)},
				Name:         project.Name,
				Description:  project.Description,
				Url:          projectUrl(data.Connection.Endpoint, project.ShortName),
			}
			return []interface{}{board}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
