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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apache/devlake/plugins/datadog/models/raw"
)

const incidentsPageWithUsers = `{
	"data": [
		{"id": "inc-1", "type": "incidents", "attributes": {"public_id": 1},
		 "relationships": {"commander_user": {"data": {"id": "user-1", "type": "users"}}}},
		{"id": "inc-2", "type": "incidents", "attributes": {"public_id": 2},
		 "relationships": {"commander_user": {"data": null}}}
	],
	"included": [
		{"id": "user-1", "type": "users", "attributes": {"name": "Ada Lovelace", "email": "ada@example.com", "handle": "ada@example.com"}},
		{"id": "type-1", "type": "incident_types", "attributes": {"name": "General Incident"}}
	],
	"meta": {"pagination": {"offset": 0, "size": 2, "next_offset": 2}}
}`

func TestParseIncidentsPage_AttachesCommanderFromIncluded(t *testing.T) {
	res := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(incidentsPageWithUsers))}

	data, next, err := parseIncidentsPage(res)
	require.NoError(t, err)
	require.Len(t, data, 2)
	require.NotNil(t, next)
	assert.Equal(t, 2, *next)

	var withCommander raw.Incident
	require.NoError(t, json.Unmarshal(data[0], &withCommander))
	require.NotNil(t, withCommander.Relationships)
	commander := withCommander.Relationships.CommanderUser.Data
	require.NotNil(t, commander)
	assert.Equal(t, "user-1", commander.Id)
	require.NotNil(t, commander.Attributes)
	assert.Equal(t, "Ada Lovelace", commander.Attributes.Name)

	var withoutCommander raw.Incident
	require.NoError(t, json.Unmarshal(data[1], &withoutCommander))
	assert.Nil(t, withoutCommander.Relationships.CommanderUser.Data)
}

func TestBuildIncidentsQuery_IncludesUsers(t *testing.T) {
	query := buildIncidentsQuery(100, 200)
	assert.Equal(t, "users", query.Get("include"))
	assert.Equal(t, "200", query.Get("page[offset]"))
}
