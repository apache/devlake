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
	"time"

	"github.com/apache/incubator-devlake/core/models/common"
)

// GhCopilotUserDailyMetrics captures per-user daily Copilot usage metrics from enterprise reports.
type GhCopilotUserDailyMetrics struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	UserId       int64     `gorm:"primaryKey" json:"userId"`

	OrganizationId string `json:"organizationId" gorm:"type:varchar(100)"`
	EnterpriseId   string `json:"enterpriseId" gorm:"type:varchar(100)"`
	UserLogin      string `json:"userLogin" gorm:"type:varchar(255);index"`
	UsedAgent      bool   `json:"usedAgent"`
	UsedChat       bool   `json:"usedChat"`

	CopilotActivityMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotUserDailyMetrics) TableName() string {
	return "_tool_copilot_user_daily_metrics"
}

// GhCopilotUserMetricsByIde stores per-user metrics broken down by IDE.
type GhCopilotUserMetricsByIde struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	UserId       int64     `gorm:"primaryKey" json:"userId"`
	Ide          string    `gorm:"primaryKey;type:varchar(50)" json:"ide"`

	LastKnownPluginName    string `json:"lastKnownPluginName" gorm:"type:varchar(100)"`
	LastKnownPluginVersion string `json:"lastKnownPluginVersion" gorm:"type:varchar(50)"`
	LastKnownIdeVersion    string `json:"lastKnownIdeVersion" gorm:"type:varchar(50)"`

	CopilotActivityMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotUserMetricsByIde) TableName() string {
	return "_tool_copilot_user_metrics_by_ide"
}

// GhCopilotUserMetricsByFeature stores per-user metrics broken down by feature.
type GhCopilotUserMetricsByFeature struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	UserId       int64     `gorm:"primaryKey" json:"userId"`
	Feature      string    `gorm:"primaryKey;type:varchar(100)" json:"feature"`

	CopilotActivityMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotUserMetricsByFeature) TableName() string {
	return "_tool_copilot_user_metrics_by_feature"
}

// GhCopilotUserMetricsByLanguageFeature stores per-user metrics by language and feature.
type GhCopilotUserMetricsByLanguageFeature struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	UserId       int64     `gorm:"primaryKey" json:"userId"`
	Language     string    `gorm:"primaryKey;type:varchar(50)" json:"language"`
	Feature      string    `gorm:"primaryKey;type:varchar(100)" json:"feature"`

	CopilotCodeMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotUserMetricsByLanguageFeature) TableName() string {
	return "_tool_copilot_user_metrics_by_language_feature"
}

// GhCopilotUserMetricsByLanguageModel stores per-user metrics by language and model.
type GhCopilotUserMetricsByLanguageModel struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	UserId       int64     `gorm:"primaryKey" json:"userId"`
	Language     string    `gorm:"primaryKey;type:varchar(50)" json:"language"`
	Model        string    `gorm:"primaryKey;type:varchar(100)" json:"model"`

	CopilotCodeMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotUserMetricsByLanguageModel) TableName() string {
	return "_tool_copilot_user_metrics_by_language_model"
}

// GhCopilotUserMetricsByModelFeature stores per-user metrics by model and feature.
type GhCopilotUserMetricsByModelFeature struct {
	ConnectionId uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId      string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	Day          time.Time `gorm:"primaryKey;type:date" json:"day"`
	UserId       int64     `gorm:"primaryKey" json:"userId"`
	Model        string    `gorm:"primaryKey;type:varchar(100)" json:"model"`
	Feature      string    `gorm:"primaryKey;type:varchar(100)" json:"feature"`

	CopilotActivityMetrics `mapstructure:",squash"`
	common.NoPKModel
}

func (GhCopilotUserMetricsByModelFeature) TableName() string {
	return "_tool_copilot_user_metrics_by_model_feature"
}
