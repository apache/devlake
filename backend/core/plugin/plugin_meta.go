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

package plugin

import "github.com/apache/devlake/core/dal"

// PluginMeta is the Minimal features a plugin should comply, should be implemented by all plugins
type PluginMeta interface {
	Description() string
	// PkgPath information lost when compiled as plugin(.so)
	RootPkgPath() string
	Name() string
}

type GrafanaDashboard struct {
	ID                   string
	Title                string
	Description          string
	GrafanaDashboardJson string
}

// PluginDashboard return its dashboard which should be display at grafana
type PluginDashboard interface {
	Dashboards() []GrafanaDashboard
}

// PluginIcon return its icon (.svg text)
type PluginIcon interface {
	SvgIcon() string
}

// PluginSource abstracts data sources
type PluginSource interface {
	Connection() dal.Tabler
	Scope() ToolLayerScope
	ScopeConfig() dal.Tabler
}
