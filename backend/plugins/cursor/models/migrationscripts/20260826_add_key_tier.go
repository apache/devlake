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
	"github.com/apache/incubator-devlake/plugins/cursor/models"
)

type addCursorKeyTier struct{}

type cursorConnection20260826 struct {
	KeyTier string `gorm:"type:varchar(20)"`
}

func (cursorConnection20260826) TableName() string { return "_tool_cursor_connections" }

type cursorAiCodeCommit20260826 struct {
	ConnectionId         uint64     `gorm:"primaryKey"`
	ScopeId              string     `gorm:"primaryKey;type:varchar(255)"`
	CommitHash           string     `gorm:"primaryKey;type:varchar(64)"`
	UserId               string     `gorm:"type:varchar(255);index"`
	UserEmail            string     `gorm:"type:varchar(255);index"`
	RepoName             string     `gorm:"type:varchar(255)"`
	BranchName           string     `gorm:"type:varchar(255)"`
	IsPrimaryBranch      *bool
	CommitSource         string     `gorm:"type:varchar(20)"`
	TotalLinesAdded      int
	TotalLinesDeleted    int
	TabLinesAdded        int
	TabLinesDeleted      int
	ComposerLinesAdded   int
	ComposerLinesDeleted int
	NonAiLinesAdded      *int
	NonAiLinesDeleted    *int
	CommitTs             *time.Time
	CreatedAt            *time.Time
	archived.NoPKModel
}

func (cursorAiCodeCommit20260826) TableName() string { return "_tool_cursor_ai_code_commits" }

type cursorAiCodeChange20260826 struct {
	ConnectionId      uint64     `gorm:"primaryKey"`
	ScopeId           string     `gorm:"primaryKey;type:varchar(255)"`
	ChangeId          string     `gorm:"primaryKey;type:varchar(255)"`
	UserId            string     `gorm:"type:varchar(255);index"`
	UserEmail         string     `gorm:"type:varchar(255);index"`
	Source            string     `gorm:"type:varchar(20)"`
	Model             string     `gorm:"type:varchar(255)"`
	TotalLinesAdded   int
	TotalLinesDeleted int
	CreatedAt         *time.Time
	Metadata          string     `gorm:"type:text"`
	archived.NoPKModel
}

func (cursorAiCodeChange20260826) TableName() string { return "_tool_cursor_ai_code_changes" }

func (*addCursorKeyTier) Up(basicRes context.BasicRes) errors.Error {
	if err := migrationhelper.AutoMigrateTables(
		basicRes,
		&cursorConnection20260826{},
		&cursorAiCodeCommit20260826{},
		&cursorAiCodeChange20260826{},
	); err != nil {
		return err
	}
	return basicRes.GetDal().Exec(
		"UPDATE _tool_cursor_connections SET key_tier = ? WHERE key_tier IS NULL OR key_tier = ''",
		models.KeyTierTeam,
	)
}

func (*addCursorKeyTier) Version() uint64 { return 20260826120000 }

func (*addCursorKeyTier) Name() string {
	return "cursor add key_tier, ai_code_commits, and ai_code_changes tables"
}
