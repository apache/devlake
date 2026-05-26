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
	"gorm.io/datatypes"
)

var _ plugin.ToolLayerScopeConfig = (*GithubScopeConfig)(nil)

type GithubScopeConfig struct {
	common.ScopeConfig           `mapstructure:",squash" json:",inline" gorm:"embedded"`
	PrType                       string            `mapstructure:"prType,omitempty" json:"prType" gorm:"type:varchar(255)"`
	PrComponent                  string            `mapstructure:"prComponent,omitempty" json:"prComponent" gorm:"type:varchar(255)"`
	PrBodyClosePattern           string            `mapstructure:"prBodyClosePattern,omitempty" json:"prBodyClosePattern" gorm:"type:varchar(255)"`
	IssueSeverity                string            `mapstructure:"issueSeverity,omitempty" json:"issueSeverity" gorm:"type:varchar(255)"`
	IssuePriority                string            `mapstructure:"issuePriority,omitempty" json:"issuePriority" gorm:"type:varchar(255)"`
	IssueComponent               string            `mapstructure:"issueComponent,omitempty" json:"issueComponent" gorm:"type:varchar(255)"`
	IssueTypeBug                 string            `mapstructure:"issueTypeBug,omitempty" json:"issueTypeBug" gorm:"type:varchar(255)"`
	IssueTypeIncident            string            `mapstructure:"issueTypeIncident,omitempty" json:"issueTypeIncident" gorm:"type:varchar(255)"`
	IssueTypeRequirement         string            `mapstructure:"issueTypeRequirement,omitempty" json:"issueTypeRequirement" gorm:"type:varchar(255)"`
	DeploymentPattern            string            `mapstructure:"deploymentPattern,omitempty" json:"deploymentPattern" gorm:"type:varchar(255)"`
	ProductionPattern            string            `mapstructure:"productionPattern,omitempty" json:"productionPattern" gorm:"type:varchar(255)"`
	EnvNamePattern               string            `mapstructure:"envNamePattern,omitempty" json:"envNamePattern" gorm:"type:varchar(255)"`
	Refdiff                      datatypes.JSONMap `mapstructure:"refdiff,omitempty" json:"refdiff" swaggertype:"object" format:"json"`
	PrSizeExcludedFileExtensions []string          `mapstructure:"prSizeExcludedFileExtensions" json:"prSizeExcludedFileExtensions" gorm:"type:json;serializer:json"`
}

// GetConnectionId implements plugin.ToolLayerScopeConfig.
func (sc GithubScopeConfig) GetConnectionId() uint64 {
	return sc.ConnectionId
}

func (GithubScopeConfig) TableName() string {
	return "_tool_github_scope_configs"
}
