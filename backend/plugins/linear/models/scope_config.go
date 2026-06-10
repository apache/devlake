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
	"github.com/apache/incubator-devlake/core/models/common"
)

// LinearScopeConfig keeps status mapping deterministic (Linear's
// WorkflowState.type maps to TODO/IN_PROGRESS/DONE without user input) but
// allows label-based issue-type mapping. Linear has no native issue "type", so
// each pattern below is a regular expression matched against an issue's label
// names to derive the domain ticket.Issue.Type. Precedence is
// INCIDENT > BUG > REQUIREMENT; an issue matching none defaults to REQUIREMENT.
type LinearScopeConfig struct {
	common.ScopeConfig   `mapstructure:",squash" json:",inline" gorm:"embedded"`
	IssueTypeRequirement string `mapstructure:"issueTypeRequirement,omitempty" json:"issueTypeRequirement" gorm:"type:varchar(255)"`
	IssueTypeBug         string `mapstructure:"issueTypeBug,omitempty" json:"issueTypeBug" gorm:"type:varchar(255)"`
	IssueTypeIncident    string `mapstructure:"issueTypeIncident,omitempty" json:"issueTypeIncident" gorm:"type:varchar(255)"`
}

func (LinearScopeConfig) TableName() string {
	return "_tool_linear_scope_configs"
}

func (sc *LinearScopeConfig) SetConnectionId(c *LinearScopeConfig, connectionId uint64) {
	c.ConnectionId = connectionId
	c.ScopeConfig.ConnectionId = connectionId
}
