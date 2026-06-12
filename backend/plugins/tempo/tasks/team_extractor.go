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
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/tempo/models"
)

var ExtractTeamsMeta = plugin.SubTaskMeta{
	Name:             "extract_teams",
	EntryPoint:       ExtractTeams,
	EnabledByDefault: true,
	Description:      "Extract teams from Tempo API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

// API response for Tempo team
type TempoTeamResponse struct {
	Id      int64  `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Self    string `json:"self"`
}

func ExtractTeams(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TempoTaskData)

	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.TempoApiParams{
				ConnectionId: data.Options.ConnectionId,
			},
			Table: RAW_TEAM_TABLE,
		},
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var apiTeam TempoTeamResponse
			err := errors.Convert(json.Unmarshal(row.Data, &apiTeam))
			if err != nil {
				return nil, err
			}

			// Transform to tool layer model
			team := &models.TempoTeam{
				TeamId:  apiTeam.Id,
				Key:     apiTeam.Key,
				Name:    apiTeam.Name,
				Summary: apiTeam.Summary,
			}
			team.ConnectionId = data.Options.ConnectionId

			return []interface{}{team}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
