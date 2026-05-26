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
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

func CreateRawDataSubTaskArgs(taskCtx plugin.SubTaskContext, rawTable string) (*api.RawDataSubTaskArgs, *AsanaTaskData) {
	data := taskCtx.GetData().(*AsanaTaskData)
	params := models.AsanaApiParams{
		ConnectionId: data.Options.ConnectionId,
		ProjectId:    data.Options.ProjectId,
	}
	return &api.RawDataSubTaskArgs{
		Ctx:    taskCtx,
		Params: params,
		Table:  rawTable,
	}, data
}

type AsanaOptions struct {
	ConnectionId  uint64 `json:"connectionId" mapstructure:"connectionId"`
	ProjectId     string `json:"projectId" mapstructure:"projectId"`
	ScopeConfigId uint64 `json:"scopeConfigId" mapstructure:"scopeConfigId,omitempty"`
}

type AsanaTaskData struct {
	Options   *AsanaOptions
	ApiClient *api.ApiAsyncClient
	Project   *models.AsanaProject
}
