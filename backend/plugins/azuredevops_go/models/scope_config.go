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
	"gorm.io/datatypes"
)

var _ plugin.ToolLayerScopeConfig = (*AzuredevopsScopeConfig)(nil)

type AzuredevopsScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline"`

	DeploymentPattern string            `mapstructure:"deploymentPattern,omitempty" json:"deploymentPattern"`
	ProductionPattern string            `mapstructure:"productionPattern,omitempty" json:"productionPattern"`
	Refdiff           datatypes.JSONMap `mapstructure:"refdiff,omitempty" json:"refdiff" swaggertype:"object" format:"json"`
	ProcessTemplate   string            `mapstructure:"processTemplate,omitempty" json:"processTemplate"`
	TypeMappings      datatypes.JSONMap `mapstructure:"typeMappings,omitempty" json:"typeMappings" swaggertype:"object" format:"json"`
	StatusMappings    datatypes.JSONMap `mapstructure:"statusMappings,omitempty" json:"statusMappings" swaggertype:"object" format:"json"`
	CollectChangelogs bool              `mapstructure:"collectChangelogs" json:"collectChangelogs"`
	CollectComments   bool              `mapstructure:"collectComments" json:"collectComments"`
}

// GetConnectionId implements plugin.ToolLayerScopeConfig.
func (sc AzuredevopsScopeConfig) GetConnectionId() uint64 {
	return sc.ConnectionId
}

func (AzuredevopsScopeConfig) TableName() string {
	return "_tool_azuredevops_go_scope_configs"
}
