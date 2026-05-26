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

var _ plugin.MigrationScript = (*addScopeConfigIssueTypeFields)(nil)

type addScopeConfigIssueTypeFields struct{}

type asanaScopeConfig20250212v2 struct {
	IssueTypeRequirement string `gorm:"type:varchar(255)"`
	IssueTypeBug         string `gorm:"type:varchar(255)"`
	IssueTypeIncident    string `gorm:"type:varchar(255)"`
}

func (asanaScopeConfig20250212v2) TableName() string {
	return "_tool_asana_scope_configs"
}

func (*addScopeConfigIssueTypeFields) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	return db.AutoMigrate(&asanaScopeConfig20250212v2{})
}

func (*addScopeConfigIssueTypeFields) Version() uint64 {
	return 20250212000004
}

func (*addScopeConfigIssueTypeFields) Name() string {
	return "asana add issue type fields to scope config"
}
