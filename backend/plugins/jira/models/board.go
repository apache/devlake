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
	"fmt"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.ToolLayerScope = (*JiraBoard)(nil)

type JiraBoard struct {
	common.Scope `mapstructure:",squash"`
	BoardId      uint64 `json:"boardId" mapstructure:"boardId" validate:"required" gorm:"primaryKey"`
	ProjectId    uint   `json:"projectId" mapstructure:"projectId"`
	Name         string `json:"name" mapstructure:"name" gorm:"type:varchar(255)"`
	Self         string `json:"self" mapstructure:"self" gorm:"type:varchar(255)"`
	Type         string `json:"type" mapstructure:"type" gorm:"type:varchar(100)"`
	Jql          string `json:"jql" mapstructure:"jql"`
	SubQuery     string `json:"subQuery" mapstructure:"subQuery"`
}

func (b JiraBoard) ScopeId() string {
	return fmt.Sprintf("%d", b.BoardId)
}

func (b JiraBoard) ScopeName() string {
	return b.Name
}

func (b JiraBoard) ScopeFullName() string {
	return b.Name
}

func (b JiraBoard) ScopeParams() interface{} {
	return &JiraApiParams{
		ConnectionId: b.ConnectionId,
		BoardId:      b.BoardId,
	}
}

func (JiraBoard) TableName() string {
	return "_tool_jira_boards"
}

type JiraApiParams struct {
	ConnectionId uint64
	BoardId      uint64
}
