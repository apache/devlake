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

	"github.com/apache/incubator-devlake/core/models/common"
)

type CheckmarxoneFinding struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ProjectId    string    `gorm:"primaryKey;type:varchar(255)" json:"projectId"`
	FindingId    string    `gorm:"primaryKey;type:varchar(255)" json:"findingId"`
	Name         string    `gorm:"type:varchar(255)" json:"name"`
	Severity     string    `gorm:"type:varchar(50)" json:"severity"`
	Status       string    `gorm:"type:varchar(50)" json:"status"`
	Description  string    `gorm:"type:text" json:"description"`
	FirstFound   time.Time `json:"firstFound"`
	LastFound    time.Time `json:"lastFound"`
	State        string    `gorm:"type:varchar(50)" json:"state"`
	Type         string    `gorm:"type:varchar(100)" json:"type"`
	Count        int       `json:"count"`
	common.NoPKModel
}

func (CheckmarxoneFinding) TableName() string {
	return "_tool_checkmarxone_findings"
}