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
	"github.com/apache/devlake/core/models/migrationscripts/archived"
	"github.com/apache/devlake/helpers/migrationhelper"
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

// Snapshot of the tool tables. Migration scripts must not import the live models
// package; later model edits would change the behavior of a script that already shipped.
type cursorAiCodeConversation20260917 struct {
	ConnectionId   uint64 `gorm:"primaryKey"`
	ScopeId        string `gorm:"primaryKey;type:varchar(255)"`
	ConversationId string `gorm:"primaryKey;type:varchar(64)"`

	Title          string `gorm:"type:varchar(512)"`
	Tldr           string `gorm:"type:text"`
	Overview       string `gorm:"type:text"`
	SummaryBullets string `gorm:"type:text"`

	archived.NoPKModel
}

func (cursorAiCodeConversation20260917) TableName() string {
	return "_tool_cursor_ai_code_conversations"
}

type cursorAiCodeRangeAnnotation20260917 struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	ScopeId      string `gorm:"primaryKey;type:varchar(255)"`
	AnnotationId string `gorm:"primaryKey;type:varchar(64)"`

	CommitHash     string `gorm:"type:varchar(64);index"`
	FilePath       string `gorm:"type:varchar(1024)"`
	ConversationId string `gorm:"type:varchar(64);index"`
	RangeStart     int
	RangeEnd       int
	Model          string `gorm:"type:varchar(255)"`
	OperationType  string `gorm:"type:varchar(64)"`
	LineCount      int

	archived.NoPKModel
}

func (cursorAiCodeRangeAnnotation20260917) TableName() string {
	return "_tool_cursor_ai_code_range_annotations"
}

func (*addCursorAiCodeCommitDetails) Up(basicRes context.BasicRes) errors.Error {
	// A failed create of the earlier wide primary key can leave a table that
	// AutoMigrate cannot alter. Drop only that broken shape.
	db := basicRes.GetDal()
	table := cursorAiCodeRangeAnnotation20260917{}.TableName()
	if db.HasTable(table) && !db.HasColumn(table, "annotation_id") {
		if err := db.DropTables(table); err != nil {
			return err
		}
	}
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&cursorConnection20260917{},
		&cursorAiCodeCommit20260917{},
		&cursorAiCodeConversation20260917{},
		&cursorAiCodeRangeAnnotation20260917{},
	)
}

func (*addCursorAiCodeCommitDetails) Version() uint64 { return 20260917120000 }

func (*addCursorAiCodeCommitDetails) Name() string {
	return "cursor add ai code commit details tables and has_ai_code_commit_details column"
}
