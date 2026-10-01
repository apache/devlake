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
)

// GrafanaIrmScopeConfig carries only the standard entity-selection fields
// (common.ScopeConfig) — same shape as incidentio's/pagerduty's. See
// grafana_irm_plan.md §4.2: a connection has exactly one scope covering its
// whole incident stream (there is no listable service/team resource to slice
// by, and the originating feature request never asked for per-team
// filtering), so there is nothing scope-specific left to configure here.
type GrafanaIrmScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`
}

func (GrafanaIrmScopeConfig) TableName() string {
	return "_tool_grafana_irm_scope_configs"
}
