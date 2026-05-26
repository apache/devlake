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

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/asana/impl"
	"github.com/apache/incubator-devlake/plugins/asana/models"
	"github.com/apache/incubator-devlake/plugins/asana/tasks"
)

func TestAsanaTaskDataFlow(t *testing.T) {
	var asana impl.Asana
	dataflowTester := e2ehelper.NewDataFlowTester(t, "asana", asana)

	taskData := &tasks.AsanaTaskData{
		Options: &tasks.AsanaOptions{
			ConnectionId: 1,
			ProjectId:    "123456789",
		},
	}

	dataflowTester.ImportCsvIntoRawTable(rawTablePath("_raw_asana_tasks.csv"), "_raw_asana_tasks")

	dataflowTester.FlushTabler(&models.AsanaTask{})
	dataflowTester.Subtask(tasks.ExtractTaskMeta, taskData)
	dataflowTester.VerifyTableWithOptions(models.AsanaTask{}, e2ehelper.TableOptions{
		CSVRelPath:  snapshotPath("_tool_asana_tasks.csv"),
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})
}
