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
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

type ComparisonInput struct {
	CommitSha string `json:"commit_sha"`
	ParentSha string `json:"parent_sha"`
	FlagName  string `json:"flag_name"`
}

var CollectComparisonMeta = plugin.SubTaskMeta{
	Name:             "CollectComparison",
	EntryPoint:       CollectComparison,
	EnabledByDefault: true,
	Description:      "Collect comparison data (modified/patch coverage) per flag from Codecov API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertFlagsMeta},
	DependencyTables: []string{models.CodecovCommit{}.TableName(), models.CodecovFlag{}.TableName()},
}

func CollectComparison(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()

	// Extract owner and repo from FullName
	owner, repo, err := ParseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	// Use sync policy time range, default to last 90 days
	var startDate time.Time
	syncPolicy := taskCtx.TaskContext().SyncPolicy()
	if syncPolicy != nil && syncPolicy.TimeAfter != nil {
		startDate = *syncPolicy.TimeAfter
		logger.Info("[Codecov] Comparison: Using sync policy from %s", startDate.Format("2006-01-02"))
	} else {
		startDate = time.Now().AddDate(0, 0, -90)
		logger.Info("[Codecov] Comparison: Using default 90 days from %s", startDate.Format("2006-01-02"))
	}

	// Get commits ordered newest-first so the most recent (most valuable) data is collected first;
	// if the task is killed or times out, older data is what gets skipped rather than recent data
	var commits []models.CodecovCommit
	err = db.All(&commits, dal.Where("connection_id = ? AND repo_id = ? AND commit_timestamp >= ?", data.Options.ConnectionId, data.Options.FullName, startDate), dal.Orderby("commit_timestamp DESC"))
	if err != nil {
		return err
	}

	// Get all flags
	var flags []models.CodecovFlag
	err = db.All(&flags, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return err
	}

	// Build skip-list from the raw API table (_raw_codecov_api_comparisons) instead of
	// the tool table (_tool_codecov_comparisons). The raw table is populated by the collector
	// before the converter runs, so it reflects all previously fetched API responses.
	paramsJSON, jsonErr := json.Marshal(CodecovApiParams{
		ConnectionId: data.Options.ConnectionId,
		Name:         data.Options.FullName,
	})
	if jsonErr != nil {
		return errors.Default.Wrap(jsonErr, "failed to marshal params for skip-list query")
	}

	collectedSet := make(map[string]bool)
	rawTable := "_raw_codecov_api_comparisons"
	if db.HasTable(rawTable) {
		var rawInputs []struct {
			Input json.RawMessage `json:"input"`
		}
		err = db.All(&rawInputs,
			dal.From(rawTable),
			dal.Where("params = ?", string(paramsJSON)),
			dal.Select("input"),
		)
		if err != nil {
			return err
		}
		for _, row := range rawInputs {
			var input ComparisonInput
			if unmarshalErr := json.Unmarshal(row.Input, &input); unmarshalErr == nil {
				key := fmt.Sprintf("%s|%s|%s", input.CommitSha, input.ParentSha, input.FlagName)
				collectedSet[key] = true
			}
		}
	}
	logger.Info("[Codecov] Comparison: Loaded %d existing raw API responses into skip-list", len(collectedSet))

	// Build comparison pairs for each commit × flag combination
	// Use the actual ParentSha from the commit (from Codecov API) instead of assuming sequential order
	// Patch coverage IS flag-specific - each flag shows how well that test type covers new code
	// Only collect NEW comparisons that don't exist in the database
	iterator := helper.NewQueueIterator()
	skippedCount := 0
	addedCount := 0
	for _, commit := range commits {
		// Skip commits without a parent (first commit in repo)
		if commit.ParentSha == "" {
			continue
		}

		// Collect comparison for each flag (flag-specific patch coverage)
		for _, flag := range flags {
			key := fmt.Sprintf("%s|%s|%s", commit.CommitSha, commit.ParentSha, flag.FlagName)
			if !collectedSet[key] {
				iterator.Push(&ComparisonInput{
					CommitSha: commit.CommitSha,
					ParentSha: commit.ParentSha, // Use actual parent from Codecov API
					FlagName:  flag.FlagName,    // Flag-specific patch coverage
				})
				addedCount++
			} else {
				skippedCount++
			}
		}
	}

	logger.Info("[Codecov] Comparison: Skipped %d already collected, collecting %d new", skippedCount, addedCount)

	// If nothing new to collect, return early
	if addedCount == 0 {
		logger.Info("[Codecov] Comparison: All data already collected, skipping API calls")
		return nil
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMPARISONS_TABLE,
		},
		Incremental: true, // ALWAYS preserve historical data
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: fmt.Sprintf("%s/compare", RepoAPIPrefix(data.Service, owner, repo)),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*ComparisonInput)
			query := url.Values{}
			query.Set("base", input.ParentSha)
			query.Set("head", input.CommitSha)
			// Send flag parameter for flag-specific patch coverage
			if input.FlagName != "" {
				query.Set("flag", input.FlagName)
			}
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			// Safety check: if status is 404 or 500+, return empty array to skip
			if res.StatusCode == http.StatusNotFound || res.StatusCode >= http.StatusInternalServerError {
				return []json.RawMessage{}, nil
			}
			var body json.RawMessage
			err := helper.UnmarshalResponse(res, &body)
			if err != nil {
				return nil, err
			}
			return []json.RawMessage{body}, nil
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
