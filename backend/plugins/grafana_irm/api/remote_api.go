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
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/devlake/helpers/pluginhelper/api/models"
	"github.com/apache/devlake/plugins/grafana_irm/models"
)

// GrafanaIrmRemotePagination is unused: there is only ever one scope to list,
// so there is never a next page.
type GrafanaIrmRemotePagination struct{}

// defaultScope is the single synthetic scope every Grafana IRM connection
// has, covering its whole real-incident stream (grafana_irm_plan.md §4.2/
// §4.3): the API has no listable service/team resource to browse, and the
// originating feature request only ever asked for one org-wide incident
// feed, so there is nothing else to list.
func defaultScope() dsmodels.DsRemoteApiScopeListEntry[models.GrafanaIrmScope] {
	return dsmodels.DsRemoteApiScopeListEntry[models.GrafanaIrmScope]{
		Type:     api.RAS_ENTRY_TYPE_SCOPE,
		Id:       "default",
		Name:     "All Incidents",
		FullName: "All Incidents",
		Data: &models.GrafanaIrmScope{
			Id:   "default",
			Name: "All Incidents",
		},
	}
}

func listGrafanaIrmRemoteScopes(
	_ *models.GrafanaIrmConnection,
	_ plugin.ApiClient,
	_ string,
	_ GrafanaIrmRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.GrafanaIrmScope],
	nextPage *GrafanaIrmRemotePagination,
	err errors.Error,
) {
	return []dsmodels.DsRemoteApiScopeListEntry[models.GrafanaIrmScope]{defaultScope()}, nil, nil
}

func searchGrafanaIrmRemoteScopes(
	_ plugin.ApiClient,
	params *dsmodels.DsRemoteApiScopeSearchParams,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.GrafanaIrmScope],
	err errors.Error,
) {
	entry := defaultScope()
	if params.Search != "" && !strings.Contains(strings.ToLower(entry.Name), strings.ToLower(params.Search)) {
		return nil, nil
	}
	return []dsmodels.DsRemoteApiScopeListEntry[models.GrafanaIrmScope]{entry}, nil
}

// RemoteScopes lists the single synthetic scope available on this connection
// @Summary list the available scope for this connection
// @Description Grafana IRM has exactly one scope per connection, covering its whole incident stream
// @Tags plugins/grafana_irm
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param groupId query string false "group ID"
// @Param pageToken query string false "page Token"
// @Success 200  {object} RemoteScopesOutput
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/grafana_irm/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

// SearchRemoteScopes searches the single synthetic scope available on this connection
// @Summary search the available scope for this connection
// @Description Grafana IRM has exactly one scope per connection, covering its whole incident stream
// @Tags plugins/grafana_irm
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param search query string false "search"
// @Param page query int false "page number"
// @Param pageSize query int false "page size per page"
// @Success 200  {object} SearchRemoteScopesOutput
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/grafana_irm/connections/{connectionId}/search-remote-scopes [GET]
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeSearch.Get(input)
}
