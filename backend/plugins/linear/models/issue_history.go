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

// LinearIssueHistory is a single entry in a Linear issue's history, used to
// build domain-layer changelogs and derive lead/cycle time.
type LinearIssueHistory struct {
	ConnectionId  uint64    `gorm:"primaryKey"`
	Id            string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	IssueId       string    `gorm:"index;type:varchar(255)" json:"issueId"`
	ActorId       string    `gorm:"type:varchar(255)" json:"actorId"`
	FromStateId   string    `gorm:"type:varchar(255)" json:"fromStateId"`
	FromStateName string    `gorm:"type:varchar(255)" json:"fromStateName"`
	ToStateId     string    `gorm:"type:varchar(255)" json:"toStateId"`
	ToStateName   string    `gorm:"type:varchar(255)" json:"toStateName"`
	CreatedAt     time.Time `gorm:"index" json:"createdAt"`
	common.NoPKModel
}

func (LinearIssueHistory) TableName() string {
	return "_tool_linear_issue_history"
}
