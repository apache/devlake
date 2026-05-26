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
	"fmt"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
)

// TaigaProject represents a Taiga project (scope)
type TaigaProject struct {
	common.Scope     `mapstructure:",squash"`
	ProjectId        uint64  `json:"projectId" gorm:"primaryKey;autoIncrement:false"`
	Name             string  `gorm:"type:varchar(255)" json:"name"`
	Slug             string  `gorm:"type:varchar(255)" json:"slug"`
	Description      string  `gorm:"type:text" json:"description"`
	Url              string  `gorm:"type:varchar(255)" json:"url"`
	IsPrivate        bool    `json:"isPrivate"`
	TotalMilestones  int     `json:"totalMilestones"`
	TotalStoryPoints float64 `json:"totalStoryPoints"`
}

func (p TaigaProject) ScopeId() string {
	return fmt.Sprintf("%d", p.ProjectId)
}

func (p TaigaProject) ScopeName() string {
	return p.Name
}

func (p TaigaProject) ScopeFullName() string {
	return p.Name
}

func (p TaigaProject) ScopeParams() interface{} {
	return &plugin.ApiResourceInput{
		Params: map[string]string{
			"connectionId": fmt.Sprintf("%d", p.ConnectionId),
			"projectId":    fmt.Sprintf("%d", p.ProjectId),
		},
	}
}

func (TaigaProject) TableName() string {
	return "_tool_taiga_projects"
}
