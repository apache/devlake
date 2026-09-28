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

package models

import (
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
)

type GrafanaIrmParams struct {
	ConnectionId uint64
	ScopeId      string
}

// GrafanaIrmScope is the connection-level scope described in
// grafana_irm_plan.md §4: the Grafana Incident API has no remote-listable
// "service"/"team" resource to scope by (unlike incidentio's incident
// types), so a scope here is a user-named grouping the operator creates by
// hand rather than one picked from a remote list. Any label-filter or other
// per-scope collection behavior is deferred until that logic is designed.
type GrafanaIrmScope struct {
	common.Scope `mapstructure:",squash"`
	Id           string `json:"id" mapstructure:"id" gorm:"primaryKey;autoIncrement:false"`
	Name         string `json:"name" mapstructure:"name"`
}

func (s GrafanaIrmScope) ScopeId() string {
	return s.Id
}

func (s GrafanaIrmScope) ScopeName() string {
	return s.Name
}

func (s GrafanaIrmScope) ScopeFullName() string {
	return s.Name
}

func (s GrafanaIrmScope) ScopeParams() interface{} {
	return &GrafanaIrmParams{
		ConnectionId: s.ConnectionId,
		ScopeId:      s.Id,
	}
}

func (s GrafanaIrmScope) TableName() string {
	return "_tool_grafana_irm_scopes"
}

var _ plugin.ToolLayerScope = (*GrafanaIrmScope)(nil)
