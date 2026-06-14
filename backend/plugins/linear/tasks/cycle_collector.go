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
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/merico-ai/graphql"
)

const RAW_CYCLES_TABLE = "linear_cycles"

// GraphqlQueryCycleWrapper is the team-scoped, paginated `cycles` query.
type GraphqlQueryCycleWrapper struct {
	Team struct {
		Cycles struct {
			Nodes    []GraphqlQueryCycle
			PageInfo *helper.GraphqlQueryPageInfo
		} `graphql:"cycles(first: $pageSize, after: $skipCursor)"`
	} `graphql:"team(id: $teamId)"`
}

type GraphqlQueryCycle struct {
	Id          string
	Number      int
	Name        string
	StartsAt    *time.Time
	EndsAt      *time.Time
	CompletedAt *time.Time
}

var CollectCyclesMeta = plugin.SubTaskMeta{
	Name:             "Collect Cycles",
	EntryPoint:       CollectCycles,
	EnabledByDefault: true,
	Description:      "Collect cycles (sprints) for a Linear team",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = CollectCycles

func CollectCycles(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LinearTaskData)
	collector, err := helper.NewGraphqlCollector(helper.GraphqlCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_CYCLES_TABLE,
		},
		GraphqlClient: data.GraphqlClient,
		PageSize:      100,
		BuildQuery: func(reqData *helper.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryCycleWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			variables := map[string]interface{}{
				"pageSize":   graphql.Int(reqData.Pager.Size),
				"skipCursor": (*graphql.String)(reqData.Pager.SkipCursor),
				"teamId":     graphql.String(data.Options.TeamId),
			}
			return query, variables, nil
		},
		GetPageInfo: func(iQuery interface{}, args *helper.GraphqlCollectorArgs) (*helper.GraphqlQueryPageInfo, error) {
			query := iQuery.(*GraphqlQueryCycleWrapper)
			return query.Team.Cycles.PageInfo, nil
		},
		ResponseParser: func(queryWrapper interface{}) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryCycleWrapper)
			for _, cycle := range query.Team.Cycles.Nodes {
				messages = append(messages, errors.Must1(json.Marshal(cycle)))
			}
			return
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
