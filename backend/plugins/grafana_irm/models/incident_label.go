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

// IncidentLabel is one key/label pair from an incident's Labels[] (see
// grafana_irm_plan.md §3.2/§5) — the only team/service grouping signal the
// API exposes (§4).
type IncidentLabel struct {
	common.NoPKModel
	ConnectionId uint64 `gorm:"primaryKey"`
	IncidentId   string `gorm:"primaryKey;autoIncrement:false"`
	// Key is stored as column `label_key`, not `key`: `key` is a reserved
	// word in MySQL (index syntax) and breaks any raw SQL that references it
	// unquoted — caught live running the e2e test (see grafana_irm_plan.md
	// §12), not by any local check.
	Key   string `gorm:"column:label_key;primaryKey;type:varchar(255)"`
	Label string
}

func (IncidentLabel) TableName() string { return "_tool_grafana_irm_incident_labels" }
