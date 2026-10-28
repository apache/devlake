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

package code

import (
	"time"

	"github.com/apache/devlake/core/models/domainlayer"
)

// AiCommit is the project-scoped domain representation of an AI-assisted commit.
// Converted from _tool_aireview_commits; stamped with project_name so Grafana
// can filter with a single WHERE clause instead of joining through project_mapping.
type AiCommit struct {
	domainlayer.DomainEntity

	ProjectName  string    `gorm:"index;type:varchar(255)"`
	CommitSha    string    `gorm:"index;type:varchar(40)"`
	RepoId       string    `gorm:"index;type:varchar(255)"`
	AiTool       string    `gorm:"type:varchar(100)"`
	AuthorName   string    `gorm:"type:varchar(255)"`
	AuthoredDate time.Time `gorm:"index"`
}

func (AiCommit) TableName() string {
	return "ai_commits"
}
