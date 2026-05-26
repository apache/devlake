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
	"time"
)

// JenkinsBuild db entity for jenkins build
type JenkinsBuild struct {
	common.NoPKModel
	// collected fields
	ConnectionId      uint64    `gorm:"primaryKey"`
	JobName           string    `gorm:"index;type:varchar(255)"`
	JobPath           string    `gorm:"index;type:varchar(255)"`
	Duration          float64   // build time
	FullName          string    `gorm:"primaryKey;type:varchar(255)"` // "path/job name#7"
	EstimatedDuration float64   // EstimatedDuration
	Number            int64     `gorm:"index"`
	Result            string    // Result
	Timestamp         int64     // start time
	StartTime         time.Time // converted by timestamp
	Type              string    `gorm:"index;type:varchar(255)"`
	Class             string    `gorm:"index;type:varchar(255)" `
	TriggeredBy       string    `gorm:"type:varchar(255)"`
	Building          bool
	HasStages         bool
}

func (JenkinsBuild) TableName() string {
	return "_tool_jenkins_builds"
}
