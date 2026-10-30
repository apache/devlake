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
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/srvhelper"
	"github.com/apache/devlake/plugins/sonarqube/models"
	"github.com/go-playground/validator/v10"
)

var vld *validator.Validate
var basicRes context.BasicRes

var dsHelper *api.DsHelper[models.SonarqubeConnection, models.SonarqubeProject, srvhelper.NoScopeConfig]
var raProxy *api.DsRemoteApiProxyHelper[models.SonarqubeConnection]
var raScopeList *api.DsRemoteApiScopeListHelper[models.SonarqubeConnection, models.SonarqubeProject, SonarqubeRemotePagination]
var raScopeSearch *api.DsRemoteApiScopeSearchHelper[models.SonarqubeConnection, models.SonarqubeProject]

func Init(br context.BasicRes, p plugin.PluginMeta) {
	basicRes = br
	vld = validator.New()
	dsHelper = api.NewDataSourceHelper[
		models.SonarqubeConnection,
		models.SonarqubeProject,
		srvhelper.NoScopeConfig,
	](
		br,
		p.Name(),
		[]string{"name"},
		func(c models.SonarqubeConnection) models.SonarqubeConnection {
			return c.Sanitize()
		},
		nil,
		nil,
	)
	raProxy = api.NewDsRemoteApiProxyHelper[models.SonarqubeConnection](dsHelper.ConnApi.ModelApiHelper)
	raScopeList = api.NewDsRemoteApiScopeListHelper[
		models.SonarqubeConnection,
		models.SonarqubeProject,
		SonarqubeRemotePagination,
	](raProxy, listSonarqubeRemoteScopes)
	raScopeSearch = api.NewDsRemoteApiScopeSearchHelperWithCount[models.SonarqubeConnection, models.SonarqubeProject](raProxy, searchSonarqubeRemoteProjects)
}
