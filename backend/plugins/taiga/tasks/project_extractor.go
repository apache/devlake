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
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/taiga/models"
)

var _ plugin.SubTaskEntryPoint = ExtractProjects

var ExtractProjectsMeta = plugin.SubTaskMeta{
	Name:             "extractProjects",
	EntryPoint:       ExtractProjects,
	EnabledByDefault: true,
	Description:      "extract Taiga projects",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ExtractProjects(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TaigaTaskData)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: TaigaApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_PROJECT_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var apiProject struct {
				Id           uint64 `json:"id"`
				Name         string `json:"name"`
				Slug         string `json:"slug"`
				Description  string `json:"description"`
				CreatedDate  string `json:"created_date"`
				ModifiedDate string `json:"modified_date"`
			}
			err := json.Unmarshal(row.Data, &apiProject)
			if err != nil {
				return nil, errors.Default.Wrap(err, "error unmarshalling project")
			}

			project := &models.TaigaProject{
				Scope: common.Scope{
					ConnectionId: data.Options.ConnectionId,
				},
				ProjectId:   apiProject.Id,
				Name:        apiProject.Name,
				Slug:        apiProject.Slug,
				Description: apiProject.Description,
			}

			return []interface{}{project}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
