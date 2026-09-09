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

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/migrationscripts/archived"
	"github.com/apache/devlake/helpers/migrationhelper"
)

type addCursorBugbotReviews struct{}

type cursorConnection20260902 struct {
	HasBugbotReviews bool
}

func (cursorConnection20260902) TableName() string { return "_tool_cursor_connections" }

type cursorBugBotReview20260902 struct {
	ConnectionId      uint64     `gorm:"primaryKey"`
	ScopeId           string     `gorm:"primaryKey;type:varchar(255)"`
	RequestId         string     `gorm:"primaryKey;type:varchar(64)"`
	Timestamp         *time.Time
	Repo              string     `gorm:"type:varchar(255);index"`
	RepoNodeId        string     `gorm:"type:varchar(255)"`
	PrNumber          *int
	CommitSha         string     `gorm:"type:varchar(64)"`
	BugsFound         int
	CostCents         *float64
	DryRun            bool
	PublicationStatus string     `gorm:"type:varchar(20)"`
	archived.NoPKModel
}

func (cursorBugBotReview20260902) TableName() string { return "_tool_cursor_bugbot_reviews" }

type cursorBugBotFinding20260902 struct {
	ConnectionId     uint64 `gorm:"primaryKey"`
	ScopeId          string `gorm:"primaryKey;type:varchar(255)"`
	RequestId        string `gorm:"primaryKey;type:varchar(64)"`
	FindingIndex     int    `gorm:"primaryKey"`
	CommentId        string `gorm:"type:varchar(64)"`
	ResolutionStatus string `gorm:"type:varchar(20)"`
	Severity         string `gorm:"type:varchar(20);index"`
	Title            string `gorm:"type:varchar(500)"`
	Description      string `gorm:"type:text"`
	Locations        string `gorm:"type:text"`
	archived.NoPKModel
}

func (cursorBugBotFinding20260902) TableName() string { return "_tool_cursor_bugbot_findings" }

func (*addCursorBugbotReviews) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&cursorConnection20260902{},
		&cursorBugBotReview20260902{},
		&cursorBugBotFinding20260902{},
	)
}

func (*addCursorBugbotReviews) Version() uint64 { return 20260902120000 }

func (*addCursorBugbotReviews) Name() string {
	return "cursor add bugbot review analytics tables and has_bugbot_reviews column"
}
