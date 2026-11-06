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
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/tempo/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTempoClient answers every GET with the same body, or with the body of
// the requested offset (0 by default) when pages is set, and records the
// queries.
type fakeTempoClient struct {
	body    string
	pages   map[string]string
	query   url.Values
	queries []url.Values
}

func (c *fakeTempoClient) Get(_ string, query url.Values, _ http.Header) (*http.Response, errors.Error) {
	c.query = query
	c.queries = append(c.queries, query)
	body := c.body
	if c.pages != nil {
		offset := query.Get("offset")
		if offset == "" {
			offset = "0"
		}
		body = c.pages[offset]
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func (c *fakeTempoClient) Post(string, url.Values, interface{}, http.Header) (*http.Response, errors.Error) {
	return nil, errors.Default.New("not implemented")
}
func (c *fakeTempoClient) SetData(string, interface{})                      {}
func (c *fakeTempoClient) GetData(string) interface{}                       { return nil }
func (c *fakeTempoClient) SetHeaders(map[string]string)                     {}
func (c *fakeTempoClient) SetBeforeFunction(plugin.ApiClientBeforeRequest)  {}
func (c *fakeTempoClient) GetBeforeFunction() plugin.ApiClientBeforeRequest { return nil }
func (c *fakeTempoClient) SetAfterFunction(plugin.ApiClientAfterResponse)   {}
func (c *fakeTempoClient) GetAfterFunction() plugin.ApiClientAfterResponse  { return nil }

func TestListTempoRemoteTeamsPagination(t *testing.T) {
	connection := &models.TempoConnection{}
	connection.ID = 1

	t.Run("follows metadata.next, which Tempo v4 returns instead of a total", func(t *testing.T) {
		// shape of GET /4/teams (PageableTeam): no "total" in metadata
		client := &fakeTempoClient{body: `{
			"self": "https://api.tempo.io/4/teams?offset=0&limit=2",
			"metadata": {"count": 2, "offset": 0, "limit": 2,
				"next": "https://api.tempo.io/4/teams?offset=2&limit=2"},
			"results": [
				{"id": 17, "name": "Backend", "summary": "", "key": ""},
				{"id": 71, "name": "Frontend", "summary": "", "key": ""}
			]
		}`}
		children, nextPage, err := listTempoRemoteTeams(connection, client, "", TempoRemotePagination{Limit: 2})
		require.NoError(t, err)
		require.Len(t, children, 2)
		assert.Equal(t, "17", children[0].Id)
		assert.Equal(t, "0", client.query.Get("offset"))
		assert.Equal(t, "2", client.query.Get("limit"))
		require.NotNil(t, nextPage)
		assert.Equal(t, TempoRemotePagination{Limit: 2, Offset: 2}, *nextPage)
	})

	t.Run("stops on the last page, which has no next link", func(t *testing.T) {
		client := &fakeTempoClient{body: `{
			"self": "https://api.tempo.io/4/teams?offset=2&limit=2",
			"metadata": {"count": 1, "offset": 2, "limit": 2,
				"previous": "https://api.tempo.io/4/teams?offset=0&limit=2"},
			"results": [{"id": 90, "name": "QA", "summary": "", "key": ""}]
		}`}
		children, nextPage, err := listTempoRemoteTeams(connection, client, "", TempoRemotePagination{Limit: 2, Offset: 2})
		require.NoError(t, err)
		require.Len(t, children, 1)
		assert.Equal(t, "2", client.query.Get("offset"))
		assert.Nil(t, nextPage)
	})
}

// two pages of GET /4/teams (PageableTeam): the first links the second
var twoTeamPages = map[string]string{
	"0": `{
		"self": "https://api.tempo.io/4/teams?offset=0&limit=50",
		"metadata": {"count": 2, "offset": 0, "limit": 50,
			"next": "https://api.tempo.io/4/teams?offset=50&limit=50"},
		"results": [
			{"id": 17, "name": "Backend", "summary": "", "key": ""},
			{"id": 71, "name": "Frontend", "summary": "", "key": ""}
		]
	}`,
	"50": `{
		"self": "https://api.tempo.io/4/teams?offset=50&limit=50",
		"metadata": {"count": 1, "offset": 50, "limit": 50,
			"previous": "https://api.tempo.io/4/teams?offset=0&limit=50"},
		"results": [{"id": 90, "name": "Backend QA", "summary": "", "key": ""}]
	}`,
}

func TestListAllTempoTeams(t *testing.T) {
	client := &fakeTempoClient{pages: twoTeamPages}
	teams, err := listAllTempoTeams(client, url.Values{"name": {"x"}})
	require.NoError(t, err)
	require.Len(t, teams, 3)
	assert.Equal(t, int64(17), teams[0].Id)
	assert.Equal(t, int64(90), teams[2].Id)
	require.Len(t, client.queries, 2)
	assert.Equal(t, "0", client.queries[0].Get("offset"))
	assert.Equal(t, "50", client.queries[1].Get("offset"))
	assert.Equal(t, "50", client.queries[1].Get("limit"))
	assert.Equal(t, "x", client.queries[1].Get("name"))
}

func TestSearchTempoRemoteTeamsSearchesEveryPage(t *testing.T) {
	client := &fakeTempoClient{pages: twoTeamPages}
	children, err := searchTempoRemoteTeams(client, &dsmodels.DsRemoteApiScopeSearchParams{
		Search: "backend", Page: 1, PageSize: 50,
	})
	require.NoError(t, err)
	require.Len(t, children, 2)
	assert.Equal(t, "17", children[0].Id)
	// on the second page of the Tempo API
	assert.Equal(t, "90", children[1].Id)
	assert.Equal(t, "backend", client.query.Get("name"))
}
