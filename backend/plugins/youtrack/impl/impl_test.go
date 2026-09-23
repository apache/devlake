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

package impl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// SubTaskMetas order is the execution-order contract: the list
// order alone schedules subtasks, and no meta may carry a Dependencies field
// anywhere in the plugin.
func TestSubTaskMetasOrderAndNoDependencies(t *testing.T) {
	metas := Youtrack{}.SubTaskMetas()
	names := make([]string, 0, len(metas))
	for _, meta := range metas {
		names = append(names, meta.Name)
		assert.Nil(t, meta.Dependencies, "%s must not declare Dependencies — list order is the contract", meta.Name)
	}
	assert.Equal(t, []string{
		"Collect Workflow States",
		"Extract Workflow States",
		"Collect Issues",
		"Extract Issues",
		"Convert Projects",
		"Convert Accounts",
		"Convert Issues",
		"Convert Issue Labels",
	}, names, "subtasks in execution order")
}
