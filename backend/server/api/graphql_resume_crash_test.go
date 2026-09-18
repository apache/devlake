/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements. See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License. You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/apache/devlake/core/config"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/runner"
	collector "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/impls/logruslog"
	server "github.com/apache/devlake/server/api"
	"github.com/apache/devlake/server/services"
	"github.com/merico-ai/graphql"
	"github.com/stretchr/testify/require"
)

type resumeFixturePlugin struct{ endpoint string }

type serverCrashRequest struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Phase       string
	CursorValue string
}

func (serverCrashRequest) TableName() string { return "server_crash_requests" }

func (p resumeFixturePlugin) Name() string        { return "graphql_resume_fixture" }
func (p resumeFixturePlugin) Description() string { return "Crash/restart fixture" }
func (p resumeFixturePlugin) RootPkgPath() string { return "github.com/apache/devlake/server/api" }
func (p resumeFixturePlugin) PrepareTaskData(ctx plugin.TaskContext, _ map[string]interface{}) (interface{}, errors.Error) {
	return collector.CreateAsyncGraphqlClient(ctx, graphql.NewClient(p.endpoint, http.DefaultClient), ctx.GetLogger(), nil)
}

type resumeFixtureQuery struct {
	Items struct {
		Nodes    []struct{ ID int }
		PageInfo collector.GraphqlQueryPageInfo
	} `graphql:"items(after: $after)"`
}

func (p resumeFixturePlugin) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{{Name: "Collect fixture", EnabledByDefault: true, EntryPoint: func(ctx plugin.SubTaskContext) errors.Error {
		c, err := collector.NewGraphqlCollector(collector.GraphqlCollectorArgs{
			RawDataSubTaskArgs: collector.RawDataSubTaskArgs{Ctx: ctx, Table: "server_resume_fixture", Params: "repo"},
			GraphqlClient:      ctx.GetData().(*collector.GraphqlAsyncClient), PageSize: 1,
			BuildQuery: func(req *collector.GraphqlRequestData) (interface{}, map[string]interface{}, error) {
				return &resumeFixtureQuery{}, map[string]interface{}{"after": (*graphql.String)(req.Pager.SkipCursor)}, nil
			},
			GetPageInfo: func(q interface{}, _ *collector.GraphqlCollectorArgs) (*collector.GraphqlQueryPageInfo, error) {
				return &q.(*resumeFixtureQuery).Items.PageInfo, nil
			},
			ResponseParser: func(q interface{}) ([]json.RawMessage, errors.Error) {
				var rows []json.RawMessage
				for _, node := range q.(*resumeFixtureQuery).Items.Nodes {
					row, err := json.Marshal(node)
					if err != nil {
						return nil, errors.Convert(err)
					}
					rows = append(rows, row)
				}
				return rows, nil
			},
		})
		if err != nil {
			return err
		}
		return c.Execute()
	}}}
}

