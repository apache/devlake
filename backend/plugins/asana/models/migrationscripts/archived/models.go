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

package archived

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/migrationscripts/archived"
)

type AsanaConnection struct {
	archived.Model
	Name             string `gorm:"type:varchar(150);uniqueIndex" json:"name" validate:"required"`
	Endpoint         string `mapstructure:"endpoint" json:"endpoint" validate:"required"`
	Proxy            string `mapstructure:"proxy" json:"proxy"`
	RateLimitPerHour int    `comment:"api request rate limit per hour" json:"rateLimitPerHour"`
	Token            string `mapstructure:"token" json:"token" gorm:"serializer:encdec" encrypt:"yes"`
}

func (AsanaConnection) TableName() string {
	return "_tool_asana_connections"
}

type AsanaProject struct {
	ConnectionId  uint64 `gorm:"primaryKey"`
	Gid           string `json:"gid" gorm:"type:varchar(255);primaryKey"`
	Name          string `json:"name" gorm:"type:varchar(255)"`
	ResourceType  string `json:"resourceType" gorm:"type:varchar(32)"`
	Archived      bool   `json:"archived"`
	WorkspaceGid  string `json:"workspaceGid" gorm:"type:varchar(255)"`
	PermalinkUrl  string `json:"permalinkUrl" gorm:"type:varchar(512)"`
	ScopeConfigId uint64 `json:"scopeConfigId,omitempty" mapstructure:"scopeConfigId"`
	archived.NoPKModel
}

func (AsanaProject) TableName() string {
	return "_tool_asana_projects"
}

type AsanaScopeConfig struct {
	archived.ScopeConfig
	IssueTypeRequirement string `mapstructure:"issueTypeRequirement,omitempty" json:"issueTypeRequirement" gorm:"type:varchar(255)"`
	IssueTypeBug         string `mapstructure:"issueTypeBug,omitempty" json:"issueTypeBug" gorm:"type:varchar(255)"`
	IssueTypeIncident    string `mapstructure:"issueTypeIncident,omitempty" json:"issueTypeIncident" gorm:"type:varchar(255)"`
}

func (AsanaScopeConfig) TableName() string {
	return "_tool_asana_scope_configs"
}

type AsanaTask struct {
	ConnectionId    uint64     `gorm:"primaryKey"`
	Gid             string     `gorm:"primaryKey;type:varchar(255)"`
	Name            string     `gorm:"type:varchar(512)"`
	Notes           string     `gorm:"type:text"`
	ResourceType    string     `gorm:"type:varchar(32)"`
	ResourceSubtype string     `gorm:"type:varchar(32)"`
	Completed       bool       `json:"completed"`
	CompletedAt     *time.Time `json:"completedAt"`
	DueOn           *time.Time `gorm:"type:date" json:"dueOn"`
	CreatedAt       time.Time  `json:"createdAt"`
	ModifiedAt      *time.Time `json:"modifiedAt"`
	PermalinkUrl    string     `gorm:"type:varchar(512)"`
	ProjectGid      string     `gorm:"type:varchar(255);index"`
	SectionGid      string     `gorm:"type:varchar(255);index"`
	SectionName     string     `gorm:"type:varchar(255)"`
	AssigneeGid     string     `gorm:"type:varchar(255)"`
	AssigneeName    string     `gorm:"type:varchar(255)"`
	CreatorGid      string     `gorm:"type:varchar(255)"`
	CreatorName     string     `gorm:"type:varchar(255)"`
	ParentGid       string     `gorm:"type:varchar(255);index"`
	NumSubtasks     int        `json:"numSubtasks"`
	StdType         string     `gorm:"type:varchar(255)"`
	StdStatus       string     `gorm:"type:varchar(255)"`
	Priority        string     `gorm:"type:varchar(255)"`
	StoryPoint      *float64   `json:"storyPoint"`
	Severity        string     `gorm:"type:varchar(255)"`
	LeadTimeMinutes *uint      `json:"leadTimeMinutes"`
	archived.NoPKModel
}

func (AsanaTask) TableName() string {
	return "_tool_asana_tasks"
}

