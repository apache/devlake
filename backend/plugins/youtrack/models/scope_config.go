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

// YoutrackScopeConfig carries the per-scope-config settings that make the
// plugin work on instances whose custom fields were renamed: six field-name
// slots naming which custom field plays each well-known role, and two flat,
// name-keyed mapping tables.
//
// The core four slots (Type/State/Priority/Assignee) fall back to their
// default field names when empty because they drive core conversion;
// StoryPointField/DueDateField are optional enrichments, so empty means off.
//
// Mappings are deliberately flat and name-keyed (a rejection of Jira's
// type-nested shape):
// same-named values across projects map once, and an upstream rename degrades
// gracefully to the defaults instead of breaking.
type YoutrackScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`

	TypeField       string `mapstructure:"typeField,omitempty" json:"typeField" gorm:"type:varchar(255)"`
	StateField      string `mapstructure:"stateField,omitempty" json:"stateField" gorm:"type:varchar(255)"`
	PriorityField   string `mapstructure:"priorityField,omitempty" json:"priorityField" gorm:"type:varchar(255)"`
	AssigneeField   string `mapstructure:"assigneeField,omitempty" json:"assigneeField" gorm:"type:varchar(255)"`
	StoryPointField string `mapstructure:"storyPointField,omitempty" json:"storyPointField" gorm:"type:varchar(255)"`
	DueDateField    string `mapstructure:"dueDateField,omitempty" json:"dueDateField" gorm:"type:varchar(255)"`

	// TypeMappings maps a type custom-field value name to a standard
	// ticket.Issue.Type: BUG|REQUIREMENT|INCIDENT|TASK|SUBTASK.
	TypeMappings map[string]string `mapstructure:"typeMappings,omitempty" json:"typeMappings" gorm:"type:json;serializer:json"`
	// StatusMappings maps a state custom-field value name to a standard
	// ticket.Issue.Status: TODO|IN_PROGRESS|DONE|OTHER.
	StatusMappings map[string]string `mapstructure:"statusMappings,omitempty" json:"statusMappings" gorm:"type:json;serializer:json"`
}

func (YoutrackScopeConfig) TableName() string {
	return "_tool_youtrack_scope_configs"
}

func (sc *YoutrackScopeConfig) SetConnectionId(c *YoutrackScopeConfig, connectionId uint64) {
	c.ConnectionId = connectionId
	c.ScopeConfig.ConnectionId = connectionId
}
