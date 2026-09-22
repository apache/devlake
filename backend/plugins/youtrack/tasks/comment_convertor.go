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

var ConvertCommentsMeta = plugin.SubTaskMeta{
	Name:             "Convert Comments",
	EntryPoint:       ConvertComments,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_youtrack_issue_comments into domain layer table issue_comments (skips deleted)",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.YoutrackIssueComment{}.TableName(), models.YoutrackIssue{}.TableName(), RAW_ISSUE_COMMENTS_TABLE},
	ProductTables:    []string{ticket.IssueComment{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertComments

func ConvertComments(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*YoutrackTaskData)
	connectionId := data.Options.ConnectionId

	commentIdGen := didgen.NewDomainIdGenerator(&models.YoutrackIssueComment{})
	issueIdGen := didgen.NewDomainIdGenerator(&models.YoutrackIssue{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.YoutrackAccount{})

	// scope the comments to this project via their parent issue
	cursor, err := db.Cursor(
		dal.Select("c.*"),
		dal.From("_tool_youtrack_issue_comments c"),
		dal.Join("LEFT JOIN _tool_youtrack_issues i ON (i.connection_id = c.connection_id AND i.id = c.issue_id)"),
		dal.Where("c.connection_id = ? AND i.project_id = ?", connectionId, data.Options.ProjectId),
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
			Table: RAW_ISSUE_COMMENTS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.YoutrackIssueComment{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			comment := inputRow.(*models.YoutrackIssueComment)
			// deleted comments stay in the tool layer but never reach the
			// domain layer, so comment metrics reflect reality
			if comment.Deleted {
				return nil, nil
			}
			domainComment := &ticket.IssueComment{
				DomainEntity: domainlayer.DomainEntity{Id: commentIdGen.Generate(connectionId, comment.Id)},
				IssueId:      issueIdGen.Generate(connectionId, comment.IssueId),
				Body:         comment.Body,
				CreatedDate:  comment.Created,
				UpdatedDate:  comment.Updated,
			}
			if comment.AuthorId != "" {
				domainComment.AccountId = accountIdGen.Generate(connectionId, comment.AuthorId)
			}
			return []interface{}{domainComment}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
