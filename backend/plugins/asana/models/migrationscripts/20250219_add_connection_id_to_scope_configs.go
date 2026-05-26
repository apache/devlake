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

package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addConnectionIdToAsanaScopeConfigs)(nil)

// asanaScopeConfig20250219 adds connection_id and name to match common.ScopeConfig.
// The init migration used archived.ScopeConfig which did not have these columns.
type asanaScopeConfig20250219 struct {
	ConnectionId uint64 `json:"connectionId" gorm:"index" mapstructure:"connectionId,omitempty"`
	Name         string `mapstructure:"name" json:"name" gorm:"type:varchar(255);uniqueIndex"`
}

func (asanaScopeConfig20250219) TableName() string {
	return "_tool_asana_scope_configs"
}

type addConnectionIdToAsanaScopeConfigs struct{}

func (*addConnectionIdToAsanaScopeConfigs) Up(basicRes context.BasicRes) errors.Error {
	return basicRes.GetDal().AutoMigrate(&asanaScopeConfig20250219{})
}

func (*addConnectionIdToAsanaScopeConfigs) Version() uint64 {
	return 20250219000001
}

func (*addConnectionIdToAsanaScopeConfigs) Name() string {
	return "add connection_id and name to _tool_asana_scope_configs"
}
