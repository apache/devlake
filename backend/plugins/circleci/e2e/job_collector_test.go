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

	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/circleci/impl"
	"github.com/apache/incubator-devlake/plugins/circleci/models"
	"github.com/apache/incubator-devlake/plugins/circleci/tasks"
	"github.com/stretchr/testify/assert"
)

// TestCircleciUnfinishedJobsInputIterator is a regression test for
// https://github.com/apache/devlake/issues/8907. The "collect unfinished job
// details" collector builds its URL from "/v2/workflow/{{ .Input.Id }}/job" while
// scanning rows into a models.CircleciJob. Its input query must therefore expose the
// workflow id in the row's Id field; a bare "DISTINCT workflow_id" left Id empty and
// produced "/v2/workflow//job" (HTTP 500). This test runs the production query
// (tasks.UnfinishedJobsInputClauses) through the real iterator and asserts each
// yielded row's Id is the workflow id, that results are DISTINCT, and that the
// status/connection filters hold.
func TestCircleciUnfinishedJobsInputIterator(t *testing.T) {
	var circleci impl.Circleci
	dataflowTester := e2ehelper.NewDataFlowTester(t, "circleci", circleci)

	const projectSlug = "github/test/repo"
	dataflowTester.FlushTabler(&models.CircleciJob{})

	seed := []models.CircleciJob{
		{ConnectionId: 1, WorkflowId: "wf-onhold", Id: "job-1", ProjectSlug: projectSlug, Status: "on_hold"},
		{ConnectionId: 1, WorkflowId: "wf-onhold", Id: "job-2", ProjectSlug: projectSlug, Status: "running"}, // same workflow -> DISTINCT
		{ConnectionId: 1, WorkflowId: "wf-queued", Id: "job-3", ProjectSlug: projectSlug, Status: "queued"},
		{ConnectionId: 1, WorkflowId: "wf-success", Id: "job-4", ProjectSlug: projectSlug, Status: "success"},   // terminal -> excluded
		{ConnectionId: 2, WorkflowId: "wf-otherconn", Id: "job-5", ProjectSlug: projectSlug, Status: "on_hold"}, // other connection -> excluded
	}
	for i := range seed {
		assert.Nil(t, dataflowTester.Dal.Create(&seed[i]))
	}

	cursor, err := dataflowTester.Dal.Cursor(tasks.UnfinishedJobsInputClauses(1, projectSlug)...)
	assert.Nil(t, err)
	iter, err := api.NewDalCursorIterator(dataflowTester.Dal, cursor, reflect.TypeOf(models.CircleciJob{}))
	assert.Nil(t, err)
	defer iter.Close()

	var ids []string
	for iter.HasNext() {
		item, err := iter.Fetch()
		assert.Nil(t, err)
		job := item.(*models.CircleciJob)
		ids = append(ids, job.Id)
	}
	sort.Strings(ids)

	// Distinct workflow ids for connection 1's non-terminal jobs, with Id populated
	// (the URL template reads .Input.Id). wf-success (terminal) and wf-otherconn
	// (connection 2) are excluded.
	assert.Equal(t, []string{"wf-onhold", "wf-queued"}, ids)
	for _, id := range ids {
		assert.NotEmpty(t, id, "Input.Id must be the workflow id, not empty (#8907)")
	}
}
