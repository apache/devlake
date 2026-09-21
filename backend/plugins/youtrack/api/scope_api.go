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
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

type PutScopesReqBody api.PutScopesReqBody[models.YoutrackProject]
type ScopeDetail api.ScopeDetail[models.YoutrackProject, models.YoutrackScopeConfig]

// PutScopes create or update youtrack projects
// @Summary create or update youtrack projects
// @Description Create or update youtrack projects
// @Tags plugins/youtrack
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param scope body PutScopesReqBody true "json"
// @Success 200  {object} []models.YoutrackProject
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scopes [PUT]
func PutScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.PutMultiple(input)
}

// PatchScope patch to youtrack project
// @Summary patch to youtrack project
// @Description patch to youtrack project
// @Tags plugins/youtrack
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param scopeId path string false "project ID"
// @Param scope body models.YoutrackProject true "json"
// @Success 200  {object} models.YoutrackProject
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scopes/{scopeId} [PATCH]
func PatchScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.Patch(input)
}

// GetScopeList get youtrack projects
// @Summary get youtrack projects
// @Description get youtrack projects
// @Tags plugins/youtrack
// @Param connectionId path int false "connection ID"
// @Param searchTerm query string false "search term for scope name"
// @Param pageSize query int false "page size, default 50"
// @Param page query int false "page size, default 1"
// @Success 200  {object} []ScopeDetail
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scopes/ [GET]
func GetScopeList(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.GetPage(input)
}

// GetScope get one youtrack project
// @Summary get one youtrack project
// @Description get one youtrack project
// @Tags plugins/youtrack
// @Param connectionId path int false "connection ID"
// @Param scopeId path string false "project ID"
// @Success 200  {object} ScopeDetail
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scopes/{scopeId} [GET]
func GetScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.GetScopeDetail(input)
}

// DeleteScope delete plugin data associated with the scope and optionally the scope itself
// @Summary delete plugin data associated with the scope and optionally the scope itself
// @Description delete data associated with plugin scope
// @Tags plugins/youtrack
// @Param connectionId path int true "connection ID"
// @Param scopeId path string true "project ID"
// @Param delete_data_only query bool false "Only delete the scope data, not the scope itself"
// @Success 200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 409  {object} api.ScopeRefDoc "References exist to this scope"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scopes/{scopeId} [DELETE]
func DeleteScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.Delete(input)
}
