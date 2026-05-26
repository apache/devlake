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

var _ plugin.SubTaskEntryPoint = ConvertStory

var ConvertStoryMeta = plugin.SubTaskMeta{
	Name:             "ConvertStory",
	EntryPoint:       ConvertStory,
	EnabledByDefault: true,
	Description:      "Convert tool layer Asana stories into domain layer issue comments",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertStory(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, rawStoryTable)
	db := taskCtx.GetDal()
	connectionId := data.Options.ConnectionId
	projectId := data.Options.ProjectId

	// Only convert comment-type stories (not system-generated ones)
	clauses := []dal.Clause{
		dal.From(&models.AsanaStory{}),
		dal.Join("LEFT JOIN _tool_asana_tasks ON _tool_asana_stories.task_gid = _tool_asana_tasks.gid AND _tool_asana_stories.connection_id = _tool_asana_tasks.connection_id"),
		dal.Where("_tool_asana_stories.connection_id = ? AND _tool_asana_tasks.project_gid = ? AND _tool_asana_stories.resource_subtype = ?",
			connectionId, projectId, "comment_added"),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	commentIdGen := didgen.NewDomainIdGenerator(&models.AsanaStory{})
	taskIdGen := didgen.NewDomainIdGenerator(&models.AsanaTask{})
	userIdGen := didgen.NewDomainIdGenerator(&models.AsanaUser{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.AsanaStory{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			toolStory := inputRow.(*models.AsanaStory)
			domainComment := &ticket.IssueComment{
				DomainEntity: domainlayer.DomainEntity{Id: commentIdGen.Generate(toolStory.ConnectionId, toolStory.Gid)},
				IssueId:      taskIdGen.Generate(toolStory.ConnectionId, toolStory.TaskGid),
				Body:         toolStory.Text,
				CreatedDate:  toolStory.CreatedAt,
			}
			if toolStory.CreatedByGid != "" {
				domainComment.AccountId = userIdGen.Generate(toolStory.ConnectionId, toolStory.CreatedByGid)
			}
			return []interface{}{domainComment}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
