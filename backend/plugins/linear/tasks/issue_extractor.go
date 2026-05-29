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

var ExtractIssuesMeta = plugin.SubTaskMeta{
	Name:             "Extract Issues",
	EntryPoint:       ExtractIssues,
	EnabledByDefault: true,
	Description:      "Extract raw issue data into tool layer tables _tool_linear_issues and _tool_linear_issue_labels",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = ExtractIssues

func ExtractIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LinearTaskData)
	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ISSUES_TABLE,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			apiIssue := &GraphqlQueryIssue{}
			if err := errors.Convert(json.Unmarshal(row.Data, apiIssue)); err != nil {
				return nil, err
			}
			connectionId := data.Options.ConnectionId
			issue := &models.LinearIssue{
				ConnectionId:  connectionId,
				Id:            apiIssue.Id,
				TeamId:        data.Options.TeamId,
				Identifier:    apiIssue.Identifier,
				Number:        apiIssue.Number,
				Title:         apiIssue.Title,
				Description:   apiIssue.Description,
				Url:           apiIssue.Url,
				Priority:      apiIssue.Priority,
				PriorityLabel: PriorityLabel(apiIssue.Priority),
				Estimate:      apiIssue.Estimate,
				CreatedAt:     apiIssue.CreatedAt,
				UpdatedAt:     apiIssue.UpdatedAt,
				StartedAt:     apiIssue.StartedAt,
				CompletedAt:   apiIssue.CompletedAt,
				CanceledAt:    apiIssue.CanceledAt,
			}
			if apiIssue.State != nil {
				issue.StateId = apiIssue.State.Id
				issue.StateName = apiIssue.State.Name
				issue.StateType = apiIssue.State.Type
			}
			if apiIssue.Assignee != nil {
				issue.AssigneeId = apiIssue.Assignee.Id
			}
			if apiIssue.Creator != nil {
				issue.CreatorId = apiIssue.Creator.Id
			}
			if apiIssue.Cycle != nil {
				issue.CycleId = apiIssue.Cycle.Id
			}
			if apiIssue.Parent != nil {
				issue.ParentId = apiIssue.Parent.Id
			}

			results := make([]interface{}, 0, len(apiIssue.Labels.Nodes)+1)
			results = append(results, issue)
			for _, label := range apiIssue.Labels.Nodes {
				results = append(results, &models.LinearIssueLabel{
					ConnectionId: connectionId,
					IssueId:      apiIssue.Id,
					LabelName:    label.Name,
				})
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
