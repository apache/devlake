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

type DatadogScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`
	// ComponentField names the Datadog incident custom field whose value
	// becomes the domain issue's Component, e.g. "services" or a
	// team-defined "area" field. Empty leaves Component unset.
	ComponentField string `mapstructure:"componentField" json:"componentField" gorm:"type:varchar(255)"`
	// SeverityField overrides Datadog's native SEV severity with a custom
	// field, for organizations that record judged impact separately from
	// the severity a paging monitor assigned.
	SeverityField string `mapstructure:"severityField" json:"severityField" gorm:"type:varchar(255)"`
}

func (DatadogScopeConfig) TableName() string {
	return "_tool_datadog_scope_configs"
}
