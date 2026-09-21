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

package tasks

import (
	"time"

	"github.com/apache/devlake/plugins/youtrack/models"
)

// YoutrackOptions are the per-scope options passed to a pipeline task.
type YoutrackOptions struct {
	ConnectionId  uint64 `json:"connectionId" mapstructure:"connectionId,omitempty"`
	ProjectId     string `json:"projectId" mapstructure:"projectId,omitempty"`
	ScopeConfigId uint64 `json:"scopeConfigId" mapstructure:"scopeConfigId,omitempty"`
	// TimeAfter limits collection to data created/updated after this time.
	TimeAfter string `json:"timeAfter" mapstructure:"timeAfter,omitempty"`
}

// YoutrackTaskData is the shared context handed to every YouTrack subtask.
type YoutrackTaskData struct {
	Options    *YoutrackOptions
	Connection *models.YoutrackConnection
	TimeAfter  *time.Time
	// ScopeConfig carries the resolved scope config (field-name slots and
	// type/status mappings). Never nil: PrepareTaskData defaults it to an
	// empty config so subtasks can rely on it.
	ScopeConfig *models.YoutrackScopeConfig
}
