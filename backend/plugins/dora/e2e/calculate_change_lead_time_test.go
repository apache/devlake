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

	// verify converter
	dataflowTester.FlushTabler(&crossdomain.ProjectPrMetric{})
	dataflowTester.Subtask(tasks.CalculateChangeLeadTimeMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&crossdomain.ProjectPrMetric{}, e2ehelper.TableOptions{
		CSVRelPath:  "./change_lead_time/project_pr_metrics.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}

func TestCalculateCLTimeWithPrHeadRefPatternDataFlow(t *testing.T) {
	var plugin impl.Dora
	dataflowTester := e2ehelper.NewDataFlowTester(t, "dora", plugin)

	taskData := &tasks.DoraTaskData{
		Options: &tasks.DoraOptions{
			ProjectName:      "project1",
			PrHeadRefPattern: "^(feat|fix)/",
		},
		PrHeadRefRegexp: regexp.MustCompile("^(feat|fix)/"),
	}

	dataflowTester.FlushTabler(&code.PullRequest{})

	dataflowTester.ImportCsvIntoTabler("./change_lead_time/project_mapping.csv", &crossdomain.ProjectMapping{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/repos.csv", &code.Repo{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/cicd_scopes.csv", &devops.CicdScope{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/pull_requests.csv", &code.PullRequest{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/cicd_deployment_commits.csv", &devops.CicdDeploymentCommit{})
	dataflowTester.ImportNullableCsvIntoTabler("./change_lead_time/commits_diffs.csv", &code.CommitsDiff{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/pull_request_comments.csv", &code.PullRequestComment{})
	dataflowTester.ImportCsvIntoTabler("./change_lead_time/pull_request_commits.csv", &code.PullRequestCommit{})

	// only pr0 (feat/a), pr1 (fix/b) and pr4 (refs/heads/feat/d) match the pattern
	dataflowTester.FlushTabler(&crossdomain.ProjectPrMetric{})
	dataflowTester.Subtask(tasks.CalculateChangeLeadTimeMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&crossdomain.ProjectPrMetric{}, e2ehelper.TableOptions{
		CSVRelPath:  "./change_lead_time/project_pr_metrics_pr_head_ref_pattern.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
