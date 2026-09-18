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
	"reflect"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/github/models"
	githubTasks "github.com/apache/devlake/plugins/github/tasks"
	"github.com/merico-ai/graphql"
)

const RAW_ACCOUNTS_TABLE = "github_graphql_accounts"

type GraphqlQueryAccountWrapper struct {
	RateLimit struct {
		Cost int
	}
	Users []GraphqlQueryAccount `graphql:"user(login: $login)" graphql-extend:"true"`
}

type GraphqlQueryAccount struct {
	Login         string
	Id            int `graphql:"databaseId"`
	Name          string
	Company       string
	Email         string
	AvatarUrl     string
	HtmlUrl       string `graphql:"url"`
	Organizations struct {
		Nodes []struct {
			Email      string
			Name       string
			DatabaseId int
			Login      string
		}
	} `graphql:"organizations(first: 10)"`
}

var CollectAccountMeta = plugin.SubTaskMeta{
	Name:             "Collect Users",
	EntryPoint:       CollectAccount,
	EnabledByDefault: true,
	Description:      "Collect Account data from GithubGraphql api, does not support either timeFilter or diffSync.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}

type SimpleAccount struct {
	Login string
}

var _ plugin.SubTaskEntryPoint = CollectAccount

func CollectAccount(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*githubTasks.GithubTaskData)

	// `user(login:)` resolves Users only, a bot login always comes back as
	// "Could not resolve to a User", so bots are left out of this lookup. They are
	// still converted to accounts: ConvertAccounts reads _tool_github_repo_accounts
	// and only enriches it from _tool_github_accounts when a row is there.
	cursor, err := db.Cursor(
		dal.Select("login"),
		dal.Orderby("login ASC"),
		dal.From(models.GithubRepoAccount{}.TableName()),
		dal.Where(
			"repo_github_id = ? and connection_id = ? and login not like ?",
			data.Options.GithubId, data.Options.ConnectionId, "%"+botLoginSuffix,
		),
	)
	if err != nil {
		return err
	}
	iterator, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(SimpleAccount{}))
	if err != nil {
		return err
	}

	collector, err := helper.NewGraphqlCollector(helper.GraphqlCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: githubTasks.GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_ACCOUNTS_TABLE,
		},
		Input:         iterator,
		InputStep:     100,
		GraphqlClient: data.GraphqlClient,
		BuildQuery: func(reqData *helper.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
			query := &GraphqlQueryAccountWrapper{}
			if reqData == nil {
				return query, map[string]interface{}{}, nil
			}
			accounts := reqData.Input.([]interface{})
			users := []map[string]interface{}{}
			for _, iAccount := range accounts {
				account := iAccount.(*SimpleAccount)
				users = append(users, map[string]interface{}{
					`login`: graphql.String(account.Login),
				})
			}
			variables := map[string]interface{}{
				"user": users,
			}
			return query, variables, nil
		},
		ResponseParser: func(queryWrapper any) (messages []json.RawMessage, err errors.Error) {
			query := queryWrapper.(*GraphqlQueryAccountWrapper)
			for _, rawL := range query.Users {
				messages = append(messages, errors.Must1(json.Marshal(rawL)))
			}
			return
		},
		IgnoreQueryErrors: true,
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}
