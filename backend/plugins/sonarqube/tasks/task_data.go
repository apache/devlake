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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

type SonarqubeOptions struct {
	// options means some custom params required by plugin running.
	// Such As How many rows do your want
	// You can use it in subtasks, and you need to pass it to main.go and pipelines.
	ConnectionId uint64 `json:"connectionId" mapstructure:"connectionId"`
	ProjectKey   string `json:"projectKey" mapstructure:"projectKey"`
}

type SonarqubeTaskData struct {
	Options          *SonarqubeOptions
	ApiClient        *api.ApiAsyncClient
	LastAnalysisDate *time.Time
	TaskStartTime    time.Time
	IsCloud          bool
}

func DecodeAndValidateTaskOptions(options map[string]interface{}) (*SonarqubeOptions, errors.Error) {
	var op SonarqubeOptions
	if err := api.Decode(options, &op, nil); err != nil {
		return nil, err
	}
	if op.ConnectionId == 0 {
		return nil, errors.Default.New("connectionId is invalid")
	}
	return &op, nil
}
