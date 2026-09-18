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
	"strings"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/github/models"
	"github.com/apache/devlake/plugins/github/tasks"
	"github.com/merico-ai/graphql"
)

const RAW_PRS_TABLE = "github_graphql_prs"

// GraphqlQueryPrWrapper is a wrapper for collecting new PRs since the previous collection
type GraphqlQueryPrWrapper struct {
	RateLimit struct {
		Cost int
	}
	Repository struct {
		PullRequests struct {
			PageInfo   *api.GraphqlQueryPageInfo
			Prs        []GraphqlQueryPr `graphql:"nodes"`
			TotalCount graphql.Int
		} `graphql:"pullRequests(first: $pageSize, after: $skipCursor, orderBy: {field: CREATED_AT, direction: DESC})"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

// GraphqlQueryPrDetailWrapper is a wrapper for refetching OPEN PRs from the database to update the details
type GraphqlQueryPrDetailWrapper struct {
	RateLimit struct {
		Cost int
	}
	Repository struct {
		PullRequests []GraphqlQueryPr `graphql:"pullRequest(number: $number)" graphql-extend:"true"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

type GraphqlQueryPr struct {
	DatabaseId int
	Number     int
	State      string
	Title      string
	IsDraft    bool
	Body       string
	Url        string
	Labels     struct {
		Nodes []struct {
			Id   string
			Name string
		}
	} `graphql:"labels(first: 100)"`
	Author    *GraphqlInlineActorQuery
	Assignees struct {
		// FIXME now domain layer just support one assignee
		Assignees []GraphqlInlineAccountQuery `graphql:"nodes"`
	} `graphql:"assignees(first: 1)"`
	ClosedAt    *time.Time
	MergedAt    *time.Time
	UpdatedAt   time.Time
	CreatedAt   time.Time
	MergeCommit *struct {
		Oid string
	}
	HeadRefName string
	HeadRefOid  string
	BaseRefName string
	BaseRefOid  string
	Commits     struct {
		PageInfo   *api.GraphqlQueryPageInfo
		Nodes      []GraphqlQueryCommit `graphql:"nodes"`
		TotalCount graphql.Int
	} `graphql:"commits(first: 100)"`
	Reviews struct {
		TotalCount graphql.Int
		Nodes      []GraphqlQueryReview `graphql:"nodes"`
	} `graphql:"reviews(first: 100)"`
	Additions      int
	Deletions      int
	MergedBy       *GraphqlInlineActorQuery
	ReviewRequests struct {
		Nodes []ReviewRequestNode `graphql:"nodes"`
	} `graphql:"reviewRequests(first: 10)"`
}

type ReviewRequestNode struct {
	RequestedReviewer RequestedReviewer `graphql:"requestedReviewer"`
}

type RequestedReviewer struct {
	User User `graphql:"... on User"`
	Team Team `graphql:"... on Team"`
}

type User struct {
	Id    int    `graphql:"databaseId"`
	Login string `graphql:"login"`
	Name  string `graphql:"name"`
}

type Team struct {
	Id   int    `graphql:"databaseId"`
	Name string `graphql:"name"`
	Slug string `graphql:"slug"`
}

type GraphqlQueryReview struct {
	Body       string
	Author     *GraphqlInlineActorQuery
	State      string `json:"state"`
	DatabaseId int    `json:"databaseId"`
	Commit     struct {
		Oid string
	}
	SubmittedAt *time.Time `json:"submittedAt"`
}

type GraphqlQueryCommit struct {
	Commit struct {
		Oid     string
		Message string
		Author  struct {
			Name  string
			Email string
			Date  time.Time
			User  *GraphqlInlineAccountQuery
		}
		Committer struct {
			Date  time.Time
			Email string
			Name  string
		}
	}
	Url string
}

var CollectPrsMeta = plugin.SubTaskMeta{
	Name:             "Collect Pull Requests",
	EntryPoint:       CollectPrs,
	EnabledByDefault: true,
	Description:      "Collect Pr data from GithubGraphql api, supports both timeFilter and diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
}

var _ plugin.SubTaskEntryPoint = CollectPrs

func CollectPrs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*tasks.GithubTaskData)
	var err errors.Error
	apiCollector, err := api.NewStatefulApiCollector(api.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: tasks.GithubApiParams{
			ConnectionId: data.Options.ConnectionId,
			Name:         data.Options.Name,
		},
		Table: RAW_PRS_TABLE,
	})
	if err != nil {
		return err
	}

	// collect new PRs since the previous run
	since := apiCollector.GetSince()
	err = apiCollector.InitGraphQLCollector(api.GraphqlCollectorArgs{
		GraphqlClient: data.GraphqlClient,
		PageSize:      10,
		/*
			(Optional) Return query string for request, or you can plug them into UrlTemplate directly
		*/
		BuildQuery: func(reqData *api.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryPrWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			ownerName := strings.Split(data.Options.Name, "/")
			variables := map[string]interface{}{
				"pageSize":   graphql.Int(reqData.Pager.Size),
				"skipCursor": (*graphql.String)(reqData.Pager.SkipCursor),
				"owner":      graphql.String(ownerName[0]),
				"name":       graphql.String(ownerName[1]),
			}
			return query, variables, nil
		},
		GetPageInfo: func(iQuery interface{}, args *api.GraphqlCollectorArgs) (*api.GraphqlQueryPageInfo, error) {
			query := iQuery.(*GraphqlQueryPrWrapper)
			return query.Repository.PullRequests.PageInfo, nil
		},
		ResponseParser: func(queryWrapper any) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryPrWrapper)
			prs := query.Repository.PullRequests.Prs
			for _, rawL := range prs {
				if since != nil && since.After(rawL.UpdatedAt) {
					return messages, api.ErrFinishCollect
				}
				messages = append(messages, errors.Must1(json.Marshal(rawL)))
			}
			return
		},
	})
	if err != nil {
		return err
	}

	// refetch(refresh) for existing PRs in the database that are still OPEN
	db := taskCtx.GetDal()
	cursor, err := db.Cursor(
		dal.From(models.GithubPullRequest{}.TableName()),
		dal.Where("state = ? AND repo_id = ? AND connection_id=?", "OPEN", data.Options.GithubId, data.Options.ConnectionId),
		dal.Orderby("github_id ASC"),
	)
	if err != nil {
		return err
	}
	iterator, err := api.NewDalCursorIterator(db, cursor, reflect.TypeOf(models.GithubPullRequest{}))
	if err != nil {
		return err
	}
	prUpdatedAt := make(map[int]time.Time)
	err = apiCollector.InitGraphQLCollector(api.GraphqlCollectorArgs{
		GraphqlClient: data.GraphqlClient,
		Input:         iterator,
		InputStep:     100,
		Incremental:   true,
		BuildQuery: func(reqData *api.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryPrDetailWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			ownerName := strings.Split(data.Options.Name, "/")
			inputPrs := reqData.Input.([]interface{})
			outputPrs := []map[string]interface{}{}
			for _, i := range inputPrs {
				inputPr := i.(*models.GithubPullRequest)
				outputPrs = append(outputPrs, map[string]interface{}{
					`number`: graphql.Int(inputPr.Number),
				})
				prUpdatedAt[inputPr.Number] = inputPr.GithubUpdatedAt
			}
			variables := map[string]interface{}{
				"pullRequest": outputPrs,
				"owner":       graphql.String(ownerName[0]),
				"name":        graphql.String(ownerName[1]),
			}
			return query, variables, nil
		},
		ResponseParser: func(queryWrapper any) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryPrDetailWrapper)
			prs := query.Repository.PullRequests
			for _, rawL := range prs {
				if rawL.UpdatedAt.After(prUpdatedAt[rawL.Number]) {
					messages = append(messages, errors.Must1(json.Marshal(rawL)))
				}
			}
			return
		},
	})
	if err != nil {
		return err
	}

	return apiCollector.Execute()
}
