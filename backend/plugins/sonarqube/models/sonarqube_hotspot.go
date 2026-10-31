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
	"github.com/apache/devlake/core/models/common"
)

type SonarqubeHotspot struct {
	ConnectionId             uint64 `gorm:"primaryKey"`
	HotspotKey               string `gorm:"primaryKey"`
	RuleKey                  string `gorm:"type:varchar(255)"`
	Component                string `gorm:"type:text"`
	ProjectKey               string `gorm:"index"`
	Line                     int
	Status                   string `gorm:"type:varchar(100)"`
	Message                  string
	Author                   string `gorm:"type:varchar(100)"`
	Assignee                 string `gorm:"type:varchar(100)"`
	SecurityCategory         string `gorm:"type:varchar(100)"`
	VulnerabilityProbability string `gorm:"type:varchar(100)"`
	CreationDate             *common.Iso8601Time
	UpdateDate               *common.Iso8601Time
	common.NoPKModel
}

func (SonarqubeHotspot) TableName() string {
	return "_tool_sonarqube_hotspots"
}
