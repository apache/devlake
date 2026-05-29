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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/merico-ai/graphql"
)

const RAW_ISSUE_HISTORY_TABLE = "linear_issue_history"

// GraphqlQueryHistoryWrapper is the per-issue, paginated `history` query.
type GraphqlQueryHistoryWrapper struct {
	Issue struct {
		History struct {
			Nodes    []GraphqlQueryHistory
			PageInfo *helper.GraphqlQueryPageInfo
		} `graphql:"history(first: $pageSize, after: $skipCursor)"`
	} `graphql:"issue(id: $issueId)"`
}

type GraphqlQueryHistory struct {
	Id        string
	CreatedAt time.Time
	Actor     *struct{ Id string }
	FromState *struct {
		Id   string
		Name string
		Type string
	}
	ToState *struct {
		Id   string
		Name string
		Type string
	}
}

var CollectIssueHistoryMeta = plugin.SubTaskMeta{
	Name:             "Collect Issue History",
	EntryPoint:       CollectIssueHistory,
	EnabledByDefault: true,
	Description:      "Collect history events for each collected Linear issue",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractIssuesMeta},
}

var _ plugin.SubTaskEntryPoint = CollectIssueHistory

func CollectIssueHistory(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)

	cursor, err := db.Cursor(
		dal.Select("id"),
		dal.From(&models.LinearIssue{}),
		dal.Where("connection_id = ? AND team_id = ?", data.Options.ConnectionId, data.Options.TeamId),
	)
	if err != nil {
		return err
	}
	iterator, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimpleLinearIssue{}))
	if err != nil {
		return err
	}

	collector, err := helper.NewGraphqlCollector(helper.GraphqlCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ISSUE_HISTORY_TABLE,
		},
		GraphqlClient: data.GraphqlClient,
		Input:         iterator,
		InputStep:     1,
		PageSize:      100,
		BuildQuery: func(reqData *helper.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryHistoryWrapper{}
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
			query := iQuery.(*GraphqlQueryHistoryWrapper)
			return query.Issue.History.PageInfo, nil
		},
		ResponseParser: func(queryWrapper interface{}) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryHistoryWrapper)
			for _, event := range query.Issue.History.Nodes {
				// Only state transitions are relevant to the status changelog.
				if event.FromState == nil && event.ToState == nil {
					continue
				}
				messages = append(messages, errors.Must1(json.Marshal(event)))
			}
			return
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
