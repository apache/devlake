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
	"fmt"
	"net/url"
	"reflect"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

const RAW_ISSUE_COMMENTS_TABLE = "youtrack_issue_comments"

// commentFields is the comment collector's fields param. The
// `deleted` flag is the whole reason comments come from this endpoint
// rather than the activities feed.
const commentFields = "id,text,created,updated,deleted," +
	"author(id,login,fullName,email,avatarUrl,guest)"

// youtrackCommentInput is the comment collector's input row: one per
// changed issue, driving one per-issue comments call. Stored on each raw
// row's input column; the extractor reads the issue id back from it.
type youtrackCommentInput struct {
	Id string `json:"id"`
}

var CollectCommentsMeta = plugin.SubTaskMeta{
	Name:             "Collect Comments",
	EntryPoint:       CollectComments,
	EnabledByDefault: true,
	Description:      "Collect comments for the changed YouTrack issues — one per-issue call, bounded by the incremental window",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.YoutrackIssue{}.TableName()},
	ProductTables:    []string{RAW_ISSUE_COMMENTS_TABLE},
}

var _ plugin.SubTaskEntryPoint = CollectComments

func CollectComments(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*YoutrackTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	apiCollector, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Options: models.YoutrackApiParams{
			ConnectionId: data.Options.ConnectionId,
			ProjectId:    data.Options.ProjectId,
		},
		Table: RAW_ISSUE_COMMENTS_TABLE,
	})
	if err != nil {
		return err
	}

	// input = the scope's tool-layer issues, bounded to the changed-issue
	// window of changedIssuesSince — the same contract the
	// changelog collector applies. The issue collector ran earlier in the
	// pipeline, so this sees the run's fresh issue rows.
	clauses := []dal.Clause{
		dal.Select("id"),
		dal.From(&models.YoutrackIssue{}),
		dal.Where("connection_id = ? AND project_id = ?", data.Options.ConnectionId, data.Options.ProjectId),
	}
	if since := changedIssuesSince(apiCollector.IsIncremental(), apiCollector.GetSince()); since != nil {
		clauses = append(clauses, dal.Where("updated >= ?", *since))
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	input, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(youtrackCommentInput{}))
	if err != nil {
		return err
	}
	logger.Info("collecting comments per changed issue (incremental: %v)", apiCollector.IsIncremental())

	err = apiCollector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		UrlTemplate: "issues/{{ .Input.Id }}/comments",
		Input:       input,
		PageSize:    pageSizeFor(data.Options),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			return url.Values{
				"fields": []string{commentFields},
				"$skip":  []string{fmt.Sprintf("%d", reqData.Pager.Skip)},
				"$top":   []string{fmt.Sprintf("%d", reqData.Pager.Size)},
			}, nil
		},
		// no total count in the response: undetermined pagination walks
		// $skip/$top until a short page (same shape as the issues endpoint)
		ResponseParser: parseJsonArrayResponse,
		AfterResponse:  ignoreHTTPStatus404,
	})
	if err != nil {
		return err
	}
	return apiCollector.Execute()
}
