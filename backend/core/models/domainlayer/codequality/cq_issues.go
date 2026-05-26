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

package codequality

import (
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
)

type CqIssue struct {
	domainlayer.DomainEntity
	Rule                     string `gorm:"type:varchar(255)"`
	Severity                 string `gorm:"type:varchar(100)"`
	Component                string
	ProjectKey               string `gorm:"index;type:varchar(500)"` //domain project key
	Line                     int
	Status                   string `gorm:"type:varchar(20)"`
	Message                  string
	Debt                     int
	Effort                   int
	CommitAuthorEmail        string `json:"author" gorm:"type:varchar(255)"`
	Assignee                 string `json:"assignee" gorm:"type:varchar(255)"`
	Hash                     string `gorm:"type:varchar(100)"`
	Tags                     string
	Type                     string `gorm:"type:varchar(100)"`
	Scope                    string `gorm:"type:varchar(255)"`
	StartLine                int    `json:"startLine"`
	EndLine                  int    `json:"endLine"`
	StartOffset              int    `json:"startOffset"`
	EndOffset                int    `json:"endOffset"`
	VulnerabilityProbability string `gorm:"type:varchar(100)"`
	SecurityCategory         string `gorm:"type:varchar(100)"`
	CreatedDate              *common.Iso8601Time
	UpdatedDate              *common.Iso8601Time
}

func (CqIssue) TableName() string {
	return "cq_issues"
}

type CqIssueImpact struct {
	common.NoPKModel
	CqIssueId       string `gorm:"primaryKey;type:varchar(255)"`
	SoftwareQuality string `gorm:"primaryKey;type:varchar(255)"`
	Severity        string `gorm:"type:varchar(100)"`
}

func (CqIssueImpact) TableName() string {
	return "cq_issue_impacts"
}
