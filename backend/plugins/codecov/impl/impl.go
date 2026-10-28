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
	"fmt"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	coreModels "github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/api"
	"github.com/apache/devlake/plugins/codecov/models"
	"github.com/apache/devlake/plugins/codecov/models/migrationscripts"
	"github.com/apache/devlake/plugins/codecov/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginSource
	plugin.DataSourcePluginBlueprintV200
} = (*Codecov)(nil)

type Codecov struct{}

func (p Codecov) Connection() dal.Tabler {
	return &models.CodecovConnection{}
}

func (p Codecov) Scope() plugin.ToolLayerScope {
	return &models.CodecovRepo{}
}

func (p Codecov) ScopeConfig() dal.Tabler {
	return &models.CodecovScopeConfig{}
}

func (p Codecov) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Codecov) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.CodecovConnection{},
		&models.CodecovRepo{},
		&models.CodecovScopeConfig{},
		&models.CodecovFlag{},
		&models.CodecovCommit{},
		&models.CodecovCoverage{},
		&models.CodecovCoverageTrend{},
		&models.CodecovCommitCoverage{},
		&models.CodecovRepoConfig{},
	}
}

func (p Codecov) Description() string {
	return "To collect and enrich data from Codecov"
}

func (p Codecov) Name() string {
	return "codecov"
}

func (p Codecov) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/codecov"
}

func (p Codecov) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Codecov) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"test": {
			"POST": api.TestConnection,
		},
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.ListConnections,
		},
		"connections/:connectionId": {
			"GET":    api.GetConnection,
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
		},
		"connections/:connectionId/test": {
			"POST": api.TestExistingConnection,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopes,
			"PUT": api.PutScopes,
		},
		"connections/:connectionId/scopes/*scopeId": {
			// Behind 'GetScopeDispatcher', there are two paths so far:
			// GetScopeLatestSyncState "connections/:connectionId/scopes/:scopeId/latest-sync-state"
			// GetScope "connections/:connectionId/scopes/:scopeId"
			// Because there may be slash in scopeId (codecovId like "owner/repo"), so we handle it manually.
			"GET":    api.GetScopeDispatcher,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scope-configs": {
			"POST": api.PostScopeConfig,
			"GET":  api.GetScopeConfigList,
		},
		"connections/:connectionId/scope-configs/:id": {
			"GET":    api.GetScopeConfig,
			"PATCH":  api.PatchScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
		},
		"connections/:connectionId/search-remote-scopes": {
			"GET": api.SearchRemoteScopes,
		},
	}
}

func (p Codecov) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		// Step 1: Collect and convert flags first (needed for flag-based coverage collection)
		tasks.CollectFlagsMeta,
		tasks.ConvertFlagsMeta,
		// Step 2: Collect commits
		tasks.CollectCommitsMeta,
		tasks.ExtractCommitsMeta,
		// Step 3: Collect coverage data (depends on flags and commits)
		tasks.CollectCommitTotalsMeta,
		tasks.CollectCommitCoverageMeta,
		tasks.CollectComparisonMeta,
		tasks.CollectFlagCoverageTrendMeta,
		// Step 4: Collect repo config (codecov.yml) from the code host
		tasks.CollectRepoConfigMeta,
		// Step 5: Convert coverage data
		tasks.ConvertComparisonMeta,
		tasks.ConvertCoverageMeta,
		tasks.ConvertCommitCoverageMeta,
		tasks.ConvertCoverageTrendMeta,
	}
}

func (p Codecov) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}

	db := taskCtx.GetDal()
	connection := &models.CodecovConnection{}
	err = db.First(connection, dal.Where("id = ?", op.ConnectionId))
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "unable to get Codecov connection by the given connection ID")
	}

	// Create synchronize API client (PrepareApiClient on connection will set headers)
	apiClient, err := helper.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}

	// Create async API client with rate limiter optimized for Codecov
	// Note: Go's http.Client already has connection pooling via http.Transport
	rateLimiter := &helper.ApiRateLimitCalculator{
		UserRateLimitPerHour: 5000, // Codecov's rate limit (adjust based on your plan)
	}

	asyncApiClient, err := helper.CreateAsyncApiClient(
		taskCtx,
		apiClient,
		rateLimiter,
	)
	if err != nil {
		return nil, err
	}

	taskCtx.GetLogger().Info("[Codecov] API client initialized with rate limiter (5000 req/hour)")

	service := connection.ServiceOrDefault()
	if validErr := connection.ValidateService(); validErr != nil {
		return nil, validErr
	}

	// Load the CodecovRepo scope to get branch and other metadata
	repo := &models.CodecovRepo{}
	err = db.First(repo, dal.Where("connection_id = ? AND codecov_id = ?", op.ConnectionId, op.FullName))
	if err != nil {
		taskCtx.GetLogger().Warn(err, "unable to load CodecovRepo scope for %s, branch will default to 'main'", op.FullName)
		repo = nil
	}

	// Auto-detect the default branch from the Codecov API
	owner, repoName, parseErr := tasks.ParseFullName(op.FullName)
	if parseErr != nil {
		taskCtx.GetLogger().Warn(parseErr, "[Codecov] Failed to parse fullName '%s', branch detection skipped", op.FullName)
	} else {
		repoUrl := fmt.Sprintf("/%s/", tasks.RepoAPIPrefix(service, owner, repoName))
		res, apiErr := apiClient.Get(repoUrl, nil, nil)
		if apiErr != nil {
			taskCtx.GetLogger().Warn(apiErr, "[Codecov] Failed to fetch repo detail for %s, using stored branch", op.FullName)
		} else if res.StatusCode != 200 {
			taskCtx.GetLogger().Warn(nil, "[Codecov] Repo detail API returned status %d for %s, using stored branch", res.StatusCode, op.FullName)
		} else {
			var repoDetail struct {
				Branch string `json:"branch"`
			}
			if unmarshalErr := helper.UnmarshalResponse(res, &repoDetail); unmarshalErr != nil {
				taskCtx.GetLogger().Warn(unmarshalErr, "[Codecov] Failed to parse repo detail response for %s", op.FullName)
			} else if repoDetail.Branch != "" {
				if repo != nil && repo.Branch != repoDetail.Branch {
					taskCtx.GetLogger().Info("[Codecov] Default branch updated: %s -> %s for %s", repo.Branch, repoDetail.Branch, op.FullName)
					repo.Branch = repoDetail.Branch
					if updateErr := db.Update(repo); updateErr != nil {
						taskCtx.GetLogger().Warn(updateErr, "[Codecov] Failed to persist branch update for %s", op.FullName)
					}
				} else if repo != nil && repo.Branch == "" {
					repo.Branch = repoDetail.Branch
					if updateErr := db.Update(repo); updateErr != nil {
						taskCtx.GetLogger().Warn(updateErr, "[Codecov] Failed to persist branch update for %s", op.FullName)
					}
				} else if repo == nil {
					// Scope record not found in DB but API returned branch info;
					// create a minimal in-memory repo so collectors can use the detected branch
					repo = &models.CodecovRepo{
						CodecovId: op.FullName,
						FullName:  op.FullName,
						Name:      repoName,
						Branch:    repoDetail.Branch,
					}
					taskCtx.GetLogger().Info("[Codecov] No scope record found, using API-detected branch '%s' for %s", repoDetail.Branch, op.FullName)
				}
			}
		}
	}

	return &tasks.CodecovTaskData{
		Options:   op,
		ApiClient: asyncApiClient,
		Repo:      repo,
		Service:   service,
	}, nil
}

func (p Codecov) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}
