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

package tasks

import (
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// cursorExtractorVersion bumps when extract logic changes in a way that requires
// a one-time full re-extract (SubtaskStateManager compares SubtaskConfig).
const cursorExtractorVersion = 4

func cursorSubtaskCommonArgs(taskCtx plugin.SubTaskContext, data *CursorTaskData, rawTable string) *helper.SubtaskCommonArgs {
	return &helper.SubtaskCommonArgs{
		SubTaskContext: taskCtx,
		Table:          rawTable,
		Params:         rawParamsFromTaskData(data),
		SubtaskConfig: map[string]any{
			"extractorVersion": cursorExtractorVersion,
		},
	}
}
