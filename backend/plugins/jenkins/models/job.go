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
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.ToolLayerScope = (*JenkinsJob)(nil)

// JenkinsJob db entity for jenkins job
type JenkinsJob struct {
	common.Scope `mapstructure:",squash"`
	FullName     string `gorm:"primaryKey;type:varchar(255)" mapstructure:"fullName" validate:"required" json:"fullName"` // "path1/path2/job name"
	Name         string `gorm:"index;type:varchar(255)" mapstructure:"name" json:"name"`                                  // scope name now is same to `jobFullName`
	Path         string `gorm:"index;type:varchar(511)" mapstructure:"-,omitempty" json:"-"`                              // "job/path1/job/path2"
	Class        string `gorm:"type:varchar(255)" mapstructure:"class,omitempty" json:"class"`
	Color        string `gorm:"type:varchar(255)" mapstructure:"color,omitempty" json:"color"`
	Base         string `gorm:"type:varchar(255)" mapstructure:"base,omitempty" json:"base"`
	Url          string `mapstructure:"url,omitempty" json:"url"`
	Description  string `mapstructure:"description,omitempty" json:"description"`
	PrimaryView  string `gorm:"type:text" mapstructure:"primaryView,omitempty" json:"primaryView"`
}

func (JenkinsJob) TableName() string {
	return "_tool_jenkins_jobs"
}

func (j JenkinsJob) ScopeId() string {
	return j.FullName
}

func (j JenkinsJob) ScopeName() string {
	return j.Name
}
func (j JenkinsJob) ScopeFullName() string {
	return j.FullName
}

func (j JenkinsJob) ScopeParams() interface{} {
	return &JenkinsApiParams{
		ConnectionId: j.ConnectionId,
		FullName:     j.FullName,
	}
}

type JenkinsApiParams struct {
	ConnectionId uint64
	FullName     string
}
