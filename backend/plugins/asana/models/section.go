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

type AsanaSection struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Gid          string `gorm:"primaryKey;type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	ResourceType string `gorm:"type:varchar(32)"`
	ProjectGid   string `gorm:"type:varchar(255);index"`
	common.NoPKModel
}

func (AsanaSection) TableName() string {
	return "_tool_asana_sections"
}
