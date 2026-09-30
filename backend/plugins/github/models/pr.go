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

type GithubPullRequest struct {
	ConnectionId    uint64 `gorm:"primaryKey"`
	GithubId        int    `gorm:"primaryKey"`
	RepoId          int    `gorm:"index"`
	HeadRepoId      int
	Number          int    `gorm:"index"` // This number is used in GET requests to the API associated to reviewers / comments / etc.
	State           string `gorm:"type:varchar(255)"`
	Title           string
	GithubCreatedAt time.Time
	GithubUpdatedAt time.Time `gorm:"index"`
	ClosedAt        *time.Time
	// In order to get the following fields, we need to collect PRs individually from GitHub
	Additions      int
	Deletions      int
	Comments       int
	Commits        int
	ReviewComments int
	IsDraft        bool
	Merged         bool
	MergedAt       *time.Time
	Body           string
	Type           string `gorm:"type:varchar(255)"`
	Component      string `gorm:"type:text"`
	MergeCommitSha string `gorm:"type:varchar(40)"`
	HeadRef        string `gorm:"type:text"`
	BaseRef        string `gorm:"type:text"`
	BaseCommitSha  string `gorm:"type:varchar(255)"`
	HeadCommitSha  string `gorm:"type:varchar(255)"`
	Url            string `gorm:"type:varchar(255)"`
	AuthorName     string `gorm:"type:text"`
	AuthorId       int
	MergedByName   string `gorm:"type:text"`
	MergedById     int
	common.NoPKModel
}

func (GithubPullRequest) TableName() string {
	return "_tool_github_pull_requests"
}
