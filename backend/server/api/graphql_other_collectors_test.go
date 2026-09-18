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

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/runner"
	collector "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/helpers/unithelper"
	"github.com/apache/devlake/impls/dalgorm"
	"github.com/apache/devlake/impls/logruslog"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	githubmodels "github.com/apache/devlake/plugins/github/models"
	githubtasks "github.com/apache/devlake/plugins/github/tasks"
	githubgraphql "github.com/apache/devlake/plugins/github_graphql/tasks"
	linear "github.com/apache/devlake/plugins/linear/tasks"
	"github.com/merico-ai/graphql"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGraphqlOtherCollectorsResume(t *testing.T) {
	dbURL := os.Getenv("DEVLAKE_SERVER_CRASH_DB_URL")
	if dbURL == "" {
		t.Skip("dedicated database not configured")
	}
	cfg := config.GetConfig()
	cfg.Set("DB_URL", dbURL)
	db, err := runner.NewGormDb(cfg, logruslog.Global)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CollectorLatestState{}, &githubmodels.GithubPullRequest{}, &githubmodels.GithubIssue{}))
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&githubmodels.GithubPullRequest{ConnectionId: 1, GithubId: 2, RepoId: 2, Number: 2, State: "OPEN", GithubCreatedAt: old, GithubUpdatedAt: old}).Error)
	require.NoError(t, db.Create(&githubmodels.GithubIssue{ConnectionId: 1, GithubId: 2, RepoId: 2, Number: 2, State: "OPEN", GithubCreatedAt: old, GithubUpdatedAt: old}).Error)
	for i, kind := range []string{"pr", "issue", "linear"} {
		t.Run(kind, func(t *testing.T) {
			var mu sync.Mutex
			fail := true
			var trace []string
			detailPattern := regexp.MustCompile(`(\w+)\s*:\s*(?:pullRequest|issue)\s*\(`)
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Query     string
					Variables map[string]interface{}
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				cursor, _ := req.Variables["skipCursor"].(string)
				detail := detailPattern.FindStringSubmatch(req.Query)
				stage := "list"
				if len(detail) > 0 || cursor != "" {
					stage = "detail"
				}
				mu.Lock()
				trace = append(trace, stage)
				shouldFail := fail && stage == "detail"
				mu.Unlock()
				if shouldFail {
					http.Error(w, "injected interruption", 503)
					return
				}
				node := map[string]interface{}{"databaseId": 1, "number": 1, "title": "first", "updatedAt": "2026-01-01T00:00:00Z", "createdAt": "2026-01-01T00:00:00Z"}
				data := map[string]interface{}{"rateLimit": map[string]int{"cost": 1}}
				if kind == "linear" {
					node = map[string]interface{}{"id": "one", "number": 1, "title": "first", "updatedAt": "2026-01-01T00:00:00Z"}
					if cursor != "" {
						node["id"] = "two"
						node["number"] = 2
					}
					data = map[string]interface{}{"team": map[string]interface{}{"issues": map[string]interface{}{"nodes": []interface{}{node}, "pageInfo": map[string]interface{}{"endCursor": "next", "hasNextPage": cursor == ""}}}}
				} else if len(detail) > 0 {
					node["databaseId"] = 2
					node["number"] = 2
					node["title"] = "second"
					data["repository"] = map[string]interface{}{detail[1]: node}
				} else {
					field := "issues"
					if kind == "pr" {
						field = "pullRequests"
					}
					data["repository"] = map[string]interface{}{field: map[string]interface{}{"nodes": []interface{}{node}, "pageInfo": map[string]interface{}{"endCursor": "", "hasNextPage": false}, "totalCount": 1}}
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
			}))
			defer upstream.Close()
			execute := func() error {
				ctx, cancel := context.WithCancel(plugin.WithTaskID(context.Background(), uint64(900+i)))
				defer cancel()
				taskCtx := unithelper.DummySubTaskContext(dalgorm.NewDalgorm(db))
				taskCtx.On("GetContext").Return(ctx)
				parent := taskCtx.TaskContext().(*mockplugin.TaskContext)
				parent.On("GetContext").Return(ctx)
				parent.On("GetConfig", mock.Anything).Return("")
				client, err := collector.CreateAsyncGraphqlClient(parent, graphql.NewClient(upstream.URL, upstream.Client()), logruslog.Global, nil)
				require.NoError(t, err)
				defer client.Release()
				client.SetMaxRetry(1, 0)
				if kind == "linear" {
					taskCtx.On("GetData").Return(&linear.LinearTaskData{Options: &linear.LinearOptions{ConnectionId: 1, TeamId: "team"}, GraphqlClient: client})
					return linear.CollectIssues(taskCtx)
				}
				taskCtx.On("GetData").Return(&githubtasks.GithubTaskData{Options: &githubtasks.GithubOptions{ConnectionId: 1, GithubId: 2, Name: "fixture/repo"}, GraphqlClient: client})
				if kind == "pr" {
					return githubgraphql.CollectPrs(taskCtx)
				}
				return githubgraphql.CollectIssues(taskCtx)
			}
			require.Error(t, execute())
			mu.Lock()
			fail = false
			mu.Unlock()
			require.NoError(t, execute())
			mu.Lock()
			require.Equal(t, []string{"list", "detail", "detail"}, trace)
			mu.Unlock()
			table := "_raw_github_graphql_issues"
			if kind == "pr" {
				table = "_raw_github_graphql_prs"
			}
			if kind == "linear" {
				table = "_raw_linear_issues"
			}
			var rows []collector.RawData
			require.NoError(t, db.Table(table).Order("id").Find(&rows).Error)
			require.Len(t, rows, 2)
			for j, row := range rows {
				var value map[string]interface{}
				require.NoError(t, json.Unmarshal(row.Data, &value))
				numbers := map[string]interface{}{}
				for key, v := range value {
					numbers[strings.ToLower(key)] = v
				}
				require.EqualValues(t, j+1, numbers["number"])
			}
		})
	}
}
