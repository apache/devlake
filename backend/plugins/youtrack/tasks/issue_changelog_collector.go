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
	"fmt"
	"net/http"
	"net/url"
	"reflect"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

const RAW_ISSUE_CHANGELOGS_TABLE = "youtrack_issue_changelogs"

// changelogCategories is the activitiesPage categories filter.
// The param is mandatory — the API 400s without it. Excluded:
// CommentsCategory (comments come from the comments endpoint, with the
// deleted flag), LinksCategory (out of scope), DescriptionCategory (text
// churn, no metric value).
const changelogCategories = "CustomFieldCategory,IssueCreatedCategory," +
	"IssueResolvedCategory,SummaryCategory,TagsCategory"

// changelogFields is the changelog collector's fields param: the item shape
// lives under activities(); the page-level cursors drive the walk. The
// added/removed user payloads carry the full account fields (verified — the
// API honours them on the polymorphic elements), so changelog-derived
// accounts are as rich as issue/comment-derived ones.
const changelogFields = "activities(" +
	"id,timestamp,category(id)," +
	"author(id,login,fullName,email,avatarUrl,guest)," +
	"target(id,idReadable,$type),targetMember," +
	"field(name,customField(name,fieldType(id)))," +
	"added(id,name,login,fullName,email,avatarUrl,guest,isResolved,$type)," +
	"removed(id,name,login,fullName,email,avatarUrl,guest,isResolved,$type)" +
	"),afterCursor,beforeCursor,hasAfter,hasBefore,reverse"

// youtrackChangelogInput is the changelog collector's input row: one per
// changed issue, driving one per-issue activitiesPage walk. Stored on each
// raw row's input column; the extractor reads the issue id back from it.
type youtrackChangelogInput struct {
	Id string `json:"id"`
}

// youtrackActivityPage is the ActivityCursorPage envelope: one page of an
// issue's activity feed. Cursors are opaque and URL-unsafe (they contain
// +, ^ and :) — they reach the wire percent-encoded because the api client
// builds the query string via url.Values.Encode.
type youtrackActivityPage struct {
	Activities  []json.RawMessage `json:"activities"`
	AfterCursor string            `json:"afterCursor"`
	HasAfter    bool              `json:"hasAfter"`
}

var CollectIssueChangelogsMeta = plugin.SubTaskMeta{
	Name:             "Collect Issue Changelogs",
	EntryPoint:       CollectIssueChangelogs,
	EnabledByDefault: true,
	Description:      "Collect the activity feed (issue changelogs) for the changed YouTrack issues — one per-issue activitiesPage cursor walk, bounded by the incremental window",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.YoutrackIssue{}.TableName()},
	ProductTables:    []string{RAW_ISSUE_CHANGELOGS_TABLE},
}

var _ plugin.SubTaskEntryPoint = CollectIssueChangelogs

func CollectIssueChangelogs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*YoutrackTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	apiCollector, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Options: models.YoutrackApiParams{
			ConnectionId: data.Options.ConnectionId,
			ProjectId:    data.Options.ProjectId,
		},
		Table: RAW_ISSUE_CHANGELOGS_TABLE,
	})
	if err != nil {
		return err
	}

	// input = the scope's tool-layer issues, bounded to the ones changed
	// since the bookmark on incremental runs — the same changed-issues
	// input as the comment collector. Re-walks are idempotent
	// because extraction upserts by activity id.
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
	input, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(youtrackChangelogInput{}))
	if err != nil {
		return err
	}
	logger.Info("collecting issue changelogs per changed issue (incremental: %v)", apiCollector.IsIncremental())

	pageSize := pageSizeFor(data.Options)
	err = apiCollector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		UrlTemplate: "issues/{{ .Input.Id }}/activitiesPage",
		Input:       input,
		PageSize:    pageSize,
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			return changelogQuery(reqData, pageSize)
		},
		// cursor walk (fetchPagesSequentially): follow the opaque
		// afterCursor until hasAfter=false — verified against the live API
		// to terminate cleanly with zero overlap between pages. Note the
		// framework also stops a walk on a short page (count < PageSize)
		// without consulting GetNextPageCustomData; verified pages come
		// back full ($top) until the last, so the two rules agree here.
		GetNextPageCustomData: nextActivityCursor,
		ResponseParser:        parseActivityPageResponse,
	})
	if err != nil {
		return err
	}
	return apiCollector.Execute()
}

// changelogQuery builds the query for one activitiesPage request (cursor
// discipline): the mandatory categories filter, fields and $top
// repeat on every request because the cursor is not self-describing; the
// cursor itself joins once the walk is under way.
func changelogQuery(reqData *helper.RequestData, pageSize int) (url.Values, errors.Error) {
	query := url.Values{
		"fields":     []string{changelogFields},
		"categories": []string{changelogCategories},
		"$top":       []string{fmt.Sprintf("%d", pageSize)},
	}
	if cursor, ok := reqData.CustomData.(string); ok && cursor != "" {
		query.Set("cursor", cursor)
	}
	return query, nil
}

// nextActivityCursor drives the walk: hasAfter=false finishes the issue's
// history; otherwise the opaque afterCursor goes to the next request
// through RequestData.CustomData.
func nextActivityCursor(_ *helper.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
	page, err := parseActivityPage(prevPageResponse)
	if err != nil {
		return nil, err
	}
	if !page.HasAfter {
		return nil, helper.ErrFinishCollect
	}
	return page.AfterCursor, nil
}

// parseActivityPageResponse is the ResponseParser: one raw row per activity
// item on the page (the raw-table grain, like one row per comment).
func parseActivityPageResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	page, err := parseActivityPage(res)
	if err != nil {
		return nil, err
	}
	return page.Activities, nil
}

// parseActivityPage decodes one ActivityCursorPage body.
func parseActivityPage(res *http.Response) (*youtrackActivityPage, errors.Error) {
	page := &youtrackActivityPage{}
	if err := helper.UnmarshalResponse(res, page); err != nil {
		return nil, err
	}
	return page, nil
}
