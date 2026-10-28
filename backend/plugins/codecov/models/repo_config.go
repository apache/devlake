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

package models

import (
	"github.com/apache/devlake/core/models/common"
)

type CodecovRepoConfig struct {
	common.NoPKModel
	ConnectionId         uint64   `gorm:"primaryKey;type:bigint" json:"connectionId"`
	RepoId               string   `gorm:"primaryKey;type:varchar(200)" json:"repoId"`
	ConfigSource         string   `gorm:"type:varchar(50)" json:"configSource"`
	RawYaml              string   `gorm:"type:text" json:"rawYaml"`
	ProjectTarget        *float64 `json:"projectTarget"`
	ProjectTargetAuto    bool     `json:"projectTargetAuto"`
	ProjectThreshold     *float64 `json:"projectThreshold"`
	ProjectInformational bool     `json:"projectInformational"`
	PatchTarget          *float64 `json:"patchTarget"`
	PatchTargetAuto      bool     `json:"patchTargetAuto"`
	PatchThreshold       *float64 `json:"patchThreshold"`
	PatchInformational   bool     `json:"patchInformational"`
}

func (CodecovRepoConfig) TableName() string {
	return "_tool_codecov_repo_configs"
}
