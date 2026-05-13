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

package archived

import (
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
)

// Service mirrors the columns contributed by live `models.Service`'s
// embedded `common.Scope` (NoPKModel + ConnectionId + ScopeConfigId)
// plus the Rootly-specific fields. Missing `scope_config_id` here
// would cause PUT /scopes to fail with "Unknown column 'scope_config_id'".
type Service struct {
	archived.NoPKModel
	ConnectionId  uint64 `gorm:"primaryKey"`
	ScopeConfigId uint64 `json:"scopeConfigId,omitempty" mapstructure:"scopeConfigId,omitempty"`
	Id            string `gorm:"primaryKey;autoIncrement:false"`
	Url           string
	Name          string
}

func (Service) TableName() string {
	return "_tool_rootly_services"
}
