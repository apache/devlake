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
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addTaskTransformationFields)(nil)

type addTaskTransformationFields struct{}

type asanaTask20250212 struct {
	SectionName     string   `gorm:"type:varchar(255)"`
	StdType         string   `gorm:"type:varchar(255)"`
	StdStatus       string   `gorm:"type:varchar(255)"`
	Priority        string   `gorm:"type:varchar(255)"`
	StoryPoint      *float64 `gorm:"type:float8"`
	Severity        string   `gorm:"type:varchar(255)"`
	LeadTimeMinutes *uint
}

func (asanaTask20250212) TableName() string {
	return "_tool_asana_tasks"
}

type asanaScopeConfig20250212 struct {
	// Regex patterns for tag-based type classification (like GitHub)
	IssueTypeRequirement string `gorm:"type:varchar(255)"`
	IssueTypeBug         string `gorm:"type:varchar(255)"`
	IssueTypeIncident    string `gorm:"type:varchar(255)"`
}

func (asanaScopeConfig20250212) TableName() string {
	return "_tool_asana_scope_configs"
}

func (*addTaskTransformationFields) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	// Add transformation fields to tasks table
	if err := db.AutoMigrate(&asanaTask20250212{}); err != nil {
		return err
	}
	// Add transformation config fields to scope_configs table
	return db.AutoMigrate(&asanaScopeConfig20250212{})
}

func (*addTaskTransformationFields) Version() uint64 {
	return 20250212000003
}

func (*addTaskTransformationFields) Name() string {
	return "asana add task transformation fields for issue tracking"
}
