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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/utils"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/merico-ai/graphql"
)

const RAW_ISSUES_TABLE = "linear_issues"

// GraphqlQueryIssueWrapper is the team-scoped, paginated `issues` query.
// Incremental runs filter server-side on updatedAt ($filter) rather than
// relying on result ordering, so collection no longer depends on an undocumented
// default sort direction.
type GraphqlQueryIssueWrapper struct {
	Team struct {
		Issues struct {
			Nodes    []GraphqlQueryIssue `graphql:"nodes"`
			PageInfo *helper.GraphqlQueryPageInfo
		} `graphql:"issues(first: $pageSize, after: $skipCursor, orderBy: updatedAt, filter: $filter)"`
	} `graphql:"team(id: $teamId)"`
}

// IssueFilter mirrors the subset of Linear's GraphQL IssueFilter input used to
// restrict collection to issues updated after a point in time. The Go type
// name is significant: the GraphQL client emits it as the variable's type
// ($filter:IssueFilter!).
type IssueFilter struct {
	UpdatedAt *DateComparator `json:"updatedAt,omitempty"`
}

// DateComparator mirrors Linear's DateComparator input (only the `gt` operator
// is needed here).
type DateComparator struct {
	Gt *time.Time `json:"gt,omitempty"`
}

// buildIssueFilter returns an IssueFilter restricting to issues updated after
// `since`. When `since` is nil (a full sync) it returns the empty filter, which
// Linear treats as "match all".
func buildIssueFilter(since *time.Time) IssueFilter {
	if since == nil {
		return IssueFilter{}
	}
	return IssueFilter{UpdatedAt: &DateComparator{Gt: since}}
}

type GraphqlQueryIssue struct {
	Id          string
	Identifier  string
	Number      int
	Title       string
	Description string
	Url         string
	Priority    int
	Estimate    *float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	CanceledAt  *time.Time
	State       *struct {
		Id   string
		Name string
		Type string
	}
	Assignee *struct{ Id string }
	Creator  *struct{ Id string }
	Cycle    *struct{ Id string }
	Parent   *struct{ Id string }
	Labels   struct {
		Nodes []struct {
			Id   string
			Name string
		}
	} `graphql:"labels(first: 50)"`
}

var CollectIssuesMeta = plugin.SubTaskMeta{
	Name:             "Collect Issues",
	EntryPoint:       CollectIssues,
	EnabledByDefault: true,
	Description:      "Collect issues for a Linear team, supports incremental collection",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = CollectIssues

func CollectIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LinearTaskData)
	apiCollector, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: LinearApiParams{
			ConnectionId: data.Options.ConnectionId,
			TeamId:       data.Options.TeamId,
		},
		Table: RAW_ISSUES_TABLE,
	})
	if err != nil {
		return err
	}

	since := apiCollector.GetSince()
	err = apiCollector.InitGraphQLCollector(helper.GraphqlCollectorArgs{
		GraphqlClient: data.GraphqlClient,
		PageSize:      100,
		BuildQuery: func(reqData *helper.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryIssueWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			variables := map[string]interface{}{
				"pageSize":   graphql.Int(reqData.Pager.Size),
				"skipCursor": (*graphql.String)(reqData.Pager.SkipCursor),
				"teamId":     graphql.String(data.Options.TeamId),
				"filter":     buildIssueFilter(since),
			}
			return query, variables, nil
		},
		GetPageInfo: func(iQuery interface{}, args *helper.GraphqlCollectorArgs) (*helper.GraphqlQueryPageInfo, error) {
			query := iQuery.(*GraphqlQueryIssueWrapper)
			return query.Team.Issues.PageInfo, nil
		},
		ResponseParser: func(queryWrapper interface{}) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryIssueWrapper)
			// The server-side $filter already restricts to issues updated after
			// `since`, so every returned issue is in scope -- no client-side
			// early-stop (and thus no dependency on sort direction) is needed.
			for _, issue := range query.Team.Issues.Nodes {
				issue.CompletedAt = utils.NilIfZeroTime(issue.CompletedAt)
				issue.CanceledAt = utils.NilIfZeroTime(issue.CanceledAt)
				issue.StartedAt = utils.NilIfZeroTime(issue.StartedAt)
				messages = append(messages, errors.Must1(json.Marshal(issue)))
			}
			return
		},
	})
	if err != nil {
		return err
	}
	return apiCollector.Execute()
}
