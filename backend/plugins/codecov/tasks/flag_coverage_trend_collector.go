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
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

type FlagInput struct {
	FlagName string `json:"flag_name"`
}

var CollectFlagCoverageTrendMeta = plugin.SubTaskMeta{
	Name:             "CollectFlagCoverageTrend",
	EntryPoint:       CollectFlagCoverageTrend,
	EnabledByDefault: true,
	Description:      "Collect historical coverage trend per flag from Codecov API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertFlagsMeta},
	DependencyTables: []string{models.CodecovFlag{}.TableName()},
}

func CollectFlagCoverageTrend(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()

	// Log sync policy time range
	syncPolicy := taskCtx.TaskContext().SyncPolicy()
	if syncPolicy != nil && syncPolicy.TimeAfter != nil {
		logger.Info("[Codecov] CollectFlagCoverageTrend: Using sync policy time range from %s", syncPolicy.TimeAfter.Format("2006-01-02"))
	} else {
		logger.Info("[Codecov] CollectFlagCoverageTrend: No sync policy, using default 90 days")
	}

	// Extract owner and repo from FullName
	owner, repo, err := ParseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	// Create iterator from collected flags, skipping empty names and names with
	// control characters (newlines, tabs, etc.) that would produce invalid URLs.
	// Some repos have misconfigured flag names like "ARM64-3.10\n3.11".
	clauses := []dal.Clause{
		dal.Select("flag_name AS flag_name"),
		dal.From(&models.CodecovFlag{}),
		dal.Where("connection_id = ? AND repo_id = ? AND flag_name != '' AND flag_name NOT REGEXP '[[:cntrl:]]'",
			data.Options.ConnectionId, data.Options.FullName),
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	iterator, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(FlagInput{}))
	if err != nil {
		return err
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_FLAG_COVERAGE_TRENDS_TABLE,
		},
		Incremental: true, // ALWAYS preserve historical data
		ApiClient:   data.ApiClient,
		Input:       iterator,
		PageSize:    100, // Max results per page
		// Use the correct per-flag coverage endpoint: /flags/{flag_name}/coverage (NO trailing slash!)
		// See: https://docs.codecov.com/reference/repos_flags_coverage_list
		UrlTemplate: fmt.Sprintf("%s/flags/{{ .Input.FlagName }}/coverage", RepoAPIPrefix(data.Service, owner, repo)),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("interval", "1d") // Daily trend data
			query.Set("page", fmt.Sprintf("%d", reqData.Pager.Page))

			// Use sync policy time range, default to last 90 days
			endDate := time.Now()
			var startDate time.Time
			syncPolicy := taskCtx.TaskContext().SyncPolicy()
			if syncPolicy != nil && syncPolicy.TimeAfter != nil {
				startDate = *syncPolicy.TimeAfter
			} else {
				// Default to last 90 days if no sync policy
				startDate = endDate.AddDate(0, 0, -90)
			}
			query.Set("start_date", startDate.Format("2006-01-02"))
			query.Set("end_date", endDate.Format("2006-01-02"))
			return query, nil
		},
		GetTotalPages: func(res *http.Response, args *helper.ApiCollectorArgs) (int, errors.Error) {
			var response struct {
				TotalPages int `json:"total_pages"`
			}
			err := helper.UnmarshalResponse(res, &response)
			if err != nil {
				return 0, err
			}
			logger.Info("[Codecov] Coverage trend has %d pages to collect", response.TotalPages)
			return response.TotalPages, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			// Safety check: if status is 404 or 500+, return empty array to skip
			if res.StatusCode == http.StatusNotFound || res.StatusCode >= http.StatusInternalServerError {
				return []json.RawMessage{}, nil
			}
			var response struct {
				Results []json.RawMessage `json:"results"`
			}
			err := helper.UnmarshalResponse(res, &response)
			if err != nil {
				return nil, err
			}
			return response.Results, nil
		},
		AfterResponse: func(res *http.Response) errors.Error {
			if res.StatusCode == http.StatusUnauthorized {
				return errors.Unauthorized.New("authentication failed, please check your AccessToken")
			}
			// Skip 404 (no coverage) and 500 (server error) without retrying
			if res.StatusCode == http.StatusNotFound || res.StatusCode >= http.StatusInternalServerError {
				return helper.ErrIgnoreAndContinue
			}
			return nil
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}
