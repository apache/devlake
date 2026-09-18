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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/runner"
	collector "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/impls/logruslog"
	githubmodels "github.com/apache/devlake/plugins/github/models"
	githubtasks "github.com/apache/devlake/plugins/github/tasks"
	jobs "github.com/apache/devlake/plugins/github_graphql/tasks"
	server "github.com/apache/devlake/server/api"
	"github.com/apache/devlake/server/services"
	"github.com/merico-ai/graphql"
	"github.com/stretchr/testify/require"
)

type githubJobsResumePlugin struct{ endpoint, phase, scenario string }

func (p githubJobsResumePlugin) Name() string        { return "github_jobs_resume" }
func (p githubJobsResumePlugin) Description() string { return "Real Github GraphQL Jobs crash fixture" }
func (p githubJobsResumePlugin) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/github_graphql"
}
func (p githubJobsResumePlugin) PrepareTaskData(ctx plugin.TaskContext, _ map[string]interface{}) (interface{}, errors.Error) {
	client, err := collector.CreateAsyncGraphqlClient(ctx, graphql.NewClient(p.endpoint, http.DefaultClient), ctx.GetLogger(), nil)
	if err != nil {
		return nil, err
	}
	return &githubtasks.GithubTaskData{
		Options:       &githubtasks.GithubOptions{ConnectionId: 1, GithubId: 2, Name: "fixture/repo"},
		GraphqlClient: client, RegexEnricher: collector.NewRegexEnricher(),
	}, nil
}
func (p githubJobsResumePlugin) SubTaskMetas() []plugin.SubTaskMeta {
	extract := jobs.ExtractJobsMeta
	extract.EntryPoint = func(ctx plugin.SubTaskContext) errors.Error {
		if p.phase == "crash" && p.scenario == "extract" {
			fmt.Println("CRASH_READY: collection finished, extractor not yet run")
			select {}
		}
		return jobs.ExtractJobs(ctx)
	}
	return []plugin.SubTaskMeta{jobs.CollectJobsMeta, extract}
}

