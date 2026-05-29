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
	"encoding/json"
	"reflect"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/merico-ai/graphql"
)

const RAW_COMMENTS_TABLE = "linear_comments"

// SimpleLinearIssue is the iterator element used to drive per-issue collection
// of child resources (comments, history). Its JSON form is stored in the raw
// row's `input` column so extractors can recover the owning issue id.
type SimpleLinearIssue struct {
	Id string
}

// GraphqlQueryCommentWrapper is the per-issue, paginated `comments` query.
type GraphqlQueryCommentWrapper struct {
	Issue struct {
		Comments struct {
			Nodes    []GraphqlQueryComment
			PageInfo *helper.GraphqlQueryPageInfo
		} `graphql:"comments(first: $pageSize, after: $skipCursor)"`
	} `graphql:"issue(id: $issueId)"`
}

type GraphqlQueryComment struct {
	Id        string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
	User      *struct{ Id string }
}

var CollectCommentsMeta = plugin.SubTaskMeta{
	Name:             "Collect Comments",
	EntryPoint:       CollectComments,
	EnabledByDefault: true,
	Description:      "Collect comments for each collected Linear issue",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractIssuesMeta},
}

var _ plugin.SubTaskEntryPoint = CollectComments

func CollectComments(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)

	apiCollector, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: LinearApiParams{
			ConnectionId: data.Options.ConnectionId,
			TeamId:       data.Options.TeamId,
		},
		Table: RAW_COMMENTS_TABLE,
	})
	if err != nil {
		return err
	}

	// Only sweep issues updated since the last successful collection: an
	// unchanged issue's comments cannot have changed, so re-fetching every
	// issue each run wastes a request per issue against Linear's hourly budget.
	since := apiCollector.GetSince()
	cursor, err := db.Cursor(issuesToCollectChildrenClauses(data.Options.ConnectionId, data.Options.TeamId, since)...)
	if err != nil {
		return err
	}
	iterator, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimpleLinearIssue{}))
	if err != nil {
		return err
	}

	err = apiCollector.InitGraphQLCollector(helper.GraphqlCollectorArgs{
		GraphqlClient: data.GraphqlClient,
		Input:         iterator,
		InputStep:     1,
		PageSize:      100,
		BuildQuery: func(reqData *helper.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryCommentWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			issue := reqData.Input.(*SimpleLinearIssue)
			variables := map[string]interface{}{
				"pageSize":   graphql.Int(reqData.Pager.Size),
				"skipCursor": (*graphql.String)(reqData.Pager.SkipCursor),
				"issueId":    graphql.String(issue.Id),
			}
			return query, variables, nil
		},
		GetPageInfo: func(iQuery interface{}, args *helper.GraphqlCollectorArgs) (*helper.GraphqlQueryPageInfo, error) {
			query := iQuery.(*GraphqlQueryCommentWrapper)
			return query.Issue.Comments.PageInfo, nil
		},
		ResponseParser: func(queryWrapper interface{}) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryCommentWrapper)
			for _, comment := range query.Issue.Comments.Nodes {
				messages = append(messages, errors.Must1(json.Marshal(comment)))
			}
			return
		},
	})
	if err != nil {
		return err
	}
	return apiCollector.Execute()
}
