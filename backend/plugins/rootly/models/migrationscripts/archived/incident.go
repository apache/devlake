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

package archived

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
)

type Incident struct {
	archived.NoPKModel
	ConnectionId      uint64 `gorm:"primaryKey"`
	Id                string `gorm:"primaryKey;autoIncrement:false"`
	Number            int
	ServiceId         string
	Url               string
	Title             string
	Summary           string
	Status            string
	Severity          string
	Urgency           string
	StartedDate       time.Time
	AcknowledgedDate  *time.Time
	MitigatedDate     *time.Time
	ResolvedDate      *time.Time
	UpdatedDate       time.Time
	CreatorUserId     string
	StartedByUserId   string
	MitigatedByUserId string
	ResolvedByUserId  string
	ClosedByUserId    string
}

func (Incident) TableName() string {
	return "_tool_rootly_incidents"
}
