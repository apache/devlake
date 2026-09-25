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
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

const RAW_ISSUES_TABLE = "youtrack_issues"

// issueFields is the issue collector's fields param.
const issueFields = "id,idReadable,numberInProject,created,updated,resolved,commentsCount," +
	"summary,description,project(id,shortName,name)," +
	"reporter(id,login,fullName,email,avatarUrl,guest)," +
	"updater(id,login,fullName,email,avatarUrl,guest)," +
	"tags(id,name)," +
	"customFields(id,name,$type,value(id,name,login,fullName,email,avatarUrl,guest,isResolved,$type))"

const (
	// defaultIssuePageSize is conservative by design: narrow pages
	// keep fields-heavy responses small.
	defaultIssuePageSize = 100
	// maxIssuePageSize is the server's undocumented hard cap on $top
	// ($top=5000 -> 400 "greater than limit 3500").
	maxIssuePageSize = 3500
)

// pageSizeFor resolves the effective issue page size: the configured value,
// clamped to [1, 3500], defaulting to 100.
func pageSizeFor(options *YoutrackOptions) int {
	if options.PageSize <= 0 {
		return defaultIssuePageSize
	}
	if options.PageSize > maxIssuePageSize {
		return maxIssuePageSize
	}
	return options.PageSize
}

// incrementalOverlap widens the lower bound of the incremental window:
// YouTrack resolves query dates in the caller's profile timezone
// while returning epoch-ms UTC, so any fixed window can silently skip rows
// at the edge. 26h absorbs every profile offset in -12..+14; re-collection
// within the overlap is harmless because extraction upserts by issue id.
const incrementalOverlap = 26 * time.Hour

var CollectIssuesMeta = plugin.SubTaskMeta{
	Name:             "Collect Issues",
	EntryPoint:       CollectIssues,
	EnabledByDefault: true,
	Description:      "Collect issues for a YouTrack project, supports incremental collection",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = CollectIssues

func CollectIssues(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*YoutrackTaskData)
	logger := taskCtx.GetLogger()
	apiCollector, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Options: models.YoutrackApiParams{
			ConnectionId: data.Options.ConnectionId,
			ProjectId:    data.Options.ProjectId,
		},
		Table: RAW_ISSUES_TABLE,
	})
	if err != nil {
		return err
	}

	query := buildIssueQuery(data.Project.ShortName, apiCollector.GetSince())
	logger.Info("collecting issues with query: %s", query)

	err = apiCollector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		UrlTemplate: "issues",
		PageSize:    pageSizeFor(data.Options),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			return url.Values{
				"query":  []string{query},
				"fields": []string{issueFields},
				"$skip":  []string{fmt.Sprintf("%d", reqData.Pager.Skip)},
				"$top":   []string{fmt.Sprintf("%d", reqData.Pager.Size)},
			}, nil
		},
		// no total count in the response: undetermined pagination walks
		// $skip/$top until a short page. Pages are fetched concurrently, so
		// the sort key must be immutable: sorting by `updated` lets an edit
		// made mid-run shift every later issue back one slot and drop the one
		// that crosses an already-fetched page boundary (Jira orders by
		// created ASC for the same reason).
		ResponseParser: parseJsonArrayResponse,
	})
	if err != nil {
		return err
	}
	return apiCollector.Execute()
}

// buildIssueQuery renders the YouTrack query for one collection window:
// `project: {KEY} updated: {since-26h} .. * sort by: created asc`.
// The lower bound is formatted in UTC; the profile-timezone hazard is
// absorbed by the overlap, never by fetching the profile. There is
// deliberately no upper bound: YouTrack reads query dates in the token
// owner's profile timezone, so a UTC "now" at a UTC+N profile would exclude
// the most recent N hours on every run, and the extractor already clips by
// its own `until`. A nil since (first full sync with no timeAfter) drops the
// updated clause entirely. The sort key is immutable (see InitCollector).
func buildIssueQuery(shortName string, since *time.Time) string {
	query := fmt.Sprintf("project: {%s}", shortName)
	if since != nil {
		from := since.Add(-incrementalOverlap).UTC().Format("2006-01-02T15:04")
		query += fmt.Sprintf(" updated: {%s} .. *", from)
	}
	return query + " sort by: created asc"
}
