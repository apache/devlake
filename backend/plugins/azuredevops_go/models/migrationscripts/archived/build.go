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
	"github.com/apache/devlake/core/models/migrationscripts/archived"
	"time"
)

type AzuredevopsBuild struct {
	archived.NoPKModel

	ConnectionId  uint64 `gorm:"primaryKey"`
	AzuredevopsId int    `gorm:"primaryKey"`
	RepositoryId  string `gorm:"type:varchar(255)"`
	Name          string `gorm:"type:varchar(100)"`
	RunName       string `gorm:"type:varchar(255)"`
	Status        string `gorm:"type:varchar(255)"`
	Result        string `gorm:"type:varchar(255)"`
	SourceBranch  string `gorm:"type:varchar(255)"`
	SourceVersion string `gorm:"type:varchar(255)"`
	// Tags is a string version of the APIs tags array that helps to identify
	// devops.CICDPipeline's environment and type.
	Tags       string
	QueueTime  *time.Time
	StartTime  *time.Time
	FinishTime *time.Time
}

func (AzuredevopsBuild) TableName() string {
	return "_tool_azuredevops_go_builds"
}
