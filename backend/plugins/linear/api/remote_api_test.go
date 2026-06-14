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

package api

import (
	"encoding/json"
	"testing"

	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

func TestMapLinearTeamsToScopeEntries(t *testing.T) {
	var response linearTeamsGraphqlResponse
	body := `{"data":{"teams":{"nodes":[` +
		`{"id":"team-uuid-1","name":"Engineering","key":"ENG","description":"core eng"},` +
		`{"id":"team-uuid-2","name":"Design","key":"DSG","description":""}` +
		`],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`
	assert.NoError(t, json.Unmarshal([]byte(body), &response))

	entries := mapLinearTeamsToScopeEntries(response)
	assert.Len(t, entries, 2)

	assert.Equal(t, api.RAS_ENTRY_TYPE_SCOPE, entries[0].Type)
	assert.Nil(t, entries[0].ParentId)
	assert.Equal(t, "team-uuid-1", entries[0].Id)
	assert.Equal(t, "Engineering", entries[0].Name)
	assert.Equal(t, "Engineering", entries[0].FullName)
	// the scope payload must carry the team id used as the scope's primary key
	assert.NotNil(t, entries[0].Data)
	assert.Equal(t, "team-uuid-1", entries[0].Data.TeamId)
	assert.Equal(t, "ENG", entries[0].Data.Key)

	assert.Equal(t, "team-uuid-2", entries[1].Id)
	assert.Equal(t, "Design", entries[1].Name)
}

func TestNextPageFrom(t *testing.T) {
	var more linearTeamsGraphqlResponse
	more.Data.Teams.PageInfo.HasNextPage = true
	more.Data.Teams.PageInfo.EndCursor = "cursor-abc"
	next := nextPageFrom(more)
	assert.NotNil(t, next)
	assert.Equal(t, "cursor-abc", next.Cursor)

	// no further pages -> nil, so the helper stops paginating
	var last linearTeamsGraphqlResponse
	last.Data.Teams.PageInfo.HasNextPage = false
	last.Data.Teams.PageInfo.EndCursor = "cursor-xyz"
	assert.Nil(t, nextPageFrom(last))
}
