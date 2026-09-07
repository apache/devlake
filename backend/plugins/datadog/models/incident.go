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

type Incident struct {
	common.NoPKModel
	ConnectionId   uint64 `gorm:"primaryKey"`
	Id             string `gorm:"primaryKey;autoIncrement:false"`
	PublicId       int64  `gorm:"index"`
	IncidentTypeId string `gorm:"index"`
	Title          string
	Url            string
	State          string
	Severity       string
	Component      string
	// CustomFields keeps Datadog's `fields` object verbatim, so a scope
	// config change can remap severity or component without re-collecting.
	CustomFields     string `gorm:"type:text"`
	IsTest           bool
	CustomerImpacted bool
	CreatedDate      time.Time
	DeclaredDate     *time.Time
	DetectedDate     *time.Time
	ResolvedDate     *time.Time
	ModifiedDate     *time.Time
	// Datadog's own durations, in seconds. TimeToRepair is the restore
	// time; TimeToResolve includes post-incident work.
	TimeToDetect  *int64
	TimeToRepair  *int64
	TimeToResolve *int64
}

func (Incident) TableName() string { return "_tool_datadog_incidents" }
