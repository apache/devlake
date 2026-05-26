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

package api

import (
	"github.com/apache/incubator-devlake/plugins/asana/tasks"
)

type AsanaTaskOptions tasks.AsanaOptions

// @Summary asana task options for pipelines
// @Description Task options for asana pipelines
// @Tags plugins/asana
// @Accept application/json
// @Param pipeline body AsanaTaskOptions true "json"
// @Router /pipelines/asana/pipeline-task [post]
func _() {}
