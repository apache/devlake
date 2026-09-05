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
	"github.com/apache/devlake/core/models/common"
	"time"
)

type JiraIssueChangelogs struct {
	common.NoPKModel

	// collected fields
	ConnectionId      uint64 `gorm:"primaryKey"`
	ChangelogId       uint64 `gorm:"primarykey"`
	IssueId           uint64 `gorm:"index"`
	AuthorAccountId   string `gorm:"type:varchar(255)"`
	AuthorDisplayName string `gorm:"type:varchar(255)"`
	AuthorActive      bool
	Created           time.Time  `gorm:"index"`
	IssueUpdated      *time.Time `comment:"corresponding issue.updated time, changelog might need update IFF changelog.issue_updated < issue.updated"`
}

type JiraIssueChangelogItems struct {
	common.NoPKModel

	// collected fields
	ConnectionId     uint64 `gorm:"primaryKey"`
	ChangelogId      uint64 `gorm:"primaryKey"`
	Field            string `gorm:"primaryKey"`
	ItemIndex        uint64 `gorm:"primaryKey"`
	FieldType        string
	FieldId          string
	FromValue        string
	FromString       string
	ToValue          string
	ToString         string
	TmpFromAccountId string
	TmpToAccountId   string
}

func (JiraIssueChangelogs) TableName() string {
	return "_tool_jira_issue_changelogs"
}

func (JiraIssueChangelogItems) TableName() string {
	return "_tool_jira_issue_changelog_items"
}
