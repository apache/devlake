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
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.ToolLayerScope = (*BitbucketRepo)(nil)
var _ plugin.ApiGroup = (*GroupResponse)(nil)

type BitbucketRepo struct {
	common.Scope `mapstructure:",squash"`
	BitbucketId  string     `json:"bitbucketId" gorm:"primaryKey;type:varchar(255)" validate:"required" mapstructure:"bitbucketId"`
	Name         string     `json:"name" gorm:"type:varchar(255)" mapstructure:"name,omitempty"`
	HTMLUrl      string     `json:"HTMLUrl" gorm:"type:varchar(255)" mapstructure:"HTMLUrl,omitempty"`
	Description  string     `json:"description" mapstructure:"description,omitempty"`
	Owner        string     `json:"owner" mapstructure:"owner,omitempty"`
	Language     string     `json:"language" gorm:"type:varchar(255)" mapstructure:"language,omitempty"`
	CloneUrl     string     `json:"cloneUrl" gorm:"type:varchar(255)" mapstructure:"cloneUrl,omitempty"`
	CreatedDate  *time.Time `json:"createdDate" mapstructure:"-"`
	UpdatedDate  *time.Time `json:"updatedDate" mapstructure:"-"`
}

func (BitbucketRepo) TableName() string {
	return "_tool_bitbucket_repos"
}

func (p BitbucketRepo) ScopeId() string {
	return p.BitbucketId
}

func (p BitbucketRepo) ScopeName() string {
	return p.Name
}

func (p BitbucketRepo) ScopeFullName() string {
	return p.BitbucketId
}

func (p BitbucketRepo) ScopeParams() interface{} {
	return &BitbucketApiParams{
		ConnectionId: p.ConnectionId,
		FullName:     p.BitbucketId,
	}
}

type BitbucketApiRepo struct {
	//Scm         string `json:"scm"`
	//HasWiki     bool   `json:"has_wiki"`
	//Uuid        string `json:"uuid"`
	//Type        string `json:"type"`
	//HasIssue    bool   `json:"has_issue"`
	//ForkPolicy  string `json:"fork_policy"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Language    string `json:"language"`
	Description string `json:"description"`
	Owner       struct {
		Displayname string `json:"display_name"`
	} `json:"owner"`
	CreatedAt *time.Time `json:"created_on"`
	UpdatedAt *time.Time `json:"updated_on"`
	Links     struct {
		Clone []struct {
			Href string `json:"href"`
			Name string `json:"name"`
		} `json:"clone"`
		Html struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

func (b BitbucketApiRepo) ConvertApiScope() *BitbucketRepo {
	scope := &BitbucketRepo{}
	scope.BitbucketId = b.FullName
	scope.CreatedDate = b.CreatedAt
	scope.UpdatedDate = b.UpdatedAt
	scope.Language = b.Language
	scope.Description = b.Description
	scope.Name = b.Name
	scope.Owner = b.Owner.Displayname
	scope.HTMLUrl = b.Links.Html.Href

	scope.CloneUrl = ""
	for _, u := range b.Links.Clone {
		if u.Name == "https" {
			scope.CloneUrl = u.Href
		}
	}
	return scope
}

type WorkspaceResponse struct {
	Pagelen int             `json:"pagelen"`
	Page    int             `json:"page"`
	Size    int             `json:"size"`
	Values  []GroupResponse `json:"values"`
}

// GroupResponse maps an entry from GET /2.0/workspaces. The cross-workspace
// GET /2.0/user/permissions/workspaces was removed by Bitbucket CHANGE-2770,
// so slug/name now live at the top level instead of under a nested workspace.
type GroupResponse struct {
	Slug string `json:"slug" group:"id"`
	Name string `json:"name" group:"name"`
}

func (p GroupResponse) GroupId() string {
	return p.Slug
}

func (p GroupResponse) GroupName() string {
	return p.Name
}

type ReposResponse struct {
	Pagelen int                `json:"pagelen"`
	Page    int                `json:"page"`
	Size    int                `json:"size"`
	Values  []BitbucketApiRepo `json:"values"`
}

type BitbucketApiParams struct {
	ConnectionId uint64
	FullName     string
}
