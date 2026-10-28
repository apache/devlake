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

type TestRegistryScope struct {
	common.Scope `mapstructure:",squash"`
	Name         string `gorm:"type:varchar(500)" json:"name" mapstructure:"name"`                                        // Repository name (e.g., "konflux-team/release-service-catalog")
	FullName     string `gorm:"primaryKey;type:varchar(500)" json:"fullName" mapstructure:"fullName" validate:"required"` // Full name with organization (e.g., "konflux-test-storage/konflux-team/release-service-catalog")
	Id           string `gorm:"-" json:"id" mapstructure:"-"`                                                             // Computed field: same as FullName, for frontend compatibility with default getPluginScopeId
}

// MarshalJSON customizes JSON serialization to populate Id field from FullName
func (s TestRegistryScope) MarshalJSON() ([]byte, error) {
	// Create an alias type to avoid infinite recursion
	type Alias TestRegistryScope

	// Populate Id from FullName for frontend compatibility
	alias := Alias(s)
	alias.Id = s.FullName

	return json.Marshal(alias)
}

func (TestRegistryScope) TableName() string {
	return "_tool_testregistry_scopes"
}

func (s TestRegistryScope) ScopeId() string {
	return s.FullName
}

func (s TestRegistryScope) ScopeName() string {
	return s.Name
}

func (s TestRegistryScope) ScopeFullName() string {
	return s.FullName
}

func (s TestRegistryScope) ScopeParams() interface{} {
	return &TestRegistryApiParams{
		ConnectionId: s.ConnectionId,
		FullName:     s.FullName,
	}
}

type TestRegistryApiParams struct {
	ConnectionId uint64 `json:"connectionId"`
	FullName     string `json:"fullName"`
}

var _ plugin.ToolLayerScope = (*TestRegistryScope)(nil)
