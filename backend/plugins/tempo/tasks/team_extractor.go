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

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/tempo/models"
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

	// The extracted teams are the team scopes themselves, which the extractor
	// re-creates: keep the scope config users assigned to them.
	clauses := []dal.Clause{dal.Where("connection_id = ?", data.Options.ConnectionId)}
	if data.Options.TeamId != 0 {
		clauses = append(clauses, dal.Where("team_id = ?", data.Options.TeamId))
	}
	var existingTeams []models.TempoTeam
	err := taskCtx.GetDal().All(&existingTeams, clauses...)
	if err != nil {
		return err
	}
	scopeConfigIds := make(map[int64]uint64, len(existingTeams))
	for _, team := range existingTeams {
		scopeConfigIds[team.TeamId] = team.ScopeConfigId
	}

	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: teamRawDataSubTaskArgs(taskCtx, data.Options),
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			team, err := extractTeam(row.Data, data.Options, scopeConfigIds)
			if err != nil || team == nil {
				return nil, err
			}
			return []interface{}{team}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}

// extractTeam returns nil for a team other than the team of this scope, so
// that collecting one team scope doesn't add every Tempo team as a scope.
func extractTeam(data []byte, opts *TempoOptions, scopeConfigIds map[int64]uint64) (*models.TempoTeam, errors.Error) {
	var apiTeam TempoTeamResponse
	err := errors.Convert(json.Unmarshal(data, &apiTeam))
	if err != nil {
		return nil, err
	}
	if opts.TeamId != 0 && apiTeam.Id != opts.TeamId {
		return nil, nil
	}

	// Transform to tool layer model
	team := &models.TempoTeam{
		TeamId:  apiTeam.Id,
		Key:     apiTeam.Key,
		Name:    apiTeam.Name,
		Summary: apiTeam.Summary,
	}
	team.ConnectionId = opts.ConnectionId
	team.ScopeConfigId = scopeConfigIds[apiTeam.Id]
	return team, nil
}
