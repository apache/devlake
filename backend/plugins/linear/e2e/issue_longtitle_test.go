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

package e2e

import (
	"testing"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/linear/impl"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/apache/incubator-devlake/plugins/linear/tasks"
	"github.com/stretchr/testify/assert"
)

// TestLinearIssueLongTitle guards against truncation/insert failure for long
// issue titles and URLs. Linear titles can exceed 255 chars (and the issue URL
// embeds a title slug), which overflowed the old varchar(255) columns and
// failed extraction with "Data too long for column 'title'". The columns are
// now untyped (longtext), matching the domain issues.title and jira's tool
// summary.
func TestLinearIssueLongTitle(t *testing.T) {
	var linear impl.Linear
	dataflowTester := e2ehelper.NewDataFlowTester(t, "linear", linear)

	taskData := &tasks.LinearTaskData{
		Options: &tasks.LinearOptions{ConnectionId: 1, TeamId: "team-1"},
	}

	dataflowTester.FlushTabler(&models.LinearIssue{})
	dataflowTester.FlushTabler(&models.LinearIssueLabel{})
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_linear_issues_long_title.csv", "_raw_linear_issues")
	// must not error with "Data too long for column 'title'"
	dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)

	var issue models.LinearIssue
	err := dataflowTester.Dal.First(&issue, dal.Where("connection_id = ? AND id = ?", 1, "issue-longtitle"))
	assert.NoError(t, err)
	assert.Len(t, issue.Title, 300, "full 300-char title must be stored untruncated")
}