// Runs the actual CollectJobs -> ExtractJobs tasks with a real SQL cursor.
// Only the upstream GitHub endpoint and connection preparation are fixtures.
func TestGithubJobsServerResumeCrash(t *testing.T) {
	dbURL := os.Getenv("DEVLAKE_SERVER_CRASH_DB_URL")
	if dbURL == "" {
		t.Skip("dedicated database not configured")
	}
	phase, scenario := os.Getenv("DEVLAKE_CRASH_PHASE"), os.Getenv("DEVLAKE_JOBS_SCENARIO")
	require.Contains(t, []string{"crash", "crash-again", "resume"}, phase)
	require.Contains(t, []string{"batch", "page", "extract"}, scenario)
	cfg := config.GetConfig()
	cfg.Set("DB_URL", dbURL)
	cfg.Set("RESUME_PIPELINES", true)
	cfg.Set("CONSUME_PIPELINES", true)
	cfg.Set("FORCE_MIGRATION", true)
	cfg.Set("PIPELINE_MAX_PARALLEL", 1)
	cfg.Set("DISABLED_REMOTE_PLUGINS", true)
	cfg.Set("PLUGIN_DIR", t.TempDir())
	cfg.Set("LOGGING_DIR", t.TempDir())
	cfg.Set("AUTH_ENABLED", false)
	cfg.Set("GITHUB_GRAPHQL_JOB_COLLECTION_MODE", "BATCHING")
	cfg.Set("GITHUB_GRAPHQL_JOB_BATCHING_INPUT_STEP", 2)
	cfg.Set("GITHUB_GRAPHQL_JOB_BATCHING_PAGE_SIZE", 3)
	if scenario == "page" {
		cfg.Set("GITHUB_GRAPHQL_JOB_COLLECTION_MODE", "PAGINATING")
		cfg.Set("GITHUB_GRAPHQL_JOB_PAGINATING_PAGE_SIZE", 1)
	}
	cfg.Set(plugin.EncodeKeyEnvStr, "DFLFZLMBBFDDCYWRECDCIYUROPPAKQDFQMMJEFPIKVFVHZBRGAZIHKRJIJZMOHWEVRSCETAGGONPSULGOXITVXISVCQGPSFAOGRDLUANEYDQFBDKVMYYHUZFHYVYGPPT")
	db, err := runner.NewGormDb(cfg, logruslog.Global)
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS jobs_requests (phase VARCHAR(16), run_id INTEGER, cursor_value VARCHAR(64))").Error)
	var original struct {
		ID, PipelineId uint64
		Status         string
	}
	committedJobs := map[int]bool{}
	if phase != "crash" {
		require.NoError(t, db.Table(models.Task{}.TableName()).Take(&original).Error)
		require.Equal(t, models.TASK_RUNNING, original.Status)
		var committed []collector.RawData
		require.NoError(t, db.Table("_raw_github_graphql_jobs").Find(&committed).Error)
		for _, raw := range committed {
			var job jobs.DbCheckRun
			require.NoError(t, json.Unmarshal(raw.Data, &job))
			committedJobs[job.DatabaseId] = true
		}
	}
	nodePattern := regexp.MustCompile(`(?:(\w+)\s*:\s*)?node\s*\(\s*id\s*:\s*\$(\w+)`)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string
			Variables map[string]interface{}
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		matches := nodePattern.FindAllStringSubmatch(req.Query, -1)
		if len(matches) == 0 {
			t.Errorf("no node fields in query: %s / %v", req.Query, req.Variables)
			http.Error(w, "bad fixture query", 400)
			return
		}
		cursor, _ := req.Variables["skipCursor"].(string)
		data := map[string]interface{}{"rateLimit": map[string]int{"cost": 1}}
		for _, match := range matches {
			field := match[1]
			if field == "" {
				field = "node"
			}
			idString, _ := req.Variables[match[2]].(string)
			runID, err := strconv.Atoi(strings.TrimPrefix(idString, "suite-"))
			if err != nil {
				t.Error(err)
				return
			}
			if err := db.Exec("INSERT INTO jobs_requests (phase,run_id,cursor_value) VALUES (?,?,?)", phase, runID, cursor).Error; err != nil {
				t.Error(err)
				return
			}
			if (phase == "crash" && ((scenario == "batch" && runID == 3) || (scenario == "page" && runID == 1 && cursor == "p1"))) || (phase == "crash-again" && runID == 1 && cursor == "p2") {
				fmt.Println("CRASH_READY: real CollectJobs checkpoint committed")
				<-r.Context().Done()
				return
			}
			first, last := 1, 3
			next := false
			endCursor := ""
			if scenario == "page" {
				if cursor != "" {
					first, _ = strconv.Atoi(strings.TrimPrefix(cursor, "p"))
					first++
				}
				last = first
				next = first < 3
				endCursor = fmt.Sprintf("p%d", first)
			}
			nodes := []map[string]interface{}{}
			for n := first; n <= last; n++ {
				nodes = append(nodes, map[string]interface{}{
					"id": fmt.Sprintf("job-%d", runID*10+n), "databaseId": runID*10 + n, "name": fmt.Sprintf("Job %d", n),
					"status": "COMPLETED", "conclusion": "SUCCESS", "startedAt": "2026-01-01T00:00:00Z", "completedAt": "2026-01-01T00:01:00Z",
					"steps": map[string]interface{}{"totalCount": 0, "nodes": []interface{}{}},
				})
			}
			data[field] = map[string]interface{}{"id": idString, "__typename": "CheckSuite",
				"workflowRun": map[string]int{"databaseId": runID},
				"checkRuns":   map[string]interface{}{"totalCount": 3, "nodes": nodes, "pageInfo": map[string]interface{}{"endCursor": endCursor, "hasNextPage": next}},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
	}))
	defer upstream.Close()
	p := githubJobsResumePlugin{upstream.URL, phase, scenario}
	require.NoError(t, plugin.RegisterPlugin(p.Name(), p))
	server.Init()
	services.InitExecuteMigration()
	require.Equal(t, services.SERVICE_STATUS_READY, services.CurrentStatus())
	if phase == "crash-again" {
		select {}
	}
	if phase == "crash" {
		require.NoError(t, db.AutoMigrate(&githubmodels.GithubRun{}, &githubmodels.GithubJob{}))
		sameTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		// Deliberately reverse insertion order with identical updated_at values.
		for _, id := range []int{3, 2, 1} {
			require.NoError(t, db.Create(&githubmodels.GithubRun{ConnectionId: 1, RepoId: 2, ID: id, CheckSuiteNodeID: fmt.Sprintf("suite-%d", id), GithubUpdatedAt: &sameTime}).Error)
		}
		_, err := services.CreatePipeline(&models.NewPipeline{Name: "real GitHub Jobs resume", Plan: models.PipelinePlan{{&models.PipelineTask{Plugin: p.Name(), Options: map[string]interface{}{}}}}}, false)
		require.NoError(t, err)
		select {}
	}
	require.Eventually(t, func() bool {
		var pipeline models.Pipeline
		return db.First(&pipeline, original.PipelineId).Error == nil && pipeline.Status == models.TASK_COMPLETED
	}, 30*time.Second, 100*time.Millisecond)
	var completedPipeline models.Pipeline
	require.NoError(t, db.First(&completedPipeline, original.PipelineId).Error)
	require.Equal(t, 1, completedPipeline.TotalTasks)
	require.Equal(t, 1, completedPipeline.FinishedTasks, "restarts must not inflate task progress")
	var tasks []models.Task
	require.NoError(t, db.Find(&tasks).Error)
	require.Len(t, tasks, 1)
	require.Equal(t, original.ID, tasks[0].ID)
	var subtasks []models.Subtask
	require.NoError(t, db.Where("task_id = ?", original.ID).Find(&subtasks).Error)
	require.Len(t, subtasks, 2, "restart must not duplicate subtask records")
	for _, subtask := range subtasks {
		require.NotNil(t, subtask.FinishedAt)
		require.False(t, subtask.IsFailed)
	}
	var actual []githubmodels.GithubJob
	require.NoError(t, db.Order("id").Find(&actual).Error)
	require.Len(t, actual, 9)
	for i, job := range actual {
		require.Equal(t, (i/3+1)*10+i%3+1, job.ID)
		require.Equal(t, i/3+1, job.RunID)
		require.Equal(t, 2, job.RepoId)
		require.EqualValues(t, 1, job.ConnectionId)
		require.Equal(t, "SUCCESS", job.Conclusion)
	}
	var requests []struct {
		RunID       int
		CursorValue string
	}
	require.NoError(t, db.Table("jobs_requests").Where("phase = ?", phase).Find(&requests).Error)
	switch scenario {
	case "extract":
		require.Empty(t, requests, "completed collector must be skipped")
	case "batch":
		require.Len(t, requests, 1)
		require.Equal(t, 3, requests[0].RunID)
	case "page":
		require.Len(t, requests, 9-len(committedJobs))
		seen := map[int]bool{}
		for _, r := range requests {
			page := 1
			if r.CursorValue != "" {
				previous, err := strconv.Atoi(strings.TrimPrefix(r.CursorValue, "p"))
				require.NoError(t, err)
				page = previous + 1
			}
			id := r.RunID*10 + page
			require.False(t, committedJobs[id], "committed page must not be refetched: %d", id)
			require.False(t, seen[id], "page requested twice: %d", id)
			seen[id] = true
		}
	}
	var rawCount int64
	require.NoError(t, db.Table("_raw_github_graphql_jobs").Count(&rawCount).Error)
	require.EqualValues(t, 9, rawCount)
	var state models.CollectorLatestState
	require.NoError(t, db.Where("raw_data_table = ?", "_raw_github_graphql_jobs").First(&state).Error)
	require.NotNil(t, state.LatestSuccessStart)
	require.WithinDuration(t, *tasks[0].BeganAt, *state.LatestSuccessStart, time.Millisecond)
}
