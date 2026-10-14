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

type JiraBoardIssue20260710 struct {
	ProjectName string `gorm:"primaryKey;type:varchar(255);default:''"`
}

func (JiraBoardIssue20260710) TableName() string {
	return "_tool_jira_board_issues"
}

type addProjectNameToBoardIssue struct{}

func (script *addProjectNameToBoardIssue) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &JiraBoardIssue20260710{})
}

func (*addProjectNameToBoardIssue) Version() uint64 {
	return 20260710000000
}

func (*addProjectNameToBoardIssue) Name() string {
	return "add project_name to _tool_jira_board_issues"
}
