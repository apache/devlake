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

	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/devops"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/refdiff/impl"
	"github.com/apache/devlake/plugins/refdiff/models"
	"github.com/apache/devlake/plugins/refdiff/tasks"
)

func TestDeploymentCommitDiffDataFlow(t *testing.T) {

	var plugin impl.RefDiff
	dataflowTester := e2ehelper.NewDataFlowTester(t, "refdiff", plugin)

	taskData := &tasks.RefdiffTaskData{
		Options: &models.RefdiffOptions{
			ProjectName: "project1",
		},
	}

	// import raw data table
	dataflowTester.ImportCsvIntoTabler("./deployment_commit_diff/project_mapping.csv", &crossdomain.ProjectMapping{})
	dataflowTester.ImportCsvIntoTabler("./deployment_commit_diff/repo_commits.csv", &code.RepoCommit{})
	dataflowTester.ImportCsvIntoTabler("./deployment_commit_diff/commit_parents.csv", &code.CommitParent{})
	dataflowTester.ImportCsvIntoTabler("./deployment_commit_diff/cicd_deployment_commits.csv", &devops.CicdDeploymentCommit{})

	// verify extraction
	dataflowTester.FlushTabler(&code.CommitsDiff{})
	dataflowTester.FlushTabler(&models.FinishedCommitsDiff{})

	dataflowTester.Subtask(tasks.CalculateDeploymentCommitsDiffMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&code.CommitsDiff{}, e2ehelper.TableOptions{
		CSVRelPath: "./deployment_commit_diff/commits_diffs.csv",
	})

	dataflowTester.VerifyTableWithOptions(&models.FinishedCommitsDiff{}, e2ehelper.TableOptions{
		CSVRelPath: "./deployment_commit_diff/_tool_refdiff_finished_commits_diffs.csv",
	})

	// a second run must not recalculate any finished pair, including deployments
	// without a previous successful deployment (finished with an empty old sha)
	dataflowTester.FlushTabler(&code.CommitsDiff{})
	dataflowTester.Subtask(tasks.CalculateDeploymentCommitsDiffMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&code.CommitsDiff{}, e2ehelper.TableOptions{
		CSVRelPath: "./deployment_commit_diff/commits_diffs_rerun.csv",
	})
}
