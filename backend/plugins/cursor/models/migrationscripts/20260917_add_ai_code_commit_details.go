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
	"github.com/apache/devlake/plugins/cursor/models"
)

type addCursorAiCodeCommitDetails struct{}

type cursorConnection20260917 struct {
	HasAiCodeCommitDetails bool
}

func (cursorConnection20260917) TableName() string { return "_tool_cursor_connections" }

type cursorAiCodeCommit20260917 struct {
	Message string `gorm:"type:text"`
}

func (cursorAiCodeCommit20260917) TableName() string { return "_tool_cursor_ai_code_commits" }

func (*addCursorAiCodeCommitDetails) Up(basicRes context.BasicRes) errors.Error {
	// Drop on retry if a prior attempt failed while creating the wide composite PK.
	if basicRes.GetDal().HasTable(models.CursorAiCodeRangeAnnotation{}.TableName()) {
		if err := basicRes.GetDal().DropTables(models.CursorAiCodeRangeAnnotation{}.TableName()); err != nil {
			return errors.Convert(err)
		}
	}
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&cursorConnection20260917{},
		&cursorAiCodeCommit20260917{},
		&models.CursorAiCodeConversation{},
		&models.CursorAiCodeRangeAnnotation{},
	)
}

func (*addCursorAiCodeCommitDetails) Version() uint64 { return 20260917120000 }

func (*addCursorAiCodeCommitDetails) Name() string {
	return "cursor add ai code commit details tables and has_ai_code_commit_details column"
}
