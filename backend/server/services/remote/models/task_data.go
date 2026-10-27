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

	coreModels "github.com/apache/devlake/core/models"
)

type RemoteSyncPolicy struct {
	TimeAfter *time.Time `json:"timeAfter,omitempty"`
}

type RemotePluginTaskData struct {
	DbUrl       string                 `json:"db_url"`
	Scope       interface{}            `json:"scope"`
	Connection  interface{}            `json:"connection"`
	ScopeConfig interface{}            `json:"scope_config"`
	Options     map[string]interface{} `json:"options"`
	SyncPolicy  *RemoteSyncPolicy      `json:"sync_policy,omitempty"`
}

func ApplySyncPolicy(data *RemotePluginTaskData, syncPolicy *coreModels.SyncPolicy) {
	if data == nil || syncPolicy == nil {
		return
	}
	if data.Options == nil {
		data.Options = make(map[string]interface{})
	}
	if syncPolicy.FullSync {
		data.Options["incremental"] = false
	}
	if syncPolicy.TimeAfter != nil {
		data.SyncPolicy = &RemoteSyncPolicy{TimeAfter: syncPolicy.TimeAfter}
	}
}
