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

package tasks

import (
	"encoding/json"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

// RAW_COMMITS_TABLE is defined in commits_collector.go

var ExtractCommitsMeta = plugin.SubTaskMeta{
	Name:             "ExtractCommits",
	EntryPoint:       ExtractCommits,
	EnabledByDefault: true,
	Description:      "Extract commits from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	DependencyTables: []string{RAW_COMMITS_TABLE},
}

func ExtractCommits(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMMITS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			var commit struct {
				Commitid  string `json:"commitid"`
				Branch    string `json:"branch"`
				Message   string `json:"message"`
				Parent    string `json:"parent"` // Parent commit SHA for comparisons
				Author    struct {
					Name string `json:"name"`
				} `json:"author"`
				Timestamp string `json:"timestamp"`
			}
			err := errors.Convert(json.Unmarshal(resData.Data, &commit))
			if err != nil {
				return nil, err
			}

			var commitTimestamp *time.Time
			if commit.Timestamp != "" {
				parsed, err := time.Parse(time.RFC3339, commit.Timestamp)
				if err == nil {
					commitTimestamp = &parsed
				}
			}

			codecovCommit := &models.CodecovCommit{
				NoPKModel:       common.NoPKModel{},
				ConnectionId:    data.Options.ConnectionId,
				RepoId:          data.Options.FullName,
				CommitSha:       commit.Commitid,
				Branch:          commit.Branch,
				CommitTimestamp: commitTimestamp,
				Message:         commit.Message,
				Author:          commit.Author.Name,
				ParentSha:       commit.Parent, // Store parent SHA for comparison API calls
			}

			return []interface{}{codecovCommit}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
