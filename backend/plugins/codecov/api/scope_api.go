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
	"net/url"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

type PutScopesReqBody api.PutScopesReqBody[models.CodecovRepo]
type ScopeDetail api.ScopeDetail[models.CodecovRepo, models.CodecovScopeConfig]

// PutScopes create or update codecov repo
// @Summary create or update codecov repo
// @Description Create or update codecov repo
// @Tags plugins/codecov
// @Accept application/json
// @Param connectionId path int true "connection ID"
// @Param scope body PutScopesReqBody true "json"
// @Success 200  {object} []models.CodecovRepo
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scopes [PUT]
func PutScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.PutMultiple(input)
}

// PatchScope patch to codecov repo
// @Summary patch to codecov repo
// @Description patch to codecov repo
// @Tags plugins/codecov
// @Accept application/json
// @Param connectionId path int true "connection ID"
// @Param scopeId path string true "scope ID"
// @Param scope body models.CodecovRepo true "json"
// @Success 200  {object} models.CodecovRepo
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scopes/{scopeId} [PATCH]
func PatchScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	scopeId := strings.TrimLeft(input.Params["scopeId"], "/")
	// URL decode the scopeId in case it's encoded (e.g., %2F for /)
	if decoded, err := url.QueryUnescape(scopeId); err == nil {
		scopeId = decoded
	}
	input.Params["scopeId"] = scopeId
	return dsHelper.ScopeApi.Patch(input)
}

// GetScopes get Codecov repos
// @Summary get Codecov repos
// @Description get Codecov repos
// @Tags plugins/codecov
// @Param connectionId path int true "connection ID"
// @Param searchTerm query string false "search term for scope name"
// @Param pageSize query int false "page size, default 50"
// @Param page query int false "page size, default 1"
// @Param blueprints query bool false "also return blueprints using these scopes as part of the payload"
// @Success 200  {object} []ScopeDetail
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scopes [GET]
func GetScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.GetPage(input)
}

// GetScopeDispatcher handles scope routes with potential suffixes like /latest-sync-state
// @Summary get scope dispatcher
// @Description Handles scope routes with suffixes
// @Tags plugins/codecov
// @Param connectionId path int true "connection ID"
// @Param scopeId path string true "scope ID"
// @Success 200  {object} ScopeDetail
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scopes/{scopeId} [GET]
func GetScopeDispatcher(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	scopeIdWithSuffix := strings.TrimLeft(input.Params["scopeId"], "/")
	// URL decode the scopeId in case it's encoded (e.g., %2F for /)
	if decoded, err := url.QueryUnescape(scopeIdWithSuffix); err == nil {
		scopeIdWithSuffix = decoded
	}
	if strings.HasSuffix(scopeIdWithSuffix, "/latest-sync-state") {
		// Remove the /latest-sync-state suffix and trim any leading slashes
		scopeId := strings.TrimSuffix(scopeIdWithSuffix, "/latest-sync-state")
		scopeId = strings.TrimLeft(scopeId, "/")
		input.Params["scopeId"] = scopeId
		return GetScopeLatestSyncState(input)
	}
	// Trim leading slash for normal scope retrieval
	input.Params["scopeId"] = strings.TrimLeft(scopeIdWithSuffix, "/")
	return GetScope(input)
}

// GetScope get one Codecov repo
// @Summary get one Codecov repo
// @Description get one Codecov repo
// @Tags plugins/codecov
// @Param connectionId path int true "connection ID"
// @Param scopeId path string true "scope ID"
// @Param blueprints query bool false "also return blueprints using these scopes as part of the payload"
// @Success 200  {object} ScopeDetail
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scopes/{scopeId} [GET]
func GetScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	scopeId := strings.TrimLeft(input.Params["scopeId"], "/")
	// URL decode the scopeId in case it's encoded (e.g., %2F for /)
	if decoded, err := url.QueryUnescape(scopeId); err == nil {
		scopeId = decoded
	}
	input.Params["scopeId"] = scopeId
	return dsHelper.ScopeApi.GetScopeDetail(input)
}

// DeleteScope delete plugin data associated with the scope and optionally the scope itself
// @Summary delete plugin data associated with the scope and optionally the scope itself
// @Description delete data associated with plugin scope
// @Tags plugins/codecov
// @Param connectionId path int true "connection ID"
// @Param scopeId path string true "scope ID"
// @Param delete_data_only query bool false "Only delete the scope data, not the scope itself"
// @Success 200  {object} models.CodecovRepo
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 409  {object} srvhelper.DsRefs "References exist to this scope"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scopes/{scopeId} [DELETE]
func DeleteScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	scopeId := strings.TrimLeft(input.Params["scopeId"], "/")
	// URL decode the scopeId in case it's encoded (e.g., %2F for /)
	if decoded, err := url.QueryUnescape(scopeId); err == nil {
		scopeId = decoded
	}
	input.Params["scopeId"] = scopeId
	return dsHelper.ScopeApi.Delete(input)
}
