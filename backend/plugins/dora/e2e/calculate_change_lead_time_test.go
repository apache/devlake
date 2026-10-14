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

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/dora/impl"
	"github.com/apache/devlake/plugins/dora/tasks"
)

func TestCalculateCLTimeDataFlow(t *testing.T) {
	var plugin impl.Dora
	dataflowTester := e2ehelper.NewDataFlowTester(t, "dora", plugin)

	taskData := &tasks.DoraTaskData{
		Options: &tasks.DoraOptions{
			ProjectName: "project1",
		},
	}

	dataflowTester.FlushTabler(&code.PullRequest{})

	// import raw data table
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/project_mapping.csv", &crossdomain.ProjectMapping{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/repos.csv", &code.Repo{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/cicd_scopes.csv", &devops.CicdScope{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/pull_requests.csv", &code.PullRequest{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/cicd_deployment_commits.csv", &devops.CicdDeploymentCommit{})
	dataflowTester.ImportNullableCsvIntoTabler("./change_lead_time/commits_diffs.csv", &code.CommitsDiff{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/pull_request_comments.csv", &code.PullRequestComment{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/pull_request_commits.csv", &code.PullRequestCommit{})
	// bot1 comments on pr1 before the first human review; it must not be counted as the first review
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/accounts.csv", &crossdomain.Account{})

	// verify converter
	dataflowTester.FlushTabler(&crossdomain.ProjectPrMetric{})
	dataflowTester.Subtask(tasks.CalculateChangeLeadTimeMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&crossdomain.ProjectPrMetric{}, e2ehelper.TableOptions{
		CSVRelPath:  "./change_lead_time/project_pr_metrics.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
