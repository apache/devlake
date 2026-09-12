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

package archived

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
)

type CheckmarxoneConnection struct {
	archived.Model
	Name         string `gorm:"type:varchar(100);uniqueIndex"`
	ServerUrl    string `gorm:"type:varchar(255)"`
	Username     string `gorm:"type:varchar(255)"`
	Password     string `gorm:"type:varchar(255)"`
	ClientId     string `gorm:"type:varchar(255)"`
	ClientSecret string `gorm:"type:varchar(255)"`
}

func (CheckmarxoneConnection) TableName() string {
	return "_tool_checkmarxone_connections"
}

type CheckmarxoneProject struct {
	ConnectionId uint64    `gorm:"primaryKey"`
	ProjectId    string    `gorm:"primaryKey;type:varchar(255)"`
	Name         string    `gorm:"type:varchar(255)"`
	Description  string    `gorm:"type:text"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	archived.NoPKModel
}

func (CheckmarxoneProject) TableName() string {
	return "_tool_checkmarxone_projects"
}

type CheckmarxoneFinding struct {
	ConnectionId uint64    `gorm:"primaryKey"`
	ProjectId    string    `gorm:"primaryKey;type:varchar(255)"`
	FindingId    string    `gorm:"primaryKey;type:varchar(255)"`
	Name         string    `gorm:"type:varchar(255)"`
	Severity     string    `gorm:"type:varchar(50)"`
	Status       string    `gorm:"type:varchar(50)"`
	Description  string    `gorm:"type:text"`
	FirstFound   time.Time `json:"firstFound"`
	LastFound    time.Time `json:"lastFound"`
	State        string    `gorm:"type:varchar(50)"`
	Type         string    `gorm:"type:varchar(100)"`
	Count        int
	archived.NoPKModel
}

func (CheckmarxoneFinding) TableName() string {
	return "_tool_checkmarxone_findings"
}