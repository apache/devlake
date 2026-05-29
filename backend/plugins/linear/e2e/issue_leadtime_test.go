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

// TestLinearIssueNegativeLeadTime guards the lead-time fallback against
// resolution timestamps that precede the creation timestamp (clock skew or
// migrated/imported issues). A negative duration must NOT be cast to uint --
// doing so wraps to a huge bogus value. The expected behaviour is that no lead
// time is derived (lead_time_minutes stays empty).
func TestLinearIssueNegativeLeadTime(t *testing.T) {
	var linear impl.Linear
	dataflowTester := e2ehelper.NewDataFlowTester(t, "linear", linear)

	taskData := &tasks.LinearTaskData{
		Options: &tasks.LinearOptions{
			ConnectionId: 1,
			TeamId:       "team-1",
		},
	}

	// extraction: raw -> tool layer
	dataflowTester.FlushTabler(&models.LinearIssue{})
	dataflowTester.FlushTabler(&models.LinearIssueLabel{})
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_linear_issues_negative_leadtime.csv", "_raw_linear_issues")
	dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)

	// conversion: tool layer -> domain layer
	dataflowTester.FlushTabler(&ticket.Issue{})
	dataflowTester.FlushTabler(&ticket.BoardIssue{})
	dataflowTester.Subtask(tasks.ConvertIssuesMeta, taskData)
	dataflowTester.VerifyTableWithOptions(ticket.Issue{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issues_negative_leadtime.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
