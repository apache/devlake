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
	"time"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

type fixCursorAiCodeSchema struct{}

// cursorAiCodeCommit20260827 uses ingested_at for the API ingestion timestamp so
// archived.NoPKModel created_at/updated_at do not collide during AutoMigrate.
type cursorAiCodeCommit20260827 struct {
	ConnectionId         uint64 `gorm:"primaryKey"`
	ScopeId              string `gorm:"primaryKey;type:varchar(255)"`
	CommitHash           string `gorm:"primaryKey;type:varchar(64)"`
	UserId               string `gorm:"type:varchar(255);index"`
	UserEmail            string `gorm:"type:varchar(255);index"`
	RepoName             string `gorm:"type:varchar(255)"`
	BranchName           string `gorm:"type:varchar(255)"`
	IsPrimaryBranch      *bool
	CommitSource         string `gorm:"type:varchar(20)"`
	TotalLinesAdded      int
	TotalLinesDeleted    int
	TabLinesAdded        int
	TabLinesDeleted      int
	ComposerLinesAdded   int
	ComposerLinesDeleted int
	NonAiLinesAdded      *int
	NonAiLinesDeleted    *int
	CommitTs             *time.Time
	IngestedAt           *time.Time `gorm:"column:ingested_at"`
	archived.NoPKModel
}

func (cursorAiCodeCommit20260827) TableName() string { return "_tool_cursor_ai_code_commits" }

type cursorAiCodeChange20260827 struct {
	ConnectionId      uint64 `gorm:"primaryKey"`
	ScopeId           string `gorm:"primaryKey;type:varchar(255)"`
	ChangeId          string `gorm:"primaryKey;type:varchar(255)"`
	UserId            string `gorm:"type:varchar(255);index"`
	UserEmail         string `gorm:"type:varchar(255);index"`
	Source            string `gorm:"type:varchar(20)"`
	Model             string `gorm:"type:varchar(255)"`
	TotalLinesAdded   int
	TotalLinesDeleted int
	IngestedAt        *time.Time `gorm:"column:ingested_at"`
	Metadata          string     `gorm:"type:text"`
	archived.NoPKModel
}

func (cursorAiCodeChange20260827) TableName() string { return "_tool_cursor_ai_code_changes" }

func (*fixCursorAiCodeSchema) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&cursorAiCodeCommit20260827{},
		&cursorAiCodeChange20260827{},
	)
}

func (*fixCursorAiCodeSchema) Version() uint64 { return 20260827120000 }

func (*fixCursorAiCodeSchema) Name() string {
	return "cursor fix ai_code_commits and ai_code_changes schema"
}
