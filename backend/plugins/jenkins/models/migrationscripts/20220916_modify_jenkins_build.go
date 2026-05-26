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
	"strings"
	"time"
)

type modifyJenkinsBuild struct{}

type jenkinsBuild20220916Before struct {
	archived.NoPKModel
	// collected fields
	ConnectionId      uint64    `gorm:"primaryKey"`
	JobName           string    `gorm:"primaryKey;type:varchar(255)"`
	Duration          float64   // build time
	FullDisplayName   string    `gorm:"primaryKey;type:varchar(255)"` // "#7"
	EstimatedDuration float64   // EstimatedDuration
	Number            int64     `gorm:"primaryKey"`
	Result            string    // Result
	Timestamp         int64     // start time
	StartTime         time.Time // converted by timestamp
	Type              string    `gorm:"index;type:varchar(255)"`
	Class             string    `gorm:"index;type:varchar(255)" `
	TriggeredBy       string    `gorm:"type:varchar(255)"`
	Building          bool
	HasStages         bool
}

func (jenkinsBuild20220916Before) TableName() string {
	return "_tool_jenkins_builds"
}

type jenkinsBuild20220916After struct {
	archived.NoPKModel
	// collected fields
	ConnectionId      uint64    `gorm:"primaryKey"`
	JobName           string    `gorm:"index;type:varchar(255)"`
	Duration          float64   // build time
	FullDisplayName   string    `gorm:"primaryKey;type:varchar(255)"` // "#7"
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

func (jenkinsBuild20220916After) TableName() string {
	return "_tool_jenkins_builds"
}

func (script *modifyJenkinsBuild) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	err := db.RenameTable("_tool_jenkins_build_repos", "_tool_jenkins_build_commits")
	if err != nil {
		return err
	}
	err = db.RenameColumn("_tool_jenkins_builds", "display_name", "full_display_name")
	if err != nil {
		return err
	}
	err = migrationhelper.TransformTable(
		basicRes,
		script,
		"_tool_jenkins_builds",
		func(s *jenkinsBuild20220916Before) (*jenkinsBuild20220916After, errors.Error) {
			// copy data
			dst := jenkinsBuild20220916After(*s)
			if strings.Contains(s.FullDisplayName, s.JobName) {
				dst.FullDisplayName = s.FullDisplayName
			} else {
				dst.FullDisplayName = fmt.Sprintf("%s %s", s.JobName, s.FullDisplayName)
			}
			return &dst, nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (*modifyJenkinsBuild) Version() uint64 {
	return 20220916231237
}

func (*modifyJenkinsBuild) Name() string {
	return "Jenkins modify build primary key"
}
