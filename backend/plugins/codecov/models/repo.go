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
	"encoding/json"

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.ToolLayerScope = (*CodecovRepo)(nil)

type CodecovRepo struct {
	common.Scope `mapstructure:",squash" gorm:"embedded"`
	CodecovId    string `json:"codecovId" gorm:"primaryKey;type:varchar(255)" validate:"required" mapstructure:"codecovId"`
	Name         string `json:"name" gorm:"type:varchar(255)" mapstructure:"name,omitempty"`
	FullName     string `json:"fullName" gorm:"type:varchar(255)" mapstructure:"fullName,omitempty"`
	Service      string `json:"service" gorm:"type:varchar(255)" mapstructure:"service,omitempty"`
	Language     string `json:"language" gorm:"type:varchar(255)" mapstructure:"language,omitempty"`
	Active       bool   `json:"active" mapstructure:"active,omitempty"`
	ActivatedAt  string `json:"activatedAt" gorm:"type:varchar(255)" mapstructure:"activatedAt,omitempty"`
	Updatestamp  string `json:"updatestamp" gorm:"type:varchar(255)" mapstructure:"updatestamp,omitempty"`
	Private      bool   `json:"private" mapstructure:"private,omitempty"`
	Branch       string `json:"branch" gorm:"type:varchar(255)" mapstructure:"branch,omitempty"`
	Id           string `gorm:"-" json:"id" mapstructure:"-"` // Computed field: same as CodecovId, for frontend compatibility
}

// MarshalJSON customizes JSON serialization to populate Id field from CodecovId
func (r CodecovRepo) MarshalJSON() ([]byte, error) {
	// Create an alias type to avoid infinite recursion
	type Alias CodecovRepo

	// Populate Id from CodecovId for frontend compatibility
	alias := Alias(r)
	alias.Id = r.CodecovId

	return json.Marshal(alias)
}

func (r CodecovRepo) ScopeId() string {
	return r.CodecovId
}

func (r CodecovRepo) ScopeName() string {
	return r.Name
}

func (r CodecovRepo) ScopeFullName() string {
	return r.FullName
}

func (r CodecovRepo) ScopeParams() interface{} {
	return &CodecovApiParams{
		ConnectionId: r.ConnectionId,
		Name:         r.FullName,
	}
}

func (CodecovRepo) TableName() string {
	return "_tool_codecov_repos"
}

type CodecovApiParams struct {
	ConnectionId uint64
	Name         string
}
