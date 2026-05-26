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
	"fmt"
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
	"time"
)

type addFullNameForBuilds struct{}

type jenkinsBuild20221131 struct {
	FullName string `gorm:"primaryKey;type:varchar(255)"` // "path/job name#7" PREPARE TO ADD THIS
}

func (jenkinsBuild20221131) TableName() string {
	return "_tool_jenkins_builds"
}

type jenkinsBuild20221131Before struct {
	archived.NoPKModel
	// collected fields
	ConnectionId      uint64    `gorm:"primaryKey"`
	JobName           string    `gorm:"index;type:varchar(255)"`
	JobPath           string    `gorm:"index;type:varchar(255)"`
	Duration          float64   // build time
	FullName          string    `gorm:"primaryKey;type:varchar(255)"` // "path/job name#7" PREPARE TO ADD THIS
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

func (jenkinsBuild20221131Before) TableName() string {
	return "_tool_jenkins_builds"
}

type jenkinsBuild20221131After struct {
	archived.NoPKModel
	// collected fields
	ConnectionId      uint64    `gorm:"primaryKey"`
	JobName           string    `gorm:"index;type:varchar(255)"`
	JobPath           string    `gorm:"index;type:varchar(255)"`
	Duration          float64   // build time
	FullName          string    `gorm:"primaryKey;type:varchar(255)"` // "path/job name#7" ADD THIS
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

func (jenkinsBuild20221131After) TableName() string {
	return "_tool_jenkins_builds"
}

func (script *addFullNameForBuilds) Up(basicRes context.BasicRes) errors.Error {
	err := migrationhelper.AutoMigrateTables(basicRes, &jenkinsBuild20221131{})
	if err != nil {
		return err
	}
	return migrationhelper.TransformTable(
		basicRes,
		script,
		"_tool_jenkins_builds",
		func(s *jenkinsBuild20221131Before) (*jenkinsBuild20221131After, errors.Error) {
			// copy data
			dst := jenkinsBuild20221131After(*s)
			if s.JobPath != "" {
				dst.FullName = fmt.Sprintf("%s/%s#%d", s.JobPath, s.JobName, s.Number)
			} else {
				dst.FullName = fmt.Sprintf("%s#%d", s.JobName, s.Number)
			}
			return &dst, nil
		},
	)
}

func (*addFullNameForBuilds) Version() uint64 {
	return 20221131000008
}

func (*addFullNameForBuilds) Name() string {
	return "add full name for builds"
}
