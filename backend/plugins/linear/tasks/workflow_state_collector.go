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

const RAW_WORKFLOW_STATES_TABLE = "linear_workflow_states"

// GraphqlQueryWorkflowStateWrapper is the team-scoped paginated `states` query.
type GraphqlQueryWorkflowStateWrapper struct {
	Team struct {
		States struct {
			Nodes    []GraphqlQueryWorkflowState
			PageInfo *helper.GraphqlQueryPageInfo
		} `graphql:"states(first: $pageSize, after: $skipCursor)"`
	} `graphql:"team(id: $teamId)"`
}

type GraphqlQueryWorkflowState struct {
	Id       string
	Name     string
	Type     string
	Color    string
	Position float64
}

var CollectWorkflowStatesMeta = plugin.SubTaskMeta{
	Name:             "Collect Workflow States",
	EntryPoint:       CollectWorkflowStates,
	EnabledByDefault: true,
	Description:      "Collect workflow states for a Linear team",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = CollectWorkflowStates

func CollectWorkflowStates(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LinearTaskData)
	collector, err := helper.NewGraphqlCollector(helper.GraphqlCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_WORKFLOW_STATES_TABLE,
		},
		GraphqlClient: data.GraphqlClient,
		PageSize:      100,
		BuildQuery: func(reqData *helper.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryWorkflowStateWrapper{}
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
			query := iQuery.(*GraphqlQueryWorkflowStateWrapper)
			return query.Team.States.PageInfo, nil
		},
		ResponseParser: func(queryWrapper interface{}) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryWorkflowStateWrapper)
			for _, state := range query.Team.States.Nodes {
				messages = append(messages, errors.Must1(json.Marshal(state)))
			}
			return
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
