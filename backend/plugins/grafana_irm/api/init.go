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
	"github.com/apache/devlake/plugins/grafana_irm/models"
	"github.com/go-playground/validator/v10"
)

var vld *validator.Validate
var basicRes context.BasicRes

var dsHelper *api.DsHelper[models.GrafanaIrmConnection, models.GrafanaIrmScope, models.GrafanaIrmScopeConfig]
var raProxy *api.DsRemoteApiProxyHelper[models.GrafanaIrmConnection]
var raScopeList *api.DsRemoteApiScopeListHelper[models.GrafanaIrmConnection, models.GrafanaIrmScope, GrafanaIrmRemotePagination]
var raScopeSearch *api.DsRemoteApiScopeSearchHelper[models.GrafanaIrmConnection, models.GrafanaIrmScope]

func Init(br context.BasicRes, p plugin.PluginMeta) {
	vld = validator.New()
	basicRes = br
	dsHelper = api.NewDataSourceHelper[
		models.GrafanaIrmConnection, models.GrafanaIrmScope, models.GrafanaIrmScopeConfig,
	](
		br,
		p.Name(),
		[]string{"name"},
		func(c models.GrafanaIrmConnection) models.GrafanaIrmConnection {
			return c.Sanitize()
		},
		nil,
		nil,
	)
	// The Grafana Incident API has no remote-listable service/team resource
	// (grafana_irm_plan.md §4), but config-ui's data-scope picker has no
	// manual-entry flow to fall back to either — so these list/search a
	// single synthetic "whole connection" scope (§4.3) rather than a real
	// remote list.
	raProxy = api.NewDsRemoteApiProxyHelper[models.GrafanaIrmConnection](dsHelper.ConnApi.ModelApiHelper)
	raScopeList = api.NewDsRemoteApiScopeListHelper[models.GrafanaIrmConnection, models.GrafanaIrmScope, GrafanaIrmRemotePagination](raProxy, listGrafanaIrmRemoteScopes)
	raScopeSearch = api.NewDsRemoteApiScopeSearchHelper[models.GrafanaIrmConnection, models.GrafanaIrmScope](raProxy, searchGrafanaIrmRemoteScopes)
}
