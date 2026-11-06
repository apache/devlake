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

	"github.com/apache/devlake/core/models/common"
)

type AzuredevopsIteration struct {
	common.NoPKModel

	ConnectionId uint64 `gorm:"primaryKey"`
	Identifier   string `gorm:"primaryKey;type:varchar(255)"`
	NodeId       int
	OrgId        string `gorm:"type:varchar(255)"`
	ProjectId    string `gorm:"type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	Path         string `gorm:"type:varchar(500)"`
	PathKey      string `gorm:"type:varchar(500)"`
	HasChildren  bool
	StartedDate  *time.Time
	EndedDate    *time.Time
	TimeFrame    string `gorm:"type:varchar(50)"`
}

func (AzuredevopsIteration) TableName() string {
	return "_tool_azuredevops_go_iterations"
}
