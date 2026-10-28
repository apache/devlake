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
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*addCoverageToFlags)(nil)

type addCoverageToFlags struct{}

type codecovFlag20251206 struct {
	Coverage *float64
}

func (codecovFlag20251206) TableName() string {
	return "_tool_codecov_flags"
}

func (script *addCoverageToFlags) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &codecovFlag20251206{})
}

func (*addCoverageToFlags) Version() uint64 {
	return 20251206100000
}

func (*addCoverageToFlags) Name() string {
	return "Codecov add coverage column to flags table"
}
