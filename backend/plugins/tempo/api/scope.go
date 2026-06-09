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
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/tempo/models"
)

type PutScopesReqBody api.PutScopesReqBody[models.TempoTeam]
type ScopeDetail api.ScopeDetail[models.TempoTeam, models.TempoScopeConfig]

// PutScope create or update tempo team
// @Summary create or update tempo team
// @Description Create or update Tempo team
// @Tags plugins/tempo
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param searchTerm query string false "search term for scope name"
// @Param scope body PutScopesReqBody true "json"
// @Success 200  {object} []models.TempoTeam
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/tempo/connections/{connectionId}/scopes [PUT]
func PutScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.PutMultiple(input)
}

// UpdateScope patch to tempo team
// @Summary patch to tempo team
// @Description patch to tempo team
// @Tags plugins/tempo
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param scopeId path int false "team ID"
// @Param scope body models.TempoTeam true "json"
// @Success 200  {object} models.TempoTeam
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/tempo/connections/{connectionId}/scopes/{scopeId} [PATCH]
func UpdateScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.Patch(input)
}

// GetScopeList get Tempo teams
// @Summary get Tempo teams
// @Description get Tempo teams
// @Tags plugins/tempo
// @Param connectionId path int false "connection ID"
// @Param pageSize query int false "page size, default 50"
// @Param page query int false "page size, default 1"
// @Param blueprints query bool false "also return blueprints using these scopes as part of the payload"
// @Success 200  {object} []ScopeDetail
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/tempo/connections/{connectionId}/scopes/ [GET]
func GetScopeList(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.GetPage(input)
}

// GetScope get one Tempo team
// @Summary get one Tempo team
// @Description get one Tempo team
// @Tags plugins/tempo
// @Param connectionId path int false "connection ID"
// @Param scopeId path int false "team ID"
// @Success 200  {object} ScopeDetail
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/tempo/connections/{connectionId}/scopes/{scopeId} [GET]
func GetScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.GetScopeDetail(input)
}

// DeleteScope delete plugin data associated with the scope and optionally the scope itself
// @Summary delete plugin data associated with the scope and optionally the scope itself
// @Description delete data associated with plugin scope
// @Tags plugins/tempo
// @Param connectionId path int true "connection ID"
// @Param scopeId path int true "scope ID"
// @Param delete_data_only query bool false "Only delete the scope data, not the scope itself"
// @Success 200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 409  {object} srvhelper.DsRefs "References exist to this scope"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/tempo/connections/{connectionId}/scopes/{scopeId} [DELETE]
func DeleteScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.Delete(input)
}

// GetScopeLatestSyncState get one Tempo team's latest sync state
// @Summary get one Tempo team's latest sync state
// @Description get one Tempo team's latest sync state
// @Tags plugins/tempo
// @Param connectionId path int true "connection ID"
// @Param scopeId path int true "scope ID"
// @Success 200  {object} []models.LatestSyncState
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/tempo/connections/{connectionId}/scopes/{scopeId}/latest-sync-state [GET]
func GetScopeLatestSyncState(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.GetScopeLatestSyncState(input)
}
