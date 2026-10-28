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
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/migrationhelper"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// testRegistryConnection20250101 is a frozen snapshot of models.TestRegistryConnection as of 2025-01-01.
// Do not update this struct — create a new migration instead.
type testRegistryConnection20250101 struct {
	helper.BaseConnection `mapstructure:",squash"`
	CITool                string `gorm:"column:ci_tool;type:varchar(50)"`
	Project               string `gorm:"column:project;type:varchar(200)"`
	GitHubOrganization    string `gorm:"column:github_organization;type:varchar(200)"`
	GitHubToken           string `gorm:"column:github_token;serializer:encdec"`
	QuayOrganization      string `gorm:"column:quay_organization;type:varchar(200)"`
	JUnitRegex            string `gorm:"column:junit_regex;type:varchar(500)"`
}

func (testRegistryConnection20250101) TableName() string {
	return "_tool_testregistry_connections"
}

type addInitTables struct{}

func (u *addInitTables) Up(baseRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		baseRes,
		&testRegistryConnection20250101{},
	)
}

func (*addInitTables) Version() uint64 {
	return 20250101000001
}

func (*addInitTables) Name() string {
	return "testregistry init schemas"
}
