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

	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

// LinearOptions are the per-scope options passed to a pipeline task.
type LinearOptions struct {
	ConnectionId  uint64 `json:"connectionId" mapstructure:"connectionId,omitempty"`
	TeamId        string `json:"teamId" mapstructure:"teamId,omitempty"`
	ScopeConfigId uint64 `json:"scopeConfigId" mapstructure:"scopeConfigId,omitempty"`
	// TimeAfter limits collection to data created/updated after this time.
	TimeAfter string `json:"timeAfter" mapstructure:"timeAfter,omitempty"`
}

// LinearTaskData is the shared context handed to every Linear subtask.
type LinearTaskData struct {
	Options       *LinearOptions
	GraphqlClient *api.GraphqlAsyncClient
	TimeAfter     *time.Time
}

type LinearApiParams models.LinearApiParams
