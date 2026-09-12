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

type GitlabMergeRequest struct {
	ConnectionId     uint64 `gorm:"primaryKey"`
	GitlabId         int    `gorm:"primaryKey"`
	Iid              int    `gorm:"index"`
	ProjectId        int    `gorm:"index"`
	SourceProjectId  int
	TargetProjectId  int
	State            string `gorm:"type:varchar(255)"`
	Title            string
	WebUrl           string `gorm:"type:varchar(255)"`
	UserNotesCount   int
	WorkInProgress   bool
	IsDetailRequired bool
	SourceBranch     string `gorm:"type:varchar(255)"`
	TargetBranch     string `gorm:"type:varchar(255)"`
	MergeCommitSha   string `gorm:"type:varchar(255)"`
	SquashCommitSha  string `gorm:"type:varchar(255)"`
	DiffHeadSha      string `gorm:"type:varchar(255)"`
	MergedAt         *time.Time
	GitlabCreatedAt  time.Time
	GitlabUpdatedAt  *time.Time
	ClosedAt         *time.Time
	Type             string `gorm:"type:varchar(255)"`
	MergedByUsername string `gorm:"type:varchar(255)"`
	Description      string
	AuthorUsername   string `gorm:"type:varchar(255)"`
	AuthorUserId     int
	Component        string `gorm:"type:text"`
	Component        string `gorm:"type:varchar(255)"`
	Additions        *int   `gorm:"comment:Lines added in this MR diff only"`
	Deletions        *int   `gorm:"comment:Lines deleted in this MR diff only"`
	// CommitUpdatedAt holds the most recent authored date among the commits of this MR.
	// GitLab does not bump `updated_at` for every push (e.g. force-pushes), so this
	// timestamp is used in addition to GitlabUpdatedAt to detect MRs that changed
	// during incremental collection.
	CommitUpdatedAt *time.Time `gorm:"comment:Latest authored date of the commits belonging to this MR"`
	common.NoPKModel
}

func (GitlabMergeRequest) TableName() string {
	return "_tool_gitlab_merge_requests"
}
