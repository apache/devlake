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
	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/migrationhelper"
)

type addRunNameToAzuredevopsBuild struct{}

type azuredevopsBuild20260916 struct {
	RunName string `gorm:"type:varchar(255)"`
}

func (azuredevopsBuild20260916) TableName() string {
	return "_tool_azuredevops_go_builds"
}

func (script *addRunNameToAzuredevopsBuild) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &azuredevopsBuild20260916{})
}

func (*addRunNameToAzuredevopsBuild) Version() uint64 {
	return 20260916000001
}

func (*addRunNameToAzuredevopsBuild) Name() string {
	return "add run name field to _tool_azuredevops_go_builds"
}
