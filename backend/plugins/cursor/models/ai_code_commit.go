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

package models

import (
	"time"

	"github.com/apache/devlake/core/models/common"
)

// CursorAiCodeCommit stores per-commit AI line attribution from
// GET /analytics/ai-code/commits (Enterprise API).
type CursorAiCodeCommit struct {
	ConnectionId uint64 `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	CommitHash   string `gorm:"primaryKey;type:varchar(64)" json:"commitHash"`

	UserId              string    `gorm:"type:varchar(255);index" json:"userId"`
	UserEmail           string    `gorm:"type:varchar(255);index" json:"userEmail"`
	RepoName            string    `gorm:"type:varchar(255)" json:"repoName"`
	BranchName          string    `gorm:"type:varchar(255)" json:"branchName"`
	IsPrimaryBranch     *bool     `json:"isPrimaryBranch"`
	CommitSource        string    `gorm:"type:varchar(20)" json:"commitSource"`
	TotalLinesAdded     int       `json:"totalLinesAdded"`
	TotalLinesDeleted   int       `json:"totalLinesDeleted"`
	TabLinesAdded       int       `json:"tabLinesAdded"`
	TabLinesDeleted     int       `json:"tabLinesDeleted"`
	ComposerLinesAdded  int       `json:"composerLinesAdded"`
	ComposerLinesDeleted int      `json:"composerLinesDeleted"`
	NonAiLinesAdded     *int      `json:"nonAiLinesAdded"`
	NonAiLinesDeleted   *int      `json:"nonAiLinesDeleted"`
	CommitTs   *time.Time `json:"commitTs"`
	IngestedAt *time.Time `gorm:"column:ingested_at" json:"createdAt"`
	Message    string     `gorm:"type:text" json:"message"`

	common.NoPKModel
}

func (CursorAiCodeCommit) TableName() string {
	return "_tool_cursor_ai_code_commits"
}
