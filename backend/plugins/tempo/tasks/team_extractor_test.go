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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTeam(t *testing.T) {
	raw17 := []byte(`{"self":"https://api.tempo.io/4/teams/17","id":17,"name":"Abstract BE Team","summary":"","key":""}`)
	raw71 := []byte(`{"self":"https://api.tempo.io/4/teams/71","id":71,"name":"SmartShopper (RTL+WHS)","summary":"Microservice Applications","key":""}`)
	scopeConfigIds := map[int64]uint64{71: 3}

	t.Run("keeps only the team of the scope", func(t *testing.T) {
		team, err := extractTeam(raw17, &TempoOptions{ConnectionId: 1, TeamId: 71}, scopeConfigIds)
		assert.Nil(t, err)
		assert.Nil(t, team)
	})

	t.Run("keeps the scope config of the team scope", func(t *testing.T) {
		team, err := extractTeam(raw71, &TempoOptions{ConnectionId: 1, TeamId: 71}, scopeConfigIds)
		assert.Nil(t, err)
		if assert.NotNil(t, team) {
			assert.Equal(t, uint64(1), team.ConnectionId)
			assert.Equal(t, int64(71), team.TeamId)
			assert.Equal(t, "SmartShopper (RTL+WHS)", team.Name)
			assert.Equal(t, "Microservice Applications", team.Summary)
			assert.Equal(t, uint64(3), team.ScopeConfigId)
		}
	})

	t.Run("without a team scope, extracts every team", func(t *testing.T) {
		team, err := extractTeam(raw17, &TempoOptions{ConnectionId: 1}, scopeConfigIds)
		assert.Nil(t, err)
		if assert.NotNil(t, team) {
			assert.Equal(t, int64(17), team.TeamId)
			assert.Equal(t, uint64(0), team.ScopeConfigId)
		}
	})
}
