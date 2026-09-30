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

type ensureCursorAiCodeRangeAnnotations struct{}

// Databases that already recorded 20260917120000 skip that script. If that run
// did not leave the tool tables behind, this script creates them.
type cursorAiCodeConversation20260930 struct {
	ConnectionId   uint64 `gorm:"primaryKey"`
	ScopeId        string `gorm:"primaryKey;type:varchar(255)"`
	ConversationId string `gorm:"primaryKey;type:varchar(64)"`

	Title          string `gorm:"type:varchar(512)"`
	Tldr           string `gorm:"type:text"`
	Overview       string `gorm:"type:text"`
	SummaryBullets string `gorm:"type:text"`

	archived.NoPKModel
}

func (cursorAiCodeConversation20260930) TableName() string {
	return "_tool_cursor_ai_code_conversations"
}

type cursorAiCodeRangeAnnotation20260930 struct {
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

func (cursorAiCodeRangeAnnotation20260930) TableName() string {
	return "_tool_cursor_ai_code_range_annotations"
}

func (*ensureCursorAiCodeRangeAnnotations) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&cursorAiCodeConversation20260930{},
		&cursorAiCodeRangeAnnotation20260930{},
	)
}

func (*ensureCursorAiCodeRangeAnnotations) Version() uint64 { return 20260930120000 }

func (*ensureCursorAiCodeRangeAnnotations) Name() string {
	return "cursor ensure ai code range annotation and conversation tables"
}
