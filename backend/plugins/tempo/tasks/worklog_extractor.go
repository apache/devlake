/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License. You may obtain a copy of the License at

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
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/tempo/models"
)

var ExtractWorklogsMeta = plugin.SubTaskMeta{
	Name:             "extract_worklogs",
	EntryPoint:       ExtractWorklogs,
	EnabledByDefault: true,
	Description:      "Extract worklogs from Tempo API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type TempoWorklogAuthor struct {
	AccountId string `json:"accountId"`
}

type TempoWorklogResponse struct {
	TempoWorklogId int64 `json:"tempoWorklogId"`
	Issue          struct {
		Id int64 `json:"id"`
	} `json:"issue"`
	TimeSpentSeconds int                `json:"timeSpentSeconds"`
	BillableSeconds  int                `json:"billableSeconds"`
	StartDate        string             `json:"startDate"`
	StartTime        string             `json:"startTime"`
	Description      string             `json:"description"`
	Author           TempoWorklogAuthor `json:"author"`
	CreatedAt        string             `json:"createdAt"`
	UpdatedAt        string             `json:"updatedAt"`
}

func ExtractWorklogs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TempoTaskData)

	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.TempoApiParams{
				ConnectionId: data.Options.ConnectionId,
			},
			Table: RAW_WORKLOG_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var apiWorklog TempoWorklogResponse
			err := errors.Convert(json.Unmarshal(row.Data, &apiWorklog))
			if err != nil {
				return nil, err
			}

			worklog := &models.TempoWorklog{
				ConnectionId:     data.Options.ConnectionId,
				TempoWorklogId:   apiWorklog.TempoWorklogId,
				IssueId:          apiWorklog.Issue.Id,
				TimeSpentSeconds: apiWorklog.TimeSpentSeconds,
				BillableSeconds:  apiWorklog.BillableSeconds,
				StartDate:        apiWorklog.StartDate,
				StartTime:        apiWorklog.StartTime,
				Description:      apiWorklog.Description,
				AuthorAccountId:  apiWorklog.Author.AccountId,
				CreatedAt:        apiWorklog.CreatedAt,
				UpdatedAt:        apiWorklog.UpdatedAt,
			}

			return []interface{}{worklog}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
