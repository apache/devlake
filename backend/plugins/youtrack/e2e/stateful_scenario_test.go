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
	"context"
	"testing"

	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/runner"
	"github.com/apache/devlake/helpers/e2ehelper"
	contextimpl "github.com/apache/devlake/impls/context"
	"github.com/stretchr/testify/require"
)

// beginStatefulScenario opens a test that exercises the framework's
// incremental/full-sync TRANSITIONS — the thing DataFlowTester.Subtask
// cannot cover, because it deletes _devlake_subtask_state and forces
// FullSync: true on every call (helpers/e2ehelper/data_flow_tester.go).
//
// Within the scenario, subtasks run through runSubtaskPreservingState: the
// subtask state survives between calls and the sync policy permits
// incremental mode, so a stateful extractor's second run goes incremental
// on an unchanged config and full-syncs on a config change — exactly the
// production semantics. The first run still full-syncs (no prior state).
//
// The scenario flushes _devlake_subtask_state at entry because the e2e
// database is shared across tests and state keys (plugin, subtask, params)
// do not include the test name.
func beginStatefulScenario(t *testing.T, dataflowTester *e2ehelper.DataFlowTester) {
	t.Helper()
	dataflowTester.FlushTabler(&models.SubtaskState{})
}

// runSubtaskPreservingState executes one subtask inside a stateful scenario
// (see beginStatefulScenario): state is neither flushed nor forced into
// full sync. Panics on error, like DataFlowTester.Subtask.
func runSubtaskPreservingState(t *testing.T, dataflowTester *e2ehelper.DataFlowTester, subtaskMeta plugin.SubTaskMeta, taskData interface{}) {
	t.Helper()
	subtaskCtx := contextimpl.NewStandaloneSubTaskContext(
		context.Background(),
		runner.CreateBasicRes(dataflowTester.Cfg, dataflowTester.Log, dataflowTester.Db),
		dataflowTester.Name,
		taskData,
		subtaskMeta.Name,
		&models.SyncPolicy{},
	)
	require.NoError(t, subtaskMeta.EntryPoint(subtaskCtx))
}
