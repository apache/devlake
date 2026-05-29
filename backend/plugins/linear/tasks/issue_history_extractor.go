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
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

var ExtractIssueHistoryMeta = plugin.SubTaskMeta{
	Name:             "Extract Issue History",
	EntryPoint:       ExtractIssueHistory,
	EnabledByDefault: true,
	Description:      "Extract raw issue history into tool layer table _tool_linear_issue_history",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = ExtractIssueHistory

func ExtractIssueHistory(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LinearTaskData)
	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ISSUE_HISTORY_TABLE,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			apiEvent := &GraphqlQueryHistory{}
			if err := errors.Convert(json.Unmarshal(row.Data, apiEvent)); err != nil {
				return nil, err
			}
			issueRef := &SimpleLinearIssue{}
			if err := errors.Convert(json.Unmarshal(row.Input, issueRef)); err != nil {
				return nil, err
			}
			event := &models.LinearIssueHistory{
				ConnectionId: data.Options.ConnectionId,
				Id:           apiEvent.Id,
				IssueId:      issueRef.Id,
				CreatedAt:    apiEvent.CreatedAt,
			}
			if apiEvent.Actor != nil {
				event.ActorId = apiEvent.Actor.Id
			}
			if apiEvent.FromState != nil {
				event.FromStateId = apiEvent.FromState.Id
				event.FromStateName = apiEvent.FromState.Name
				event.FromStateType = apiEvent.FromState.Type
			}
			if apiEvent.ToState != nil {
				event.ToStateId = apiEvent.ToState.Id
				event.ToStateName = apiEvent.ToState.Name
				event.ToStateType = apiEvent.ToState.Type
			}
			return []interface{}{event}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
