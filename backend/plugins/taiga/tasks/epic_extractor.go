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

var _ plugin.SubTaskEntryPoint = ExtractEpics

var ExtractEpicsMeta = plugin.SubTaskMeta{
	Name:             "extractEpics",
	EntryPoint:       ExtractEpics,
	EnabledByDefault: true,
	Description:      "extract Taiga epics",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractEpics(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TaigaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: TaigaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_EPIC_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var apiEpic struct {
				Id              uint64 `json:"id"`
				Ref             int    `json:"ref"`
				Subject         string `json:"subject"`
				StatusExtraInfo struct {
					Name string `json:"name"`
				} `json:"status_extra_info"`
				IsClosed            bool       `json:"is_closed"`
				CreatedDate         *time.Time `json:"created_date"`
				ModifiedDate        *time.Time `json:"modified_date"`
				AssignedTo          *uint64    `json:"assigned_to"`
				AssignedToExtraInfo *struct {
					FullNameDisplay string `json:"full_name_display"`
				} `json:"assigned_to_extra_info"`
				Color string `json:"color"`
			}
			err := json.Unmarshal(row.Data, &apiEpic)
			if err != nil {
				return nil, errors.Default.Wrap(err, "error unmarshalling epic")
			}

			var assignedTo uint64
			var assignedToName string
			if apiEpic.AssignedTo != nil {
				assignedTo = *apiEpic.AssignedTo
			}
			if apiEpic.AssignedToExtraInfo != nil {
				assignedToName = apiEpic.AssignedToExtraInfo.FullNameDisplay
			}

			epic := &models.TaigaEpic{
				ConnectionId:   data.Options.ConnectionId,
				ProjectId:      data.Options.ProjectId,
				EpicId:         apiEpic.Id,
				Ref:            apiEpic.Ref,
				Subject:        apiEpic.Subject,
				Status:         apiEpic.StatusExtraInfo.Name,
				IsClosed:       apiEpic.IsClosed,
				CreatedDate:    apiEpic.CreatedDate,
				ModifiedDate:   apiEpic.ModifiedDate,
				AssignedTo:     assignedTo,
				AssignedToName: assignedToName,
				Color:          apiEpic.Color,
			}

			return []interface{}{epic}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
