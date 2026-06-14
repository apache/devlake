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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/merico-ai/graphql"
)

const RAW_ACCOUNTS_TABLE = "linear_accounts"

// GraphqlQueryAccountWrapper is the paginated `users` query envelope.
type GraphqlQueryAccountWrapper struct {
	Users struct {
		Nodes    []GraphqlQueryAccount
		PageInfo *helper.GraphqlQueryPageInfo
	} `graphql:"users(first: $pageSize, after: $skipCursor)"`
}

type GraphqlQueryAccount struct {
	Id          string
	Name        string
	DisplayName string
	Email       string
	AvatarUrl   string
	Active      bool
}

var CollectAccountsMeta = plugin.SubTaskMeta{
	Name:             "Collect Users",
	EntryPoint:       CollectAccounts,
	EnabledByDefault: true,
	Description:      "Collect workspace users from the Linear GraphQL API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}

var _ plugin.SubTaskEntryPoint = CollectAccounts

func CollectAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LinearTaskData)
	collector, err := helper.NewGraphqlCollector(helper.GraphqlCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ACCOUNTS_TABLE,
		},
		GraphqlClient: data.GraphqlClient,
		PageSize:      100,
		BuildQuery: func(reqData *helper.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryAccountWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			variables := map[string]interface{}{
				"pageSize":   graphql.Int(reqData.Pager.Size),
				"skipCursor": (*graphql.String)(reqData.Pager.SkipCursor),
			}
			return query, variables, nil
		},
		GetPageInfo: func(iQuery interface{}, args *helper.GraphqlCollectorArgs) (*helper.GraphqlQueryPageInfo, error) {
			query := iQuery.(*GraphqlQueryAccountWrapper)
			return query.Users.PageInfo, nil
		},
		ResponseParser: func(queryWrapper interface{}) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryAccountWrapper)
			for _, account := range query.Users.Nodes {
				messages = append(messages, errors.Must1(json.Marshal(account)))
			}
			return
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
