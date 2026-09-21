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
)

// defaultEntities is the domain-type pair every YouTrack scope config
// collects: issues (TICKET) and their changelog-derived cross-domain records
// (CROSS). The config-ui always sends both; this default keeps raw API
// callers from persisting a scope config that collects nothing.
var defaultEntities = []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS}

// defaultScopeConfigEntities fills in the plugin's default entity selection
// when the request body leaves it unset. An explicit selection — even a
// single domain type — is never overridden. Over the wire the body is
// JSON-decoded into map[string]interface{}, so entities arrives as
// []interface{}; the []string shape only occurs in-process.
func defaultScopeConfigEntities(body map[string]interface{}) {
	switch entities := body["entities"].(type) {
	case []interface{}:
		if len(entities) == 0 {
			body["entities"] = defaultEntities
		}
	case []string:
		if len(entities) == 0 {
			body["entities"] = defaultEntities
		}
	default:
		body["entities"] = defaultEntities
	}
}

// PostScopeConfig create scope config for YouTrack
// @Summary create scope config for YouTrack
// @Description create scope config for YouTrack
// @Tags plugins/youtrack
// @Accept application/json
// @Param scopeConfig body models.YoutrackScopeConfig true "scope config"
// @Success 200  {object} models.YoutrackScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scope-configs [POST]
func PostScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	defaultScopeConfigEntities(input.Body)
	return dsHelper.ScopeConfigApi.Post(input)
}

// PatchScopeConfig update scope config for YouTrack
// @Summary update scope config for YouTrack
// @Description update scope config for YouTrack
// @Tags plugins/youtrack
// @Accept application/json
// @Param scopeConfigId path int true "scopeConfigId"
// @Param scopeConfig body models.YoutrackScopeConfig true "scope config"
// @Success 200  {object} models.YoutrackScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scope-configs/{scopeConfigId} [PATCH]
func PatchScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.Patch(input)
}

// GetScopeConfig return one scope config
// @Summary return one scope config
// @Description return one scope config
// @Tags plugins/youtrack
// @Param scopeConfigId path int true "scopeConfigId"
// @Success 200  {object} models.YoutrackScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scope-configs/{scopeConfigId} [GET]
func GetScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.GetDetail(input)
}

// GetScopeConfigList return all scope configs
// @Summary return all scope configs
// @Description return all scope configs
// @Tags plugins/youtrack
// @Param pageSize query int false "page size, default 50"
// @Param page query int false "page size, default 1"
// @Success 200  {object} []models.YoutrackScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scope-configs [GET]
func GetScopeConfigList(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.GetAll(input)
}

// DeleteScopeConfig delete a scope config
// @Summary delete a scope config
// @Description delete a scope config
// @Tags plugins/youtrack
// @Param scopeConfigId path int true "scopeConfigId"
// @Param connectionId path int true "connectionId"
// @Success 200
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/scope-configs/{scopeConfigId} [DELETE]
func DeleteScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.Delete(input)
}

// GetProjectsByScopeConfig return projects details related by scope config
// @Summary return all related projects
// @Description return all related projects
// @Tags plugins/youtrack
// @Param scopeConfigId path int true "scopeConfigId"
// @Success 200  {object} models.ProjectScopeOutput
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/scope-config/{scopeConfigId}/projects [GET]
func GetProjectsByScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.GetProjectsByScopeConfig(input)
}
