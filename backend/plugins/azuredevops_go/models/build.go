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
	"time"
)

type AzuredevopsBuild struct {
	common.NoPKModel

	ConnectionId  uint64 `gorm:"primaryKey"`
	AzuredevopsId int    `json:"id" gorm:"primaryKey"`
	RepositoryId  string
	Status        string
	Result        string
	Name          string
	RunName       string
	SourceBranch  string
	SourceVersion string
	// Tags is a string version of the APIs tags array that helps to identify
	// devops.CICDPipeline's environment and type.
	Tags       string
	QueueTime  *time.Time
	StartTime  *time.Time
	FinishTime *time.Time
}

func (AzuredevopsBuild) TableName() string {
	return "_tool_azuredevops_go_builds"
}

type AzuredevopsApiBuild struct {
	Id          int        `json:"id"`
	BuildNumber string     `json:"buildNumber"`
	Status      string     `json:"status"`
	Result      string     `json:"result"`
	QueueTime   *time.Time `json:"queueTime"`
	StartTime   *time.Time `json:"startTime"`
	FinishTime  *time.Time `json:"finishTime"`
	Url         string     `json:"url"`
	Definition  struct {
		Drafts      []interface{} `json:"drafts"`
		Id          int           `json:"id"`
		Name        string        `json:"name"`
		Url         string        `json:"url"`
		Uri         string        `json:"uri"`
		Path        string        `json:"path"`
		Type        string        `json:"type"`
		QueueStatus string        `json:"queueStatus"`
		Revision    int           `json:"revision"`
	} `json:"definition"`
	BuildNumberRevision int      `json:"buildNumberRevision"`
	Uri                 string   `json:"uri"`
	SourceBranch        string   `json:"sourceBranch"`
	SourceVersion       string   `json:"sourceVersion"`
	Priority            string   `json:"priority"`
	Reason              string   `json:"reason"`
	Tags                []string `json:"tags"`
}
