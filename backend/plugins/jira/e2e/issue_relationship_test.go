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
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/jira/impl"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks"
)

func TestIssueRelationshipDataFlow(t *testing.T) {
	var plugin impl.Jira
	dataflowTester := e2ehelper.NewDataFlowTester(t, "jira", plugin)

	taskData := &tasks.JiraTaskData{
		Options: &tasks.JiraOptions{
			ConnectionId: 2,
			BoardId:      8,
			ScopeConfig:  &models.JiraScopeConfig{},
		},
	}

	// import raw data table
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_jira_api_issue_relationships.csv", "_raw_jira_api_issues")
	// verify issue extraction
	dataflowTester.FlushTabler(&models.JiraIssueRelationship{})
	dataflowTester.FlushTabler(&models.JiraIssue{})
	dataflowTester.FlushTabler(&models.JiraBoardIssue{})
	dataflowTester.FlushTabler(&models.JiraSprintIssue{})
	dataflowTester.FlushTabler(&models.JiraIssueComment{})
	dataflowTester.FlushTabler(&models.JiraIssueChangelogs{})
	dataflowTester.FlushTabler(&models.JiraIssueChangelogItems{})
	dataflowTester.FlushTabler(&models.JiraWorklog{})
	dataflowTester.FlushTabler(&models.JiraAccount{})
	dataflowTester.FlushTabler(&models.JiraIssueType{})
	dataflowTester.FlushTabler(&models.JiraIssueLabel{})
	dataflowTester.FlushTabler(&models.JiraIssueField{})
	dataflowTester.Subtask(tasks.ExtractIssuesMeta, taskData)

	dataflowTester.VerifyTableWithOptions(&models.JiraIssueRelationship{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/_tool_jira_issue_relationships.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	// verify issue conversion
	dataflowTester.FlushTabler(&models.JiraBoardIssue{})
	dataflowTester.FlushTabler(&ticket.IssueRelationship{})
	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_jira_board_issues_relations.csv", &models.JiraBoardIssue{})

	dataflowTester.Subtask(tasks.ConvertIssueRelationshipsMeta, taskData)
	dataflowTester.VerifyTableWithOptions(&ticket.IssueRelationship{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/issue_relationships.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
