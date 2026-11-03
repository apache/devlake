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
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/circleci/impl"
	"github.com/apache/devlake/plugins/circleci/models"
	"github.com/apache/devlake/plugins/circleci/tasks"
	"github.com/stretchr/testify/assert"
)

const unfinishedTestProjectSlug = "github/test/repo"

var unfinishedTestNow = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)

func daysAgo(days int) *common.Iso8601Time {
	return &common.Iso8601Time{Time: unfinishedTestNow.AddDate(0, 0, -days)}
}

func seedUnfinishedWorkflows(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) {
	dataflowTester.FlushTabler(&models.CircleciWorkflow{})
	seed := []models.CircleciWorkflow{
		{ConnectionId: 1, Id: "wf-onhold", ProjectSlug: unfinishedTestProjectSlug, Status: "on_hold", CreatedDate: daysAgo(1)},
		{ConnectionId: 1, Id: "wf-queued", ProjectSlug: unfinishedTestProjectSlug, Status: "running", CreatedDate: daysAgo(29)},
		{ConnectionId: 1, Id: "wf-failing", ProjectSlug: unfinishedTestProjectSlug, Status: "failing", CreatedDate: daysAgo(2)},
		{ConnectionId: 1, Id: "wf-success", ProjectSlug: unfinishedTestProjectSlug, Status: "success", CreatedDate: daysAgo(1)},
		{ConnectionId: 1, Id: "wf-old-onhold", ProjectSlug: unfinishedTestProjectSlug, Status: "on_hold", CreatedDate: daysAgo(31)},
		{ConnectionId: 2, Id: "wf-otherconn", ProjectSlug: unfinishedTestProjectSlug, Status: "on_hold", CreatedDate: daysAgo(1)},
	}
	for i := range seed {
		assert.Nil(t, dataflowTester.Dal.Create(&seed[i]))
	}
}

func collectInputIds(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, clauses []dal.Clause, rowType reflect.Type) []string {
	cursor, err := dataflowTester.Dal.Cursor(clauses...)
	assert.Nil(t, err)
	iter, err := api.NewDalCursorIterator(dataflowTester.Dal, cursor, rowType)
	assert.Nil(t, err)
	defer iter.Close()

	var ids []string
	for iter.HasNext() {
		item, err := iter.Fetch()
		assert.Nil(t, err)
		ids = append(ids, reflect.ValueOf(item).Elem().FieldByName("Id").String())
	}
	sort.Strings(ids)
	return ids
}

// TestCircleciUnfinishedJobsInputIterator is a regression test for
// https://github.com/apache/devlake/issues/8907 and
// https://github.com/apache/devlake/issues/9304. The "collect unfinished job
// details" collector builds its URL from "/v2/workflow/{{ .Input.Id }}/job" while
// scanning rows into a models.CircleciJob. Its input query must therefore expose the
// workflow id in the row's Id field; a bare "DISTINCT workflow_id" left Id empty and
// produced "/v2/workflow//job" (HTTP 500). It must also skip workflows older than the
// lookback window: an approval job that is never approved stays on_hold forever, and
// its workflow was collected again on every run (#9304).
func TestCircleciUnfinishedJobsInputIterator(t *testing.T) {
	var circleci impl.Circleci
	dataflowTester := e2ehelper.NewDataFlowTester(t, "circleci", circleci)

	seedUnfinishedWorkflows(t, dataflowTester)
	dataflowTester.FlushTabler(&models.CircleciJob{})
	seed := []models.CircleciJob{
		{ConnectionId: 1, WorkflowId: "wf-onhold", Id: "job-1", ProjectSlug: unfinishedTestProjectSlug, Status: "on_hold"},
		{ConnectionId: 1, WorkflowId: "wf-onhold", Id: "job-2", ProjectSlug: unfinishedTestProjectSlug, Status: "running"}, // same workflow -> DISTINCT
		{ConnectionId: 1, WorkflowId: "wf-queued", Id: "job-3", ProjectSlug: unfinishedTestProjectSlug, Status: "queued"},
		{ConnectionId: 1, WorkflowId: "wf-success", Id: "job-4", ProjectSlug: unfinishedTestProjectSlug, Status: "success"},    // terminal -> excluded
		{ConnectionId: 1, WorkflowId: "wf-old-onhold", Id: "job-5", ProjectSlug: unfinishedTestProjectSlug, Status: "on_hold"}, // older than lookback -> excluded
		{ConnectionId: 2, WorkflowId: "wf-otherconn", Id: "job-6", ProjectSlug: unfinishedTestProjectSlug, Status: "on_hold"},  // other connection -> excluded
	}
	for i := range seed {
		assert.Nil(t, dataflowTester.Dal.Create(&seed[i]))
	}

	clauses := tasks.UnfinishedJobsInputClauses(1, unfinishedTestProjectSlug, tasks.UnfinishedDetailsCreatedAfter(unfinishedTestNow))
	ids := collectInputIds(t, dataflowTester, clauses, reflect.TypeOf(models.CircleciJob{}))

	// Distinct workflow ids for connection 1's non-terminal jobs, with Id populated
	// (the URL template reads .Input.Id).
	assert.Equal(t, []string{"wf-onhold", "wf-queued"}, ids)
	for _, id := range ids {
		assert.NotEmpty(t, id, "Input.Id must be the workflow id, not empty (#8907)")
	}
}

// TestCircleciUnfinishedWorkflowsInputIterator is a regression test for
// https://github.com/apache/devlake/issues/9304: the "collect unfinished workflow
// details" collector must skip workflows older than the lookback window.
func TestCircleciUnfinishedWorkflowsInputIterator(t *testing.T) {
	var circleci impl.Circleci
	dataflowTester := e2ehelper.NewDataFlowTester(t, "circleci", circleci)

	seedUnfinishedWorkflows(t, dataflowTester)

	clauses := tasks.UnfinishedWorkflowsInputClauses(1, unfinishedTestProjectSlug, tasks.UnfinishedDetailsCreatedAfter(unfinishedTestNow))
	ids := collectInputIds(t, dataflowTester, clauses, reflect.TypeOf(models.CircleciWorkflow{}))

	assert.Equal(t, []string{"wf-failing", "wf-onhold", "wf-queued"}, ids)
}
