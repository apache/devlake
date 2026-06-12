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

// @Summary blueprints setting for icla
// @Description blueprint setting for icla
// @Tags plugins/icla
// @Accept application/json
// @Param blueprint body iclaBlueprintSetting true "json"
// @Router /blueprints/icla/blueprint-setting [post]
func _() {}

//nolint:unused
type iclaBlueprintSetting []struct {
	Version     string `json:"version" example:"1.0.0"`
	Connections []struct {
		Plugin string `json:"plugin" example:"icla"`
	} `json:"connections"`
}

// @Summary pipelines plan for icla
// @Description pipelines plan for icla
// @Tags plugins/icla
// @Accept application/json
// @Param pipeline body iclaPipelinePlan true "json"
// @Router /pipelines/icla/pipeline-plan [post]
func _() {}

//nolint:unused
type iclaPipelinePlan [][]struct {
	Plugin string `json:"plugin" example:"icla"`
}
