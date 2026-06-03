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
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

type expandPrimaryView struct{}

type JenkinsJob20260602 struct {
	PrimaryView string `gorm:"type:text"`
}

func (JenkinsJob20260602) TableName() string {
	return "_tool_jenkins_jobs"
}

func (u *expandPrimaryView) Up(baseRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(baseRes, &JenkinsJob20260602{})
}

func (*expandPrimaryView) Version() uint64 {
	return 20260602000000
}

func (*expandPrimaryView) Name() string {
	return "expand primary_view column to text"
}