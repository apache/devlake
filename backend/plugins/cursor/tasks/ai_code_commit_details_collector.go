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
	"net/http"
	"net/url"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

const rawAiCodeCommitDetailsTable = "cursor_ai_code_commit_details"

type aiCodeCommitDetailInput struct {
	CommitHash string
	BranchName string
}

// CollectAiCodeCommitDetails collects file-level blame and conversation metadata from
// GET /analytics/ai-code/commits/:commitHash (Enterprise alpha; skipped when inaccessible).
func CollectAiCodeCommitDetails(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if data.Connection.KeyTier != models.KeyTierEnterprise {
		taskCtx.GetLogger().Info("Skipping AI code commit detail collection: key tier is %q, not enterprise", data.Connection.KeyTier)
		return nil
	}
	if !data.Connection.HasAiCodeCommitDetails {
		taskCtx.GetLogger().Info("Skipping AI code commit detail collection: connection lacks commit-details access")
		return nil
	}

	apiClient, err := CreateApiClient(taskCtx.TaskContext(), data.Connection)
	if err != nil {
		return err
	}

	commitIter, pendingCommits, err := newAiCodeCommitDetailIterator(taskCtx, data)
	if err != nil {
		return err
	}
	if pendingCommits == 0 {
		taskCtx.GetLogger().Info("No AI code commits pending commit-detail collection")
		return nil
	}

	rawArgs := helper.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Table:   rawAiCodeCommitDetailsTable,
		Options: rawParamsFromTaskData(data),
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: rawArgs,
		ApiClient:          apiClient,
		Input:              commitIter,
		PageSize:           1,
		UrlTemplate:        "analytics/ai-code/commits/{{ .Input.CommitHash }}",
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input, ok := reqData.Input.(aiCodeCommitDetailInput)
			if !ok {
				return nil, errors.Default.New("invalid input type for commit-detail collector")
			}
			q := url.Values{}
			if branch := strings.TrimSpace(input.BranchName); branch != "" {
				q.Set("branch", branch)
			}
			return q, nil
		},
		GetTotalPages: func(_ *http.Response, _ *helper.ApiCollectorArgs) (int, errors.Error) {
			return 1, nil
		},
		ResponseParser: parseCommitDetailsWholeBodyResponse,
		AfterResponse:  ignoreHTTPStatus404,
	})
	if err != nil {
		return err
	}

	taskCtx.GetLogger().Info("Collecting AI code commit details for %d commit(s)", pendingCommits)
	return collector.Execute()
}

func newAiCodeCommitDetailIterator(taskCtx plugin.SubTaskContext, data *CursorTaskData) (*helper.QueueIterator, int, errors.Error) {
	db := taskCtx.GetDal()
	commitTable := models.CursorAiCodeCommit{}.TableName()
	detailTable := models.CursorAiCodeRangeAnnotation{}.TableName()

	clauses := []dal.Clause{
		dal.Select("c.commit_hash", "c.branch_name"),
		dal.From(commitTable + " c"),
		dal.Where("c.connection_id = ? AND c.scope_id = ?", data.Options.ConnectionId, data.Options.ScopeId),
		dal.Orderby("c.commit_hash ASC"),
	}
	if db.HasTable(detailTable) {
		clauses = append(clauses, dal.Where(`NOT EXISTS (
			SELECT 1 FROM `+detailTable+` r
			WHERE r.connection_id = c.connection_id
			  AND r.scope_id = c.scope_id
			  AND r.commit_hash = c.commit_hash
		)`))
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close()

	iter := helper.NewQueueIterator()
	pendingCommits := 0
	for cursor.Next() {
		var row struct {
			CommitHash string
			BranchName string
		}
		if fetchErr := db.Fetch(cursor, &row); fetchErr != nil {
			return nil, 0, errors.Default.Wrap(fetchErr, "failed to read commit for detail collection")
		}
		hash := strings.TrimSpace(row.CommitHash)
		if hash == "" {
			continue
		}
		iter.Push(aiCodeCommitDetailInput{
			CommitHash: hash,
			BranchName: strings.TrimSpace(row.BranchName),
		})
		pendingCommits++
	}

	return iter, pendingCommits, nil
}

func parseCommitDetailsWholeBodyResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	body, err := readResponseBody(res)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, nil
	}
	return []json.RawMessage{body}, nil
}
