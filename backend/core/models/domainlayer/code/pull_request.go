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
	"fmt"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"time"

	"github.com/apache/devlake/core/models/domainlayer"
)

const (
	OPEN   = "OPEN"
	CLOSED = "CLOSED"
	MERGED = "MERGED"
)

type PullRequest struct {
	domainlayer.DomainEntity
	BaseRepoId     string `gorm:"index"`
	HeadRepoId     string `gorm:"index"`
	Status         string `gorm:"type:varchar(100);comment:open/closed or other"`
	OriginalStatus string `gorm:"type:varchar(100)"`
	Title          string
	Description    string
	Url            string `gorm:"type:varchar(255)"`
	AuthorName     string `gorm:"type:varchar(100)"`
	//User		   domainUser.User `gorm:"foreignKey:AuthorId"`
	AuthorId       string `gorm:"type:varchar(100)"`
	MergedByName   string `gorm:"type:varchar(100)"`
	MergedById     string `gorm:"type:varchar(100)"`
	ParentPrId     string `gorm:"index;type:varchar(100)"`
	PullRequestKey int
	CreatedDate    time.Time
	MergedDate     *time.Time
	ClosedDate     *time.Time
	Type           string `gorm:"type:varchar(100)"`
	Component      string `gorm:"type:text"`
	MergeCommitSha string `gorm:"type:varchar(40)"`
	HeadRef        string `gorm:"type:varchar(255)"`
	BaseRef        string `gorm:"type:varchar(255)"`
	BaseCommitSha  string `gorm:"type:varchar(40)"`
	HeadCommitSha  string `gorm:"type:varchar(40)"`
	Additions      int
	Deletions      int
	IsDraft        bool
}

func (PullRequest) TableName() string {
	return "pull_requests"
}

func (pr PullRequest) ConvertStatusToIncidentStatus() string {
	switch pr.Status {
	case OPEN:
		return ticket.TODO
	case CLOSED:
		return ticket.OTHER
	case MERGED:
		return ticket.DONE
	default:
		return ticket.OTHER
	}
}

func (pr PullRequest) ToIncident() (*ticket.Incident, error) {
	incident := &ticket.Incident{
		DomainEntity:            pr.DomainEntity,
		Url:                     pr.Url,
		IncidentKey:             fmt.Sprintf("%d", pr.PullRequestKey),
		Title:                   pr.Title,
		Description:             pr.Description,
		Status:                  pr.ConvertStatusToIncidentStatus(),
		OriginalStatus:          pr.OriginalStatus,
		ResolutionDate:          pr.MergedDate,
		CreatedDate:             &pr.CreatedDate,
		OriginalEstimateMinutes: nil,
		TimeSpentMinutes:        nil,
		TimeRemainingMinutes:    nil,
		CreatorId:               pr.AuthorId,
		CreatorName:             pr.AuthorName,
		ParentIncidentId:        pr.ParentPrId,
		Priority:                "",
		Severity:                "",
		Urgency:                 "",
		Component:               pr.Component,
		OriginalProject:         "",
		ScopeId:                 pr.BaseRepoId,
		Table:                   "repos",
		AssigneeId:              pr.AuthorId,
		AssigneeName:            pr.AuthorName,
	}

	if pr.MergedDate != nil {
		incident.UpdatedDate = pr.MergedDate
	}
	if incident.UpdatedDate == nil {
		incident.UpdatedDate = pr.ClosedDate
	}

	if pr.MergedDate != nil {
		temp := uint(pr.MergedDate.Sub(pr.CreatedDate).Minutes())
		incident.LeadTimeMinutes = &temp
	}
	return incident, nil
}
