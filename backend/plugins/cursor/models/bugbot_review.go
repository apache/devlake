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

	"github.com/apache/incubator-devlake/core/models/common"
)

// CursorBugBotReview stores one completed BugBot review from
// GET /analytics/team/bugbot-reviews.
type CursorBugBotReview struct {
	ConnectionId uint64 `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	RequestId    string `gorm:"primaryKey;type:varchar(64)" json:"requestId"`

	Timestamp         *time.Time `json:"timestamp"`
	Repo              string     `gorm:"type:varchar(255);index" json:"repo"`
	RepoNodeId        string     `gorm:"type:varchar(255)" json:"repoNodeId"`
	PrNumber          *int       `json:"prNumber"`
	CommitSha         string     `gorm:"type:varchar(64)" json:"commitSha"`
	BugsFound         int        `json:"bugsFound"`
	CostCents         *float64   `json:"costCents"`
	DryRun            bool       `json:"dryRun"`
	PublicationStatus string     `gorm:"type:varchar(20)" json:"publicationStatus"`

	common.NoPKModel
}

func (CursorBugBotReview) TableName() string {
	return "_tool_cursor_bugbot_reviews"
}
