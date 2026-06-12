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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractAccountsMeta)
}

var ExtractAccountsMeta = plugin.SubTaskMeta{
	Name:             "Extract Users",
	EntryPoint:       ExtractAccounts,
	EnabledByDefault: true,
	Description:      "Extract raw account data  into tool layer table github_accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{RAW_ACCOUNT_TABLE},
	ProductTables:    []string{models.GithubAccount{}.TableName()},
}

type DetailGithubAccountResponse struct {
	GithubAccountResponse
	Name            string    `json:"name"`
	Company         string    `json:"company"`
	Blog            string    `json:"blog"`
	Location        string    `json:"location"`
	Email           string    `json:"email"`
	Hireable        bool      `json:"hireable"`
	Bio             string    `json:"bio"`
	TwitterUsername string    `json:"twitter_username"`
	PublicRepos     int       `json:"public_repos"`
	PublicGists     int       `json:"public_gists"`
	Followers       int       `json:"followers"`
	Following       int       `json:"following"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func ExtractAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[DetailGithubAccountResponse]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_ACCOUNT_TABLE,
		},
		Extract: func(body *DetailGithubAccountResponse, row *api.RawData) ([]any, errors.Error) {
			if body.Id == 0 {
				return nil, nil
			}
			githubAccount := &models.GithubAccount{
				ConnectionId: data.Options.ConnectionId,
				Id:           body.Id,
				Login:        body.Login,
				Name:         body.Name,
				Company:      body.Company,
				Email:        body.Email,
				AvatarUrl:    body.AvatarUrl,
				Url:          body.Url,
				HtmlUrl:      body.HtmlUrl,
				Type:         body.Type,
			}
			return []any{githubAccount}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
