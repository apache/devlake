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

type BitbucketPullRequest struct {
	ConnectionId       uint64 `gorm:"primaryKey"`
	RepoId             string `gorm:"primaryKey;type:varchar(255)"`
	BitbucketId        int    `gorm:"primaryKey"`
	Number             int    `gorm:"index"` // This number is used in GET requests to the API associated to reviewers / comments / etc.
	BaseRepoId         string
	HeadRepoId         string
	State              string `gorm:"type:varchar(255)"`
	Title              string
	Description        string
	BitbucketCreatedAt time.Time
	BitbucketUpdatedAt time.Time `gorm:"index"`
	ClosedAt           *time.Time
	CommentCount       int
	Commits            int
	MergedAt           *time.Time
	Body               string
	Type               string `gorm:"type:varchar(255)"`
	Component          string `gorm:"type:text"`
	MergeCommitSha     string `gorm:"type:varchar(40)"`
	HeadRef            string `gorm:"type:varchar(255)"`
	BaseRef            string `gorm:"type:varchar(255)"`
	BaseCommitSha      string `gorm:"type:varchar(255)"`
	HeadCommitSha      string `gorm:"type:varchar(255)"`
	Url                string `gorm:"type:varchar(255)"`
	AuthorName         string `gorm:"type:varchar(255)"`
	AuthorId           string `gorm:"type:varchar(255)"`
	MergedByName       string `gorm:"type:varchar(100)"`
	MergedById         string `gorm:"type:varchar(100)"`
	common.NoPKModel
}

func (BitbucketPullRequest) TableName() string {
	return "_tool_bitbucket_pull_requests"
}
