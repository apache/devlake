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

package impl

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/apache/incubator-devlake/core/models/domainlayer/devops"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	githubImpl "github.com/apache/incubator-devlake/plugins/github/impl"
	"github.com/apache/incubator-devlake/plugins/github/models"
	githubTasks "github.com/apache/incubator-devlake/plugins/github/tasks"
	"github.com/apache/incubator-devlake/plugins/github_graphql/model/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/github_graphql/tasks"
	"github.com/merico-ai/graphql"
)

// make sure interface is implemented
var _ interface {
	plugin.PluginMeta
	plugin.PluginTask
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginSource
	plugin.CloseablePluginTask
	plugin.PluginMigration
} = (*GithubGraphql)(nil)

type GithubGraphql struct{}

func (p GithubGraphql) Connection() dal.Tabler {
	return &models.GithubConnection{}
}

func (p GithubGraphql) Scope() plugin.ToolLayerScope {
	return &models.GithubRepo{}
}

func (p GithubGraphql) ScopeConfig() dal.Tabler {
	return &models.GithubScopeConfig{}
}

func (p GithubGraphql) Description() string {
	return "collect some GithubGraphql data"
}

func (p GithubGraphql) Name() string {
	return "github_graphql"
}

func (p GithubGraphql) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.GithubDeployment{},
	}
}

func (p GithubGraphql) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		// collect millstones
		githubTasks.CollectMilestonesMeta,
		githubTasks.ExtractMilestonesMeta,

		// collect issue & pr, deps on millstone
		tasks.CollectIssuesMeta,
		tasks.ExtractIssuesMeta,
		tasks.CollectPrsMeta,
		tasks.ExtractPrsMeta,

		// collect workflow run & job
		githubTasks.CollectRunsMeta,
		githubTasks.ExtractRunsMeta,
		tasks.CollectJobsMeta,
		tasks.ExtractJobsMeta,

		// collect others
		githubTasks.CollectApiCommentsMeta,
		githubTasks.ExtractApiCommentsMeta,
		githubTasks.CollectApiEventsMeta,
		githubTasks.ExtractApiEventsMeta,
		githubTasks.CollectApiPrReviewCommentsMeta,
		githubTasks.ExtractApiPrReviewCommentsMeta,

		// collect account, deps on all before
		tasks.CollectAccountMeta,
		tasks.ExtractAccountsMeta,

		// convert to domain layer
		githubTasks.ConvertRunsMeta,
		githubTasks.ConvertJobsMeta,
		githubTasks.EnrichPullRequestIssuesMeta,
		githubTasks.ConvertRepoMeta,
		githubTasks.ConvertIssuesMeta,
		githubTasks.ConvertCommitsMeta,
		githubTasks.ConvertIssueLabelsMeta,
		githubTasks.ConvertPullRequestCommitsMeta,
		githubTasks.ConvertPullRequestsMeta,
		githubTasks.ConvertPullRequestReviewsMeta,
		githubTasks.ConvertPullRequestLabelsMeta,
		githubTasks.ConvertPullRequestIssuesMeta,
		githubTasks.ConvertIssueAssigneeMeta,
		githubTasks.ConvertIssueCommentsMeta,
		githubTasks.ConvertPullRequestCommentsMeta,
		githubTasks.ConvertReviewsMeta,
		githubTasks.ConvertMilestonesMeta,
		githubTasks.ConvertAccountsMeta,

		// deployment
		tasks.CollectDeploymentsMeta,
		tasks.ExtractDeploymentsMeta,
		tasks.ConvertDeploymentsMeta,

		// releases
		tasks.CollectReleaseMeta,
		tasks.ExtractReleasesMeta,
		githubTasks.ConvertReleasesMeta,
	}
}

type GraphQueryRateLimit struct {
	RateLimit *struct {
		Limit     graphql.Int
		Remaining graphql.Int
		ResetAt   time.Time
	}
}

func (p GithubGraphql) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	logger := taskCtx.GetLogger()
	logger.Debug("%v", options)
	var op githubTasks.GithubOptions
	err := helper.Decode(options, &op, nil)
	if err != nil {
		return nil, err
	}
	connectionHelper := helper.NewConnectionHelper(
		taskCtx,
		nil,
		p.Name(),
	)
	connection := &models.GithubConnection{}
	err = connectionHelper.FirstById(connection, op.ConnectionId)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get github connection by the given connection ID: %v")
	}

	apiClient, err := githubTasks.CreateApiClient(taskCtx, connection)
	if err != nil {
		return nil, errors.Default.Wrap(err, "unable to get github API client instance")
	}

	err = githubImpl.EnrichOptions(taskCtx, &op, apiClient.ApiClient)
	if err != nil {
		return nil, err
	}

	graphqlClient, err := tasks.CreateGraphqlClient(
		taskCtx,
		connection,
		apiClient.ApiClient.GetClient(),
		func(ctx context.Context, client *graphql.Client, logger log.Logger) (rateRemaining int, resetAt *time.Time, err errors.Error) {
			var query GraphQueryRateLimit
			dataErrors, err := errors.Convert01(client.Query(taskCtx.GetContext(), &query, nil))
			if err != nil {
				return 0, nil, err
			}
			if len(dataErrors) > 0 {
				return 0, nil, errors.Default.Wrap(dataErrors[0], `query rate limit fail`)
			}
			if query.RateLimit == nil {
				return 0, nil, errors.Default.New("rate limit unavailable")
			}
			logger.Info(`github graphql init success with remaining %d/%d and will reset at %s`,
				query.RateLimit.Remaining, query.RateLimit.Limit, query.RateLimit.ResetAt)
			return int(query.RateLimit.Remaining), &query.RateLimit.ResetAt, nil
		})
	if err != nil {
		return nil, err
	}

	graphqlClient.SetGetRateCost(func(q interface{}) int {
		v := reflect.ValueOf(q)
		return int(v.Elem().FieldByName(`RateLimit`).FieldByName(`Cost`).Int())
	})

	regexEnricher := helper.NewRegexEnricher()
	if err = regexEnricher.TryAdd(devops.DEPLOYMENT, op.ScopeConfig.DeploymentPattern); err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid value for `deploymentPattern`")
	}
	if err = regexEnricher.TryAdd(devops.PRODUCTION, op.ScopeConfig.ProductionPattern); err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid value for `productionPattern`")
	}
	if err = regexEnricher.TryAdd(devops.ENV_NAME_PATTERN, op.ScopeConfig.EnvNamePattern); err != nil {
		return nil, errors.BadInput.Wrap(err, "invalid value for `envNamePattern`")
	}

	taskData := &githubTasks.GithubTaskData{
		Options:       &op,
		ApiClient:     apiClient,
		GraphqlClient: graphqlClient,
		RegexEnricher: regexEnricher,
	}

	return taskData, nil
}

// RootPkgPath information lost when compiled as plugin(.so)
func (p GithubGraphql) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/githubGraphql"
}

func (p GithubGraphql) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return nil
}

func (p GithubGraphql) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*githubTasks.GithubTaskData)
	if !ok {
		return errors.Default.New(fmt.Sprintf("GetData failed when try to close %+v", taskCtx))
	}
	data.ApiClient.Release()
	data.GraphqlClient.Release()
	return nil
}

func (p GithubGraphql) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}
