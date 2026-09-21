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
)

type jiraOAuth20260907 struct {
	ClientId     string `gorm:"type:varchar(255)"`
	ClientSecret string `gorm:"type:text"`
	CloudId      string `gorm:"type:varchar(255)"`
}

func (jiraOAuth20260907) TableName() string {
	return "_tool_jira_connections"
}

type addJiraOAuth20260907 struct{}

func (script *addJiraOAuth20260907) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &jiraOAuth20260907{})
}

func (*addJiraOAuth20260907) Version() uint64 {
	return 20260907120000
}

func (*addJiraOAuth20260907) Name() string {
	return "add oauth2 client credentials to _tool_jira_connections"
}
