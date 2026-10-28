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
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

type AgentReadyConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	GitHubConnectionId    uint64 `mapstructure:"githubConnectionId" json:"githubConnectionId" gorm:"column:github_connection_id;not null"`
	SubmissionsRepo       string `mapstructure:"submissionsRepo" json:"submissionsRepo" validate:"required" gorm:"column:submissions_repo;type:varchar(255);not null"`
	SubmissionsPath       string `mapstructure:"submissionsPath" json:"submissionsPath" gorm:"column:submissions_path;type:varchar(255);default:submissions"`
	Branch                string `mapstructure:"branch" json:"branch" gorm:"column:branch;type:varchar(255)"`
}

func (AgentReadyConnection) TableName() string {
	return "_tool_agentready_connections"
}

func (c AgentReadyConnection) Sanitize() AgentReadyConnection {
	return c
}
