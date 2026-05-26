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
	"regexp"
	"testing"

	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/linker/impl"
	"github.com/apache/incubator-devlake/plugins/linker/tasks"
)

func TestLinkPrToIssueWithSharedRepo(t *testing.T) {
	var plugin impl.Linker
	dataflowTester := e2ehelper.NewDataFlowTester(t, "linker", plugin)

	regexpStr := "#(\\d+)"
	re, err := regexp.Compile(regexpStr)
	if err != nil {
		panic(err)
	}
	taskData := &tasks.LinkerTaskData{
		Options: &tasks.LinkerOptions{
			PrToIssueRegexp: regexpStr,
			ProjectName:     "GitHub1",
		},
		PrToIssueRegexp: re,
	}

	// Import input tables for two projects sharing repo R1
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/cross_project_issues.csv", &ticket.Issue{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/cross_project_pull_requests.csv", &code.PullRequest{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/cross_project_project_mapping.csv", &crossdomain.ProjectMapping{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/cross_project_board_issues.csv", &ticket.BoardIssue{})

	// Pre-populate pull_request_issues with data from GitHub2's linker and from GitHub converter.
	// These rows must survive when we run the linker for GitHub1.
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/cross_project_pull_request_issues_before.csv", &crossdomain.PullRequestIssue{})

	dataflowTester.Subtask(tasks.LinkPrToIssueMeta, taskData)

	// Verify: GitHub1's links are created, GitHub2's linker link and converter link survive
	dataflowTester.VerifyTable(
		crossdomain.PullRequestIssue{},
		"./snapshot_tables/cross_project_pull_request_issues_after.csv",
		e2ehelper.ColumnWithRawData(
			"pull_request_id",
			"pull_request_key",
			"issue_id",
			"issue_key",
		),
	)
}

func TestLinkPrToIssue(t *testing.T) {
	var plugin impl.Linker
	dataflowTester := e2ehelper.NewDataFlowTester(t, "linker", plugin)

	regexpStr := "#(\\d+)"
	re, err := regexp.Compile(regexpStr)
	if err != nil {
		panic(err)
	}
	taskData := &tasks.LinkerTaskData{
		Options: &tasks.LinkerOptions{
			PrToIssueRegexp: regexpStr,
			ProjectName:     "GitHub1",
		},
		PrToIssueRegexp: re,
	}

	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/issues.csv", &ticket.Issue{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/pull_requests.csv", &code.PullRequest{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/project_mapping.csv", &crossdomain.ProjectMapping{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/board_issues.csv", &ticket.BoardIssue{})

	dataflowTester.FlushTabler(&crossdomain.PullRequestIssue{})
	dataflowTester.Subtask(tasks.LinkPrToIssueMeta, taskData)
	dataflowTester.VerifyTable(
		crossdomain.PullRequestIssue{},
		"./snapshot_tables/pull_request_issues.csv",
		[]string{
			"pull_request_id",
			"pull_request_key",
			"issue_id",
			"issue_key",
			"_raw_data_params",
			"_raw_data_table",
			"_raw_data_id",
			"_raw_data_remark",
		},
	)

}
