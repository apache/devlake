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

type CircleciTrigger struct {
	Type       string `json:"type"`
	ReceivedAt string `json:"received_at"`
	Actor      struct {
		Login     string `json:"login"`
		AvatarUrl string `json:"avatar_url"`
	} `json:"actor"`
}

type CircleciVcs struct {
	ProviderName        string `json:"provider_name"`
	TargetRepositoryUrl string `json:"target_repository_url"`
	Branch              string `json:"branch"`
	ReviewId            string `json:"review_id"`
	ReviewUrl           string `json:"review_url"`
	Revision            string `json:"revision"`
	Tag                 string `json:"tag"`
	OriginRepositoryUrl string `json:"origin_repository_url"`
	Commit              struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	} `json:"commit"`
}

type CircleciPipeline struct {
	ConnectionId      uint64              `gorm:"primaryKey;type:BIGINT" json:"connectionId" mapstructure:"connectionId"`
	Id                string              `gorm:"primaryKey;type:varchar(100)" json:"id" mapstructure:"id"`
	ProjectSlug       string              `gorm:"type:varchar(255)" json:"projectSlug" mapstructure:"projectSlug"`
	UpdatedDate       *common.Iso8601Time `json:"updated_at"`
	CreatedDate       *common.Iso8601Time `json:"created_at"`
	Number            int64               `json:"number" mapstructure:"number"` // pipeline number within the project?
	TriggerParameters any                 `gorm:"serializer:json" json:"trigger_parameters" mapstructure:"triggerParameters"`
	State             string              `gorm:"type:varchar(100)" json:"state" mapstructure:"state"`
	Trigger           CircleciTrigger     `gorm:"serializer:json" json:"trigger"`
	Vcs               CircleciVcs         `gorm:"serializer:json" json:"vcs"`
	common.NoPKModel  `swaggerignore:"true" json:"-" mapstructure:"-"`
}

func (CircleciPipeline) TableName() string {
	return "_tool_circleci_pipelines"
}
