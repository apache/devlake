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

type boardIssue20260820 struct {
	ProjectName string `gorm:"primaryKey;type:varchar(255);default:''"`
}

func (boardIssue20260820) TableName() string {
	return "board_issues"
}

type addProjectNameToBoardIssue struct{}

func (*addProjectNameToBoardIssue) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &boardIssue20260820{})
}

func (*addProjectNameToBoardIssue) Version() uint64 {
	return 20260820000000
}

func (*addProjectNameToBoardIssue) Name() string {
	return "add project_name to board_issues"
}
