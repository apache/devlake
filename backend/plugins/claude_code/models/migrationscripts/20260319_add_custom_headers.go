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

package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

// addClaudeCodeCustomHeaders adds the custom_headers column to the connections table.
type addClaudeCodeCustomHeaders struct{}

func (script *addClaudeCodeCustomHeaders) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&claudeCodeConnection20260319{},
	)
}

type claudeCodeConnection20260319 struct {
	archived.Model
	Name             string `gorm:"type:varchar(100);uniqueIndex" json:"name"`
	Endpoint         string `gorm:"type:varchar(255)" json:"endpoint"`
	Proxy            string `gorm:"type:varchar(255)" json:"proxy"`
	RateLimitPerHour int    `json:"rateLimitPerHour"`
	Token            string `json:"token"`
	Organization     string `gorm:"type:varchar(255)" json:"organization"`
	CustomHeaders    string `gorm:"type:json" json:"customHeaders"`
}

func (claudeCodeConnection20260319) TableName() string {
	return "_tool_claude_code_connections"
}

func (*addClaudeCodeCustomHeaders) Version() uint64 {
	return 20260319000000
}

func (*addClaudeCodeCustomHeaders) Name() string {
	return "claude-code add custom headers to connections"
}
