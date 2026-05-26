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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ExtractAccountOrgMeta)
}

var ExtractAccountOrgMeta = plugin.SubTaskMeta{
	Name:             "Extract User Org",
	EntryPoint:       ExtractAccountOrg,
	EnabledByDefault: true,
	Description:      "Extract raw account org data into tool layer table github_account_orgs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{RAW_ACCOUNT_ORG_TABLE},
	ProductTables:    []string{models.GithubAccountOrg{}.TableName()},
}

type GithubAccountOrgsResponse struct {
	Login       string `json:"login"`
	Id          int    `json:"id"`
	NodeId      string `json:"node_id"`
	AvatarUrl   string `json:"avatar_url"`
	Description string `json:"description"`
}

func ExtractAccountOrg(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*GithubTaskData)
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[[]GithubAccountOrgsResponse]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_ACCOUNT_ORG_TABLE,
		},
		Extract: func(body *[]GithubAccountOrgsResponse, row *api.RawData) ([]any, errors.Error) {
			simpleAccount := &SimpleAccountWithId{}
			err := errors.Convert(json.Unmarshal(row.Input, simpleAccount))
			if err != nil {
				return nil, err
			}
			results := make([]interface{}, 0, len(*body))
			for _, apiAccountOrg := range *body {
				githubAccount := &models.GithubAccountOrg{
					ConnectionId: data.Options.ConnectionId,
					AccountId:    simpleAccount.AccountId,
					OrgId:        apiAccountOrg.Id,
					OrgLogin:     apiAccountOrg.Login,
				}
				results = append(results, githubAccount)
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
