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
)

// IncidentAssignment is one real, filled role assignment on an incident.
// IncidentMembership.Assignments on the wire is a fixed-size array of role
// *slots*, most of them empty placeholders with User.UserID == "" (verified
// live, see grafana_irm_plan.md §3.4/§9) — only entries with a real assigned
// user are ever extracted into this table.
type IncidentAssignment struct {
	common.NoPKModel
	ConnectionId uint64 `gorm:"primaryKey"`
	IncidentId   string `gorm:"primaryKey;autoIncrement:false"`
	UserId       string `gorm:"primaryKey;type:varchar(255)"`
	RoleName     string `gorm:"primaryKey;type:varchar(255)"`
	UserName     string
}

func (IncidentAssignment) TableName() string { return "_tool_grafana_irm_incident_assignments" }
