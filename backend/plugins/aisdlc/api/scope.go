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
)

func GetScopeDispatcher(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	scopeIdWithSuffix := strings.TrimLeft(input.Params["scopeId"], "/")
	if scopeId, found := strings.CutSuffix(scopeIdWithSuffix, "/latest-sync-state"); found {
		input.Params["scopeId"] = scopeId
		return GetScopeLatestSyncState(input)
	}
	return GetScope(input)
}

func GetScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	input.Params["scopeId"] = strings.TrimLeft(input.Params["scopeId"], "/")
	return dsHelper.ScopeApi.GetScopeDetail(input)
}

func PatchScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	input.Params["scopeId"] = strings.TrimLeft(input.Params["scopeId"], "/")
	return dsHelper.ScopeApi.Patch(input)
}

func DeleteScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	input.Params["scopeId"] = strings.TrimLeft(input.Params["scopeId"], "/")
	return dsHelper.ScopeApi.Delete(input)
}

func GetScopeList(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.GetPage(input)
}

func PutScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.PutMultiple(input)
}
