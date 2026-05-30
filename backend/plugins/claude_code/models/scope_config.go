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
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.ToolLayerScopeConfig = (*ClaudeCodeScopeConfig)(nil)

// ClaudeCodeScopeConfig contains configuration for Claude Code data scope.
type ClaudeCodeScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`
}

func (ClaudeCodeScopeConfig) TableName() string {
	return "_tool_claude_code_scope_configs"
}

// GetConnectionId implements plugin.ToolLayerScopeConfig.
func (sc ClaudeCodeScopeConfig) GetConnectionId() uint64 {
	return sc.ConnectionId
}
