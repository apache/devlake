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

var ConvertCommentsMeta = plugin.SubTaskMeta{
	Name:             "Convert Comments",
	EntryPoint:       ConvertComments,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_linear_comments into domain layer table issue_comments",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.LinearComment{}.TableName(), RAW_COMMENTS_TABLE},
	ProductTables:    []string{ticket.IssueComment{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertComments

func ConvertComments(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)
	connectionId := data.Options.ConnectionId

	issueIdGen := didgen.NewDomainIdGenerator(&models.LinearIssue{})
	commentIdGen := didgen.NewDomainIdGenerator(&models.LinearComment{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.LinearAccount{})

	cursor, err := db.Cursor(
		dal.Select("c.*"),
		dal.From("_tool_linear_comments c"),
		dal.Join("LEFT JOIN _tool_linear_issues i ON (i.connection_id = c.connection_id AND i.id = c.issue_id)"),
		dal.Where("c.connection_id = ? AND i.team_id = ?", connectionId, data.Options.TeamId),
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
			Table: RAW_COMMENTS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.LinearComment{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			comment := inputRow.(*models.LinearComment)
			domainComment := &ticket.IssueComment{
				DomainEntity: domainlayer.DomainEntity{Id: commentIdGen.Generate(connectionId, comment.Id)},
				IssueId:      issueIdGen.Generate(connectionId, comment.IssueId),
				Body:         comment.Body,
				CreatedDate:  comment.CreatedAt,
			}
			if comment.AuthorId != "" {
				domainComment.AccountId = accountIdGen.Generate(connectionId, comment.AuthorId)
			}
			if !comment.UpdatedAt.IsZero() {
				domainComment.UpdatedDate = &comment.UpdatedAt
			}
			return []interface{}{domainComment}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
