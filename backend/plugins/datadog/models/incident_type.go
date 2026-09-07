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
	"github.com/apache/devlake/core/plugin"
)

type DatadogParams struct {
	ConnectionId uint64
	ScopeId      string
}

// IncidentType is the collection scope. Datadog names incidents
// `<Prefix>-<public id>` per type, so Prefix is what reconstructs the
// human-facing key the UI shows.
type IncidentType struct {
	common.Scope `mapstructure:",squash"`
	Id           string `json:"id" mapstructure:"id" gorm:"primaryKey;autoIncrement:false"`
	Name         string `json:"name" mapstructure:"name"`
	Prefix       string `json:"prefix" mapstructure:"prefix"`
	IsDefault    bool   `json:"isDefault" mapstructure:"isDefault"`
}

func (t IncidentType) ScopeId() string {
	return t.Id
}

func (t IncidentType) ScopeName() string {
	return t.Name
}

func (t IncidentType) ScopeFullName() string {
	return t.Name
}

func (t IncidentType) ScopeParams() interface{} {
	return &DatadogParams{
		ConnectionId: t.ConnectionId,
		ScopeId:      t.Id,
	}
}

func (t IncidentType) TableName() string {
	return "_tool_datadog_incident_types"
}

var _ plugin.ToolLayerScope = (*IncidentType)(nil)
