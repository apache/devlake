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
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/jira/impl"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks"

	"github.com/stretchr/testify/require"
)

func TestEpicDataflow(t *testing.T) {
	var plugin impl.Jira
	dataflowTester := e2ehelper.NewDataFlowTester(t, "jira", plugin)
	taskData := &tasks.JiraTaskData{
		Options: &tasks.JiraOptions{
			ConnectionId: 1,
			BoardId:      93,
			ScopeConfig:  &models.JiraScopeConfig{StoryPointField: "customfield_10024"},
		},
	}

	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_jira_api_issue_types.csv", "_raw_jira_api_issue_types")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_jira_api_issues.csv", "_raw_jira_api_issues")
	dataflowTester.ImportCsvIntoRawTable("./raw_tables/_raw_jira_external_epics.csv", "_raw_jira_api_epics")

	dataflowTester.FlushTabler(&models.JiraIssue{})
	dataflowTester.FlushTabler(&models.JiraBoardIssue{})
	dataflowTester.FlushTabler(&models.JiraSprintIssue{})
	dataflowTester.FlushTabler(&models.JiraIssueChangelogs{})
	dataflowTester.FlushTabler(&models.JiraIssueChangelogItems{})
	dataflowTester.FlushTabler(&models.JiraWorklog{})
	dataflowTester.FlushTabler(&models.JiraAccount{})
	dataflowTester.FlushTabler(&models.JiraIssueType{})
	dataflowTester.FlushTabler(&models.JiraIssueComment{})
	dataflowTester.FlushTabler(&models.JiraIssueLabel{})
	dataflowTester.FlushTabler(&models.JiraIssueField{})

	ctx := dataflowTester.SubtaskContext(taskData)

	// run pre-req subtasks
	require.NoError(t, tasks.ExtractIssueTypesMeta.EntryPoint(ctx))
	require.NoError(t, tasks.ExtractIssuesMeta.EntryPoint(ctx))
	dataflowTester.VerifyTableWithOptions(
		models.JiraIssue{}, e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/_tool_jira_issues_for_external_epics.csv",
			TargetFields: nil,
			IgnoreFields: nil,
			IgnoreTypes:  []interface{}{common.NoPKModel{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.JiraBoardIssue{}, e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/_tool_jira_board_issues_for_external_epics.csv",
			TargetFields: []string{"connection_id", "board_id", "issue_id"},
			IgnoreFields: nil,
			IgnoreTypes:  []interface{}{common.NoPKModel{}},
		},
	)
	t.Run("batch_single", func(t *testing.T) {
		// run the part of the collector that queries tools data
		iter, err := tasks.GetEpicKeysIterator(ctx.GetDal(), taskData, 1)
		require.NoError(t, err)
		require.True(t, iter.HasNext())
		e1, err := iter.Fetch()
		require.NoError(t, err)
		require.True(t, iter.HasNext())
		e2, err := iter.Fetch()
		require.NoError(t, err)
		require.False(t, iter.HasNext())
		require.Equal(t, 1, len(e1.([]interface{})))
		require.Equal(t, 1, len(e2.([]interface{})))
		epicKeys := []string{
			*(e1.([]interface{})[0].(*string)),
			*(e2.([]interface{})[0].(*string)),
		}
		require.Contains(t, epicKeys, "K5-1")
		require.Contains(t, epicKeys, "K5-4")
	})
	t.Run("batch_multiple", func(t *testing.T) {
		// run the part of the collector that queries tools data
		iter, err := tasks.GetEpicKeysIterator(ctx.GetDal(), taskData, 2)
		require.NoError(t, err)
		require.True(t, iter.HasNext())
		e, err := iter.Fetch()
		require.NoError(t, err)
		require.False(t, iter.HasNext())
		require.Equal(t, 2, len(e.([]interface{})))
		epicKeys := []string{
			*(e.([]interface{})[0].(*string)),
			*(e.([]interface{})[1].(*string)),
		}
		require.Contains(t, epicKeys, "K5-1")
		require.Contains(t, epicKeys, "K5-4")
	})

	require.NoError(t, tasks.ExtractEpicsMeta.EntryPoint(ctx))

	dataflowTester.VerifyTableWithOptions(
		models.JiraBoardIssue{}, e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/_tool_jira_board_issues_for_external_epics.csv",
			TargetFields: nil,
			IgnoreFields: nil,
			IgnoreTypes:  []interface{}{common.NoPKModel{}},
		},
	)
	dataflowTester.VerifyTableWithOptions(
		models.JiraIssue{}, e2ehelper.TableOptions{
			CSVRelPath:   "./snapshot_tables/_tool_jira_issues_for_external_epics.csv",
			TargetFields: nil,
			IgnoreFields: nil,
			IgnoreTypes:  []interface{}{common.NoPKModel{}},
		},
	)
}
