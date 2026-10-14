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
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/gitlab/impl"
	"github.com/apache/devlake/plugins/gitlab/models"
	"github.com/apache/devlake/plugins/gitlab/tasks"
)

func TestGitlabMrDetailDataFlow(t *testing.T) {

	var gitlab impl.Gitlab
	dataflowTester := e2ehelper.NewDataFlowTester(t, "gitlab", gitlab)

	taskData := &tasks.GitlabTaskData{
		Options: &tasks.GitlabOptions{
			ConnectionId: 1,
			ProjectId:    12345678,
			ScopeConfig:  new(models.GitlabScopeConfig),
		},
	}
	// import raw data table
	dataflowTester.FlushTabler(&code.PullRequestAssignee{})
	dataflowTester.FlushTabler(&code.PullRequestReviewer{})
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_gitlab_api_merge_requests.csv",
		"_raw_gitlab_api_merge_request_details")

	// verify extraction
	dataflowTester.FlushTabler(&models.GitlabMergeRequest{})
	dataflowTester.FlushTabler(&models.GitlabMrLabel{})
	dataflowTester.FlushTabler(&models.GitlabReviewer{})
	dataflowTester.FlushTabler(&models.GitlabAssignee{})
	dataflowTester.Subtask(tasks.ExtractApiMergeRequestDetailsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&models.GitlabMergeRequest{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_gitlab_merge_request_details.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
	dataflowTester.VerifyTableWithOptions(&models.GitlabMrLabel{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_gitlab_mr_labels.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// verify conversion
	dataflowTester.FlushTabler(&code.PullRequest{})
	dataflowTester.FlushTabler(&code.PullRequestLabel{})
	dataflowTester.Subtask(tasks.ConvertApiMergeRequestsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&code.PullRequest{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/pull_requests.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// verify conversion
	dataflowTester.FlushTabler(&code.PullRequestLabel{})
	dataflowTester.Subtask(tasks.ConvertMrLabelsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&code.PullRequestLabel{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/pull_request_labels.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