type AsanaSection struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Gid          string `gorm:"primaryKey;type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	ResourceType string `gorm:"type:varchar(32)"`
	ProjectGid   string `gorm:"type:varchar(255);index"`
	archived.NoPKModel
}

func (AsanaSection) TableName() string {
	return "_tool_asana_sections"
}

type AsanaUser struct {
	ConnectionId  uint64 `gorm:"primaryKey"`
	Gid           string `gorm:"primaryKey;type:varchar(255)"`
	Name          string `gorm:"type:varchar(255)"`
	Email         string `gorm:"type:varchar(255)"`
	ResourceType  string `gorm:"type:varchar(32)"`
	PhotoUrl      string `gorm:"type:varchar(512)"`
	WorkspaceGids string `gorm:"type:text"`
	archived.NoPKModel
}

func (AsanaUser) TableName() string {
	return "_tool_asana_users"
}

type AsanaWorkspace struct {
	ConnectionId   uint64 `gorm:"primaryKey"`
	Gid            string `gorm:"primaryKey;type:varchar(255)"`
	Name           string `gorm:"type:varchar(255)"`
	ResourceType   string `gorm:"type:varchar(32)"`
	IsOrganization bool   `json:"isOrganization"`
	archived.NoPKModel
}

func (AsanaWorkspace) TableName() string {
	return "_tool_asana_workspaces"
}

type AsanaTeam struct {
	ConnectionId    uint64 `gorm:"primaryKey"`
	Gid             string `gorm:"primaryKey;type:varchar(255)"`
	Name            string `gorm:"type:varchar(255)"`
	ResourceType    string `gorm:"type:varchar(32)"`
	Description     string `gorm:"type:text"`
	HtmlDescription string `gorm:"type:text"`
	OrganizationGid string `gorm:"type:varchar(255);index"`
	PermalinkUrl    string `gorm:"type:varchar(512)"`
	archived.NoPKModel
}

func (AsanaTeam) TableName() string {
	return "_tool_asana_teams"
}

type AsanaStory struct {
	ConnectionId    uint64    `gorm:"primaryKey"`
	Gid             string    `gorm:"primaryKey;type:varchar(255)"`
	ResourceType    string    `gorm:"type:varchar(32)"`
	ResourceSubtype string    `gorm:"type:varchar(64)"`
	Text            string    `gorm:"type:text"`
	HtmlText        string    `gorm:"type:text"`
	IsPinned        bool      `json:"isPinned"`
	IsEdited        bool      `json:"isEdited"`
	StickerName     string    `gorm:"type:varchar(64)"`
	CreatedAt       time.Time `json:"createdAt"`
	CreatedByGid    string    `gorm:"type:varchar(255)"`
	CreatedByName   string    `gorm:"type:varchar(255)"`
	TaskGid         string    `gorm:"type:varchar(255);index"`
	TargetGid       string    `gorm:"type:varchar(255);index"`
	archived.NoPKModel
}

func (AsanaStory) TableName() string {
	return "_tool_asana_stories"
}

type AsanaTag struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	Gid          string `gorm:"primaryKey;type:varchar(255)"`
	Name         string `gorm:"type:varchar(255)"`
	ResourceType string `gorm:"type:varchar(32)"`
	Color        string `gorm:"type:varchar(32)"`
	Notes        string `gorm:"type:text"`
	WorkspaceGid string `gorm:"type:varchar(255);index"`
	PermalinkUrl string `gorm:"type:varchar(512)"`
	archived.NoPKModel
}

func (AsanaTag) TableName() string {
	return "_tool_asana_tags"
}

type AsanaTaskTag struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	TaskGid      string `gorm:"primaryKey;type:varchar(255)"`
	TagGid       string `gorm:"primaryKey;type:varchar(255)"`
	archived.NoPKModel
}

func (AsanaTaskTag) TableName() string {
	return "_tool_asana_task_tags"
}

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
	archived.NoPKModel
}

func (AsanaCustomField) TableName() string {
	return "_tool_asana_custom_fields"
}

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
	archived.NoPKModel
}

func (AsanaTaskCustomFieldValue) TableName() string {
	return "_tool_asana_task_custom_field_values"
}

type AsanaProjectMembership struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	ProjectGid   string `gorm:"primaryKey;type:varchar(255)"`
	UserGid      string `gorm:"primaryKey;type:varchar(255)"`
	Role         string `gorm:"type:varchar(32)"`
	archived.NoPKModel
}

func (AsanaProjectMembership) TableName() string {
	return "_tool_asana_project_memberships"
}

type AsanaTeamMembership struct {
	ConnectionId uint64 `gorm:"primaryKey"`
	TeamGid      string `gorm:"primaryKey;type:varchar(255)"`
	UserGid      string `gorm:"primaryKey;type:varchar(255)"`
	IsGuest      bool   `json:"isGuest"`
	archived.NoPKModel
}

func (AsanaTeamMembership) TableName() string {
	return "_tool_asana_team_memberships"
}
