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

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/linear/impl"
	"github.com/apache/incubator-devlake/plugins/linear/models"
	"github.com/apache/incubator-devlake/plugins/linear/tasks"
)

func TestLinearCommentDataFlow(t *testing.T) {
	var linear impl.Linear
	dataflowTester := e2ehelper.NewDataFlowTester(t, "linear", linear)

	taskData := &tasks.LinearTaskData{
		Options: &tasks.LinearOptions{
			ConnectionId: 1,
			TeamId:       "team-1",
		},
	}

	// the comment convertor joins to issues, so populate _tool_linear_issues first
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_linear_issues.csv", "_raw_linear_issues")
	dataflowTester.FlushTabler(&models.LinearIssue{})
	dataflowTester.FlushTabler(&models.LinearIssueLabel{})
	dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)

	// verify extraction: raw -> tool layer
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_linear_comments.csv", "_raw_linear_comments")
	dataflowTester.FlushTabler(&models.LinearComment{})
	dataflowTester.Subtask(tasks.ExtractCommentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(models.LinearComment{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_linear_comments.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// verify conversion: tool layer -> domain layer
	dataflowTester.FlushTabler(&ticket.IssueComment{})
	dataflowTester.Subtask(tasks.ConvertCommentsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(ticket.IssueComment{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_comments.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
