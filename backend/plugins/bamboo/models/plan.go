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
	"strings"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.ToolLayerScope = (*BambooPlan)(nil)
var _ plugin.ApiScope = (*ApiBambooPlan)(nil)

type BambooPlan struct {
	common.Scope              `mapstructure:",squash"`
	PlanKey                   string  `json:"planKey" mapstructure:"planKey" gorm:"primaryKey"`
	Name                      string  `json:"name" mapstructure:"name"`
	Expand                    string  `json:"expand" mapstructure:"expand"`
	ProjectKey                string  `json:"projectKey" mapstructure:"projectKey" gorm:"index"`
	ProjectName               string  `json:"projectName" mapstructure:"projectName"`
	Description               string  `json:"description" mapstructure:"description"`
	ShortName                 string  `json:"shortName" mapstructure:"shortName"`
	BuildName                 string  `json:"buildName" mapstructure:"buildName"`
	ShortKey                  string  `json:"shortKey" mapstructure:"shortKey"`
	Type                      string  `json:"type" mapstructure:"type"`
	Enabled                   bool    `json:"enabled" mapstructure:"enabled"`
	Href                      string  `json:"href" mapstructure:"href"`
	Rel                       string  `json:"rel" mapstructure:"rel"`
	IsFavourite               bool    `json:"isFavourite" mapstructure:"isFavourite"`
	IsActive                  bool    `json:"isActive" mapstructure:"isActive"`
	IsBuilding                bool    `json:"isBuilding" mapstructure:"isBuilding"`
	AverageBuildTimeInSeconds float64 `json:"averageBuildTimeInSeconds" mapstructure:"averageBuildTimeInSeconds"`
}

func (p BambooPlan) ScopeId() string {
	return p.PlanKey
}

func (p BambooPlan) ScopeName() string {
	return p.Name
}

func (p BambooPlan) ScopeFullName() string {
	return p.Name
}

func (p BambooPlan) ScopeParams() interface{} {
	return &BambooApiParams{
		ConnectionId: p.ConnectionId,
		PlanKey:      p.PlanKey,
	}
}

func (p BambooPlan) TableName() string {
	return "_tool_bamboo_plans"
}

type ApiBambooPlan struct {
	ShortName string `json:"shortName"`
	ShortKey  string `json:"shortKey"`
	Type      string `json:"type"`
	Enabled   bool   `json:"enabled"`
	Link      struct {
		Href string `json:"href"`
		Rel  string `json:"rel"`
	} `json:"link"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	PlanKey struct {
		Key string `json:"key"`
	} `json:"planKey"`
}

func (p ApiBambooPlan) ConvertApiScope() plugin.ToolLayerScope {
	return &BambooPlan{
		PlanKey:   p.Key,
		Name:      p.Name,
		ShortName: p.ShortName,
		ShortKey:  p.ShortKey,
		Type:      p.Type,
		Enabled:   p.Enabled,
		Href:      p.Link.Href,
		Rel:       p.Link.Rel,
	}
}

type SearchEntity struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	ProjectName string `json:"projectName"`
	PlanName    string `json:"planName"`
	BranchName  string `json:"branchName"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

// Name tries to keep plan's name field the same with name in /remote-scopes.
// In /remote-scopes, plan's name is "{projectName - planName}".
func (entity SearchEntity) Name() string {
	return strings.Join([]string{entity.ProjectName, entity.PlanName}, " - ")
}

type ApiSearchResult struct {
	Id           string       `json:"id"`
	Type         string       `json:"type"`
	SearchEntity SearchEntity `json:"searchEntity"`
}

type ApiBambooSearchPlanResponse struct {
	ApiBambooSizeData `json:"squash"`
	SearchResults     []ApiSearchResult `json:"searchResults"`
}
