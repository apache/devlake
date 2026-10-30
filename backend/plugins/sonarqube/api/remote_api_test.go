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
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSearchSonarqubeRemoteProjectsReturnsTotal(t *testing.T) {
	apiClient := mockplugin.NewApiClient(t)
	var gotQuery url.Values
	apiClient.EXPECT().Get("components/search_projects", mock.Anything, mock.Anything).
		RunAndReturn(func(_ string, query url.Values, _ http.Header) (*http.Response, errors.Error) {
			gotQuery = query
			body := `{"paging":{"pageIndex":2,"pageSize":100,"total":237},` +
				`"components":[{"key":"k1","name":"proj-1"},{"key":"k2","name":"proj-2"}]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})

	children, total, err := searchSonarqubeRemoteProjects(apiClient, &dsmodels.DsRemoteApiScopeSearchParams{
		Search:   "proj",
		Page:     2,
		PageSize: 100,
	})

	assert.Nil(t, err)
	assert.Equal(t, 237, total)
	assert.Len(t, children, 2)
	assert.Equal(t, "k1", children[0].Id)
	assert.Equal(t, "proj-1", children[0].Name)
	assert.Equal(t, "2", gotQuery.Get("p"))
	assert.Equal(t, "100", gotQuery.Get("ps"))
	assert.Equal(t, `query = "proj"`, gotQuery.Get("filter"))
}

func TestSonarqubeSearchProjectsQueryFilter(t *testing.T) {
	t.Parallel()
	if g, w := sonarqubeSearchProjectsQueryFilter("iap"), `query = "iap"`; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
	if g, w := sonarqubeSearchProjectsQueryFilter("  my term  "), `query = "my term"`; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
	// double quotes are stripped to avoid breaking the filter string
	if g, w := sonarqubeSearchProjectsQueryFilter(`a"x`), `query = "ax"`; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
}
