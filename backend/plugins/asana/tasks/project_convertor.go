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
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

var _ plugin.SubTaskEntryPoint = ConvertProject

var ConvertProjectMeta = plugin.SubTaskMeta{
	Name:             "ConvertProject",
	EntryPoint:       ConvertProject,
	EnabledByDefault: true,
	Description:      "Convert tool layer Asana projects into domain layer boards",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertProject(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, rawProjectTable)
	db := taskCtx.GetDal()
	connectionId := data.Options.ConnectionId
	projectId := data.Options.ProjectId

	clauses := []dal.Clause{
		dal.From(&models.AsanaProject{}),
		dal.Where("connection_id = ? AND gid = ?", connectionId, projectId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	boardIdGen := didgen.NewDomainIdGenerator(&models.AsanaProject{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.AsanaProject{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			toolProject := inputRow.(*models.AsanaProject)
			domainBoard := &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{Id: boardIdGen.Generate(toolProject.ConnectionId, toolProject.Gid)},
				Name:         toolProject.Name,
				Url:          toolProject.PermalinkUrl,
				Type:         "asana",
			}
			return []interface{}{domainBoard}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
