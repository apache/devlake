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

var _ plugin.MigrationScript = (*addLineCountsToCommitCoverages)(nil)

type addLineCountsToCommitCoverages struct{}

type commitCoverage20260316 struct {
	LinesCovered int
	LinesTotal   int
	LinesMissed  int
	Hits         int
	Partials     int
	Misses       int
}

func (commitCoverage20260316) TableName() string {
	return "_tool_codecov_commit_coverages"
}

func (script *addLineCountsToCommitCoverages) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(basicRes, &commitCoverage20260316{})
}

func (*addLineCountsToCommitCoverages) Version() uint64 {
	return 20260316000000
}

func (*addLineCountsToCommitCoverages) Name() string {
	return "Codecov add line count columns to commit_coverages table"
}
