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

// AisdlcScopeConfig holds the station registration for one scope.
// These values are configured. They are not inferred from repository names.
type AisdlcScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`
	TenantId           string `json:"tenantId" mapstructure:"tenantId" gorm:"type:varchar(255)"`
	SourceId           string `json:"sourceId" mapstructure:"sourceId" gorm:"type:varchar(255)"`
	StationId          string `json:"stationId" mapstructure:"stationId" gorm:"type:varchar(255)"`
	DisplayName        string `json:"displayName" mapstructure:"displayName" gorm:"type:varchar(255)"`
	Parser             string `json:"parser" mapstructure:"parser" gorm:"type:varchar(64)"`
	Branch             string `json:"branch" mapstructure:"branch" gorm:"type:varchar(255)"`
	PathPrefix         string `json:"pathPrefix" mapstructure:"pathPrefix" gorm:"type:varchar(1024)"`
}

func (AisdlcScopeConfig) TableName() string {
	return "_tool_aisdlc_scope_configs"
}
