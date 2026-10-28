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

// PostScopeConfig create scope config
// @Summary create scope config
// @Description create scope config
// @Tags plugins/codecov
// @Accept application/json
// @Param connectionId path int true "connection ID"
// @Param scopeConfig body models.CodecovScopeConfig true "scope config"
// @Success 200  {object} models.CodecovScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scope-configs [POST]
func PostScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.Post(input)
}

// GetScopeConfigList get all scope configs
// @Summary get all scope configs
// @Description get all scope configs
// @Tags plugins/codecov
// @Param connectionId path int true "connection ID"
// @Param pageSize query int false "page size, default 50"
// @Param page query int false "page size, default 1"
// @Success 200  {object} []models.CodecovScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scope-configs [GET]
func GetScopeConfigList(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.GetAll(input)
}

// GetScopeConfig get one scope config
// @Summary get one scope config
// @Description get one scope config
// @Tags plugins/codecov
// @Param connectionId path int true "connection ID"
// @Param id path int true "scope config ID"
// @Success 200  {object} models.CodecovScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scope-configs/{id} [GET]
func GetScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.GetDetail(input)
}

// PatchScopeConfig patch scope config
// @Summary patch scope config
// @Description patch scope config
// @Tags plugins/codecov
// @Accept application/json
// @Param connectionId path int true "connection ID"
// @Param id path int true "scope config ID"
// @Param scopeConfig body models.CodecovScopeConfig true "scope config"
// @Success 200  {object} models.CodecovScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scope-configs/{id} [PATCH]
func PatchScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.Patch(input)
}

// DeleteScopeConfig delete scope config
// @Summary delete scope config
// @Description delete scope config
// @Tags plugins/codecov
// @Param connectionId path int true "connection ID"
// @Param id path int true "scope config ID"
// @Success 200  {object} models.CodecovScopeConfig
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 409  {object} srvhelper.DsRefs "References exist to this scope config"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scope-configs/{id} [DELETE]
func DeleteScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.Delete(input)
}

