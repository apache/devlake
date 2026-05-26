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
	"github.com/apache/incubator-devlake/core/models/common"
)

type AsanaTag struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Gid          string `gorm:"primaryKey;type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	ResourceType string `gorm:"type:varchar(32)"`
	Color        string `gorm:"type:varchar(32)"`
	Notes        string `gorm:"type:text"`
	WorkspaceGid string `gorm:"type:varchar(255);index"`
	PermalinkUrl string `gorm:"type:varchar(512)"`
	common.NoPKModel
}

func (AsanaTag) TableName() string {
	return "_tool_asana_tags"
}

// AsanaTaskTag is a many-to-many relationship between tasks and tags
type AsanaTaskTag struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	TaskGid      string `gorm:"primaryKey;type:varchar(255)"`
	TagGid       string `gorm:"primaryKey;type:varchar(255)"`
	common.NoPKModel
}

func (AsanaTaskTag) TableName() string {
	return "_tool_asana_task_tags"
}
