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
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/taiga/models"
)

var _ plugin.SubTaskEntryPoint = ExtractIssues

var ExtractIssuesMeta = plugin.SubTaskMeta{
	Name:             "extractIssues",
	EntryPoint:       ExtractIssues,
	EnabledByDefault: true,
	Description:      "extract Taiga issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TaigaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: TaigaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_ISSUE_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var apiIssue struct {
				Id              uint64 `json:"id"`
				Ref             int    `json:"ref"`
				Subject         string `json:"subject"`
				StatusExtraInfo struct {
					Name string `json:"name"`
				} `json:"status_extra_info"`
				TypeExtraInfo *struct {
					Name string `json:"name"`
				} `json:"type_extra_info"`
				PriorityExtraInfo *struct {
					Name string `json:"name"`
				} `json:"priority_extra_info"`
				SeverityExtraInfo *struct {
					Name string `json:"name"`
				} `json:"severity_extra_info"`
				IsClosed            bool       `json:"is_closed"`
				CreatedDate         *time.Time `json:"created_date"`
				ModifiedDate        *time.Time `json:"modified_date"`
				FinishedDate        *time.Time `json:"finished_date"`
				AssignedTo          *uint64    `json:"assigned_to"`
				AssignedToExtraInfo *struct {
					FullNameDisplay string `json:"full_name_display"`
				} `json:"assigned_to_extra_info"`
				Milestone *uint64 `json:"milestone"`
			}
			err := json.Unmarshal(row.Data, &apiIssue)
			if err != nil {
				return nil, errors.Default.Wrap(err, "error unmarshalling issue")
			}

			var assignedTo uint64
			var assignedToName string
			if apiIssue.AssignedTo != nil {
				assignedTo = *apiIssue.AssignedTo
			}
			if apiIssue.AssignedToExtraInfo != nil {
				assignedToName = apiIssue.AssignedToExtraInfo.FullNameDisplay
			}
			var issueTypeName string
			if apiIssue.TypeExtraInfo != nil {
				issueTypeName = apiIssue.TypeExtraInfo.Name
			}
			var priority string
			if apiIssue.PriorityExtraInfo != nil {
				priority = apiIssue.PriorityExtraInfo.Name
			}
			var severity string
			if apiIssue.SeverityExtraInfo != nil {
				severity = apiIssue.SeverityExtraInfo.Name
			}
			var milestoneId uint64
			if apiIssue.Milestone != nil {
				milestoneId = *apiIssue.Milestone
			}

			issue := &models.TaigaIssue{
				ConnectionId:   data.Options.ConnectionId,
				ProjectId:      data.Options.ProjectId,
				IssueId:        apiIssue.Id,
				Ref:            apiIssue.Ref,
				Subject:        apiIssue.Subject,
				Status:         apiIssue.StatusExtraInfo.Name,
				IssueTypeName:  issueTypeName,
				Priority:       priority,
				Severity:       severity,
				IsClosed:       apiIssue.IsClosed,
				CreatedDate:    apiIssue.CreatedDate,
				ModifiedDate:   apiIssue.ModifiedDate,
				FinishedDate:   apiIssue.FinishedDate,
				AssignedTo:     assignedTo,
				AssignedToName: assignedToName,
				MilestoneId:    milestoneId,
			}

			return []interface{}{issue}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
