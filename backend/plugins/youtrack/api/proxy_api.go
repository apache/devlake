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

// Proxy forwards api request to the YouTrack REST API through the connection.
// The config UI uses exactly one proxied path
// (`admin/projects/{id}/customFields?...`) for field and bundle discovery;
// scope discovery goes through remote-scopes instead.
// @Summary Forward api request to the YouTrack REST API
// @Description Forward api request to the YouTrack REST API
// @Tags plugins/youtrack
// @Param connectionId path int true "connection ID"
// @Param path path string true "path to YouTrack REST API"
// @Success 200 {object} interface{} "Success"
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Router /plugins/youtrack/connections/{connectionId}/proxy/rest/{path} [GET]
func Proxy(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raProxy.Proxy(input)
}
