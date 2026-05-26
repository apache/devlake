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
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractApiEventsMeta)
}

var ExtractApiEventsMeta = plugin.SubTaskMeta{
	Name:             "Extract Events",
	EntryPoint:       ExtractApiEvents,
	EnabledByDefault: true,
	Description:      "Extract raw Events data into tool layer table github_issue_events",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{RAW_EVENTS_TABLE},
	ProductTables:    []string{models.GithubIssueEvent{}.TableName()},
}

type IssueEvent struct {
	GithubId int `json:"id"`
	Event    string
	Actor    *GithubAccountResponse
	Issue    struct {
		Id int
	}
	GithubCreatedAt common.Iso8601Time `json:"created_at"`
}

func ExtractApiEvents(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)

	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[IssueEvent]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_EVENTS_TABLE,
		},
		Extract: func(body *IssueEvent, row *api.RawData) ([]any, errors.Error) {
			results := make([]interface{}, 0, 1)
			if body.GithubId == 0 || body.Actor == nil {
				return nil, nil
			}
			githubIssueEvent := &models.GithubIssueEvent{
				ConnectionId:    data.Options.ConnectionId,
				GithubId:        body.GithubId,
				IssueId:         body.Issue.Id,
				Type:            body.Event,
				GithubCreatedAt: body.GithubCreatedAt.ToTime(),
			}
			if body.Actor != nil {
				githubIssueEvent.AuthorUsername = body.Actor.Login
				githubAccount, err := convertAccount(body.Actor, data.Options.GithubId, data.Options.ConnectionId)
				if err != nil {
					return nil, err
				}
				results = append(results, githubAccount)
			}
			results = append(results, githubIssueEvent)
			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
