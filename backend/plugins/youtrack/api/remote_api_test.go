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
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/stretchr/testify/assert"
)

func TestMapYoutrackProjectsToScopeEntries(t *testing.T) {
	projects := []youtrackProject{
		{Id: "0-12", ShortName: "PROJ2", Name: "Project 2", Description: "ops", Archived: false},
		{Id: "0-7", ShortName: "PROJ1", Name: "Project 1", Archived: true},
	}

	entries := mapYoutrackProjectsToScopeEntries(projects)
	assert.Len(t, entries, 2)

	// every entry is a selectable leaf scope keyed by the internal project id
	assert.Equal(t, api.RAS_ENTRY_TYPE_SCOPE, entries[0].Type)
	assert.Nil(t, entries[0].ParentId)
	assert.Equal(t, "0-12", entries[0].Id)

	// the picker displays `shortName (name)` so users find projects by the
	// key they know from issue ids (PROJ-123)
	assert.Equal(t, "PROJ2 (Project 2)", entries[0].Name)
	assert.Equal(t, "PROJ2 (Project 2)", entries[0].FullName)

	// the scope payload carries the internal id used as the scope's primary
	// key, plus shortName for display and the project:-query
	assert.NotNil(t, entries[0].Data)
	assert.Equal(t, "0-12", entries[0].Data.Id)
	assert.Equal(t, "PROJ2", entries[0].Data.ShortName)
	assert.Equal(t, "Project 2", entries[0].Data.Name)
	assert.Equal(t, "ops", entries[0].Data.Description)
	assert.False(t, entries[0].Data.Archived)

	// archived projects are de-emphasized in the display name, and the flag
	// rides along in the payload so the UI can style them
	assert.Equal(t, "PROJ1 (Project 1) [archived]", entries[1].Name)
	assert.True(t, entries[1].Data.Archived)
}

func TestNextPageFromYoutrack(t *testing.T) {
	// a full page means there may be more projects behind it
	next := nextPageFromYoutrack(YoutrackRemotePagination{Skip: 0, Top: 100}, 100)
	assert.NotNil(t, next)
	assert.Equal(t, 100, next.Skip)
	assert.Equal(t, 100, next.Top)

	// a short page is the last one
	assert.Nil(t, nextPageFromYoutrack(YoutrackRemotePagination{Skip: 0, Top: 100}, 37))
	assert.Nil(t, nextPageFromYoutrack(YoutrackRemotePagination{Skip: 100, Top: 100}, 0))
}

func TestListYoutrackRemoteScopesWithFakeServer(t *testing.T) {
	setupTestGlobals()
	fake := newFakeYoutrack(t)
	var gotQuery url.Values
	var gotAuth string
	fake.mux.HandleFunc("/api/admin/projects", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "0-12", "shortName": "PROJ2", "name": "Project 2", "archived": false, "$type": "Project"},
			{"id": "0-7", "shortName": "PROJ1", "name": "Project 1", "description": "the code", "archived": true, "$type": "Project"},
		}))
	})

	connection := &models.YoutrackConnection{}
	connection.Endpoint = fake.endpoint()
	connection.Token = "perm:am9obi5kb2U"
	apiClient, err := api.NewApiClientFromConnection(context.TODO(), basicRes, connection)
	assert.Nil(t, err)

	children, nextPage, err := listYoutrackRemoteScopes(connection, apiClient, "", YoutrackRemotePagination{})
	assert.Nil(t, err)

	// the picker query requests exactly the scope columns and paginates
	// with YouTrack's $top/$skip parameters
	assert.Equal(t, "id,shortName,name,description,archived", gotQuery.Get("fields"))
	assert.Equal(t, "100", gotQuery.Get("$top"))
	assert.Equal(t, "0", gotQuery.Get("$skip"))
	assert.Equal(t, "Bearer perm:am9obi5kb2U", gotAuth)

	// the page maps to selectable scopes in shortName (name) display form
	assert.Len(t, children, 2)
	assert.Equal(t, "0-12", children[0].Id)
	assert.Equal(t, "PROJ2 (Project 2)", children[0].Name)
	assert.Equal(t, "PROJ1 (Project 1) [archived]", children[1].Name)
	// a short page terminates pagination
	assert.Nil(t, nextPage)
}
