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

// Incident is the tool-layer row for a Grafana IRM incident. Status is the
// raw wire value ("active"/"resolved" as verified live, see
// grafana_irm_plan.md §3.4) and is what the collector's refresh pass
// (tasks/incidents_collector.go) filters on. CreatedDate/ResolvedDate use
// CreatedTime/ClosedTime rather than IncidentStart/IncidentEnd per the
// resolved open decision in §9 (they mirror each other exactly under normal
// API-driven resolution, so either pair works).
type Incident struct {
	common.NoPKModel
	ConnectionId uint64 `gorm:"primaryKey"`
	Id           string `gorm:"primaryKey;autoIncrement:false"`
	Title        string
	// Url is the incident's absolute overview URL. OverviewURL on the wire
	// is relative (verified live, see §3.4) and must be prefixed with the
	// connection's endpoint by the extractor before being stored here.
	Url          string
	Status       string `gorm:"index;type:varchar(255)"`
	Severity     string
	CreatedDate  time.Time
	UpdatedDate  time.Time
	ResolvedDate *time.Time
}

func (Incident) TableName() string { return "_tool_grafana_irm_incidents" }
