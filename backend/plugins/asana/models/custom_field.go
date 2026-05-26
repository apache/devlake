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
)

// AsanaCustomField represents a custom field definition
type AsanaCustomField struct {
	ConnectionId            uint64 `gorm:"primaryKey"`
	Gid                     string `gorm:"primaryKey;type:varchar(255)"`
	Name                    string `gorm:"type:varchar(255)"`
	ResourceType            string `gorm:"type:varchar(32)"`
	ResourceSubtype         string `gorm:"type:varchar(32)"`
	Type                    string `gorm:"type:varchar(32)"`
	Description             string `gorm:"type:text"`
	Precision               int    `json:"precision"`
	IsGlobalToWorkspace     bool   `json:"isGlobalToWorkspace"`
	HasNotificationsEnabled bool   `json:"hasNotificationsEnabled"`
	common.NoPKModel
}

func (AsanaCustomField) TableName() string {
	return "_tool_asana_custom_fields"
}

// AsanaTaskCustomFieldValue represents a custom field value on a task
type AsanaTaskCustomFieldValue struct {
	ConnectionId    uint64   `gorm:"primaryKey"`
	TaskGid         string   `gorm:"primaryKey;type:varchar(255)"`
	CustomFieldGid  string   `gorm:"primaryKey;type:varchar(255)"`
	CustomFieldName string   `gorm:"type:varchar(255)"`
	DisplayValue    string   `gorm:"type:text"`
	TextValue       string   `gorm:"type:text"`
	NumberValue     *float64 `json:"numberValue"`
	EnumValueGid    string   `gorm:"type:varchar(255)"`
	EnumValueName   string   `gorm:"type:varchar(255)"`
	common.NoPKModel
}

func (AsanaTaskCustomFieldValue) TableName() string {
	return "_tool_asana_task_custom_field_values"
}