// Opt-in: start a real API server and its production queue/runner. Kill the
// crash-phase container after CRASH_READY, then run resume against the SAME
// disposable database. No task IDs or statuses are injected by this test.
func TestGraphqlServerResumeCrash(t *testing.T) {
	dbURL := os.Getenv("DEVLAKE_SERVER_CRASH_DB_URL")
	if dbURL == "" {
		t.Skip("dedicated server crash database not configured")
	}
	phase := os.Getenv("DEVLAKE_CRASH_PHASE")
	rerun := os.Getenv("DEVLAKE_SERVER_RERUN") == "true"
	require.Contains(t, []string{"crash", "resume"}, phase)
	cfg := config.GetConfig()
	cfg.Set("DB_URL", dbURL)
	cfg.Set("RESUME_PIPELINES", !rerun)
	cfg.Set("CONSUME_PIPELINES", true)
	cfg.Set("FORCE_MIGRATION", true)
	cfg.Set("PIPELINE_MAX_PARALLEL", 1)
	cfg.Set("DISABLED_REMOTE_PLUGINS", true)
	cfg.Set("PLUGIN_DIR", t.TempDir())
	cfg.Set("LOGGING_DIR", t.TempDir())
	cfg.Set("AUTH_ENABLED", false)
	cfg.Set(plugin.EncodeKeyEnvStr, "DFLFZLMBBFDDCYWRECDCIYUROPPAKQDFQMMJEFPIKVFVHZBRGAZIHKRJIJZMOHWEVRSCETAGGONPSULGOXITVXISVCQGPSFAOGRDLUANEYDQFBDKVMYYHUZFHYVYGPPT")
	db, err := runner.NewGormDb(cfg, logruslog.Global)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&serverCrashRequest{}))
	var original struct {
		ID         uint64
		PipelineId uint64
		Status     string
	}
	if phase == "resume" {
		require.NoError(t, db.Table(models.Task{}.TableName()).Take(&original).Error)
		require.Equal(t, models.TASK_RUNNING, original.Status)
		var pipeline struct{ Status string }
		require.NoError(t, db.Table(models.Pipeline{}.TableName()).Where("id = ?", original.PipelineId).Take(&pipeline).Error)
		require.Equal(t, models.TASK_RUNNING, pipeline.Status)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Variables struct{ After *string } }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		cursor := ""
		if req.Variables.After != nil {
			cursor = *req.Variables.After
		}
		if err := db.Exec("INSERT INTO server_crash_requests (phase, cursor_value) VALUES (?, ?)", phase, cursor).Error; err != nil {
			t.Error(err)
			return
		}
		if phase == "crash" && cursor == "page-1" {
			fmt.Println("CRASH_READY: real server pipeline running, first page committed")
			<-r.Context().Done()
			return
		}
		id, next := 1, true
		if cursor != "" {
			id, next = 2, false
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{"items": map[string]interface{}{
			"nodes": []map[string]int{{"id": id}}, "pageInfo": collector.GraphqlQueryPageInfo{EndCursor: "page-1", HasNextPage: next},
		}}})
	}))
	defer upstream.Close()
	p := resumeFixturePlugin{endpoint: upstream.URL}
	require.NoError(t, plugin.RegisterPlugin(p.Name(), p))
	server.Init()
	services.InitExecuteMigration()
	require.Equal(t, services.SERVICE_STATUS_READY, services.CurrentStatus())
	router := server.CreateApiServer()
	server.SetupApiServer(router)
	apiServer := httptest.NewServer(router)
	defer apiServer.Close()
	if phase == "crash" {
		payload := models.NewPipeline{Name: "graphql crash resume", Plan: models.PipelinePlan{{&models.PipelineTask{Plugin: p.Name(), Options: map[string]interface{}{}}}}}
		body, err := json.Marshal(payload)
		require.NoError(t, err)
		response, err := http.Post(apiServer.URL+"/pipelines", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer response.Body.Close()
		require.Less(t, response.StatusCode, 300)
		// The orchestrator must SIGKILL this process before the timeout.
		select {}
	}
	if rerun {
		var interrupted models.Task
		require.NoError(t, db.First(&interrupted, original.ID).Error)
		require.Equal(t, models.TASK_FAILED, interrupted.Status)
		var count int64
		require.NoError(t, db.Table("server_crash_requests").Where("phase = ?", phase).Count(&count).Error)
		require.Zero(t, count, "RESUME_PIPELINES=false must not restart collection")
		response, err := http.Post(fmt.Sprintf("%s/pipelines/%d/rerun", apiServer.URL, original.PipelineId), "application/json", nil)
		require.NoError(t, err)
		require.Less(t, response.StatusCode, 300)
		response.Body.Close()
	}
	require.Eventually(t, func() bool {
		var pipeline models.Pipeline
		return db.First(&pipeline, original.PipelineId).Error == nil && pipeline.Status == models.TASK_COMPLETED
	}, 30*time.Second, 100*time.Millisecond)
	var tasks []models.Task
	require.NoError(t, db.Order("id").Find(&tasks).Error)
	completedID := original.ID
	if rerun {
		require.Len(t, tasks, 2)
		require.Equal(t, models.TASK_FAILED, tasks[0].Status)
		require.Greater(t, tasks[1].ID, original.ID)
		completedID = tasks[1].ID
	} else {
		require.Len(t, tasks, 1)
		require.Equal(t, original.ID, tasks[0].ID)
	}
	require.Equal(t, models.TASK_COMPLETED, tasks[len(tasks)-1].Status)
	var subtask models.Subtask
	require.NoError(t, db.Where("task_id = ?", completedID).First(&subtask).Error)
	require.NotNil(t, subtask.FinishedAt)
	var requests []string
	require.NoError(t, db.Table("server_crash_requests").Where("phase = ?", phase).Order("id").Pluck("cursor_value", &requests).Error)
	if rerun {
		require.Equal(t, []string{"", "page-1"}, requests)
	} else {
		require.Equal(t, []string{"page-1"}, requests)
	}
	var rows []collector.RawData
	require.NoError(t, db.Table("_raw_server_resume_fixture").Order("id").Find(&rows).Error)
	require.Len(t, rows, 2)
	for i, row := range rows {
		var node struct{ ID int }
		require.NoError(t, json.Unmarshal(row.Data, &node))
		require.Equal(t, i+1, node.ID)
	}
	var state collector.GraphqlCollectorState
	require.NoError(t, db.Where("input_hash = ? AND raw_data_table = ?", "", "_raw_server_resume_fixture").First(&state).Error)
	require.Equal(t, completedID, state.TaskID)
	require.True(t, state.Completed)
}
