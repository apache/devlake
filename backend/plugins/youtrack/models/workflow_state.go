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

// YoutrackWorkflowState is one value of a project's State bundle (tool
// layer). The grain is per-project — PK (ConnectionId, ProjectId, Id) —
// matching the collection path (`/api/admin/projects/{id}/customFields`);
// shared bundles simply yield one row per project. Only states are persisted
// (type-bundle values are read live by the mapping widget) because
// `isResolved` feeds the zero-config DONE status default.
type YoutrackWorkflowState struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	ProjectId    string `gorm:"primaryKey;type:varchar(255)" json:"projectId"`
	Id           string `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name         string `gorm:"type:varchar(255)" json:"name"`
	IsResolved   bool   `json:"isResolved"`
	Ordinal      int    `json:"ordinal"`
	Archived     bool   `json:"archived"`
	common.NoPKModel
}

func (YoutrackWorkflowState) TableName() string {
	return "_tool_youtrack_workflow_states"
}
