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
)

// GetScopeLatestSyncState get the latest sync state of a scope
// @Summary get the latest sync state of a scope
// @Description get the latest sync state of a scope
// @Tags plugins/codecov
// @Param connectionId path int true "connection ID"
// @Param scopeId path string true "scope ID"
// @Success 200  {object} []models.LatestSyncState
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/codecov/connections/{connectionId}/scopes/{scopeId}/latest-sync-state [GET]
func GetScopeLatestSyncState(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	scopeId := strings.TrimLeft(input.Params["scopeId"], "/")
	// URL decode the scopeId in case it's encoded (e.g., %2F for /)
	if decoded, err := url.QueryUnescape(scopeId); err == nil {
		scopeId = decoded
	}
	input.Params["scopeId"] = scopeId
	return dsHelper.ScopeApi.GetScopeLatestSyncState(input)
}

