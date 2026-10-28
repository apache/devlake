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

type CommitFlagInput struct {
	CommitSha string `json:"commit_sha"`
	FlagName  string `json:"flag_name"`
}

var CollectCommitCoverageMeta = plugin.SubTaskMeta{
	Name:             "CollectCommitCoverage",
	EntryPoint:       CollectCommitCoverage,
	EnabledByDefault: true,
	Description:      "Collect coverage per flag per commit from Codecov API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertFlagsMeta},
	DependencyTables: []string{models.CodecovCommit{}.TableName(), models.CodecovFlag{}.TableName()},
}

func CollectCommitCoverage(taskCtx plugin.SubTaskContext) errors.Error {
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
		logger.Info("[Codecov] CommitCoverage: Using sync policy from %s", startDate.Format("2006-01-02"))
	} else {
		startDate = time.Now().AddDate(0, 0, -90)
		logger.Info("[Codecov] CommitCoverage: Using default 90 days from %s", startDate.Format("2006-01-02"))
	}

	// Get distinct commits filtered by sync policy.
	// The commits table may lack a PK (pre-migration), causing duplicate rows.
	// Only commit_sha is needed downstream for the API iterator.
	var commits []models.CodecovCommit
	err = db.All(&commits,
		dal.Select("DISTINCT connection_id, repo_id, commit_sha"),
		dal.Where("connection_id = ? AND repo_id = ? AND commit_timestamp >= ?", data.Options.ConnectionId, data.Options.FullName, startDate),
	)
	if err != nil {
		return err
	}

	// Get all flags
	var flags []models.CodecovFlag
	err = db.All(&flags, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return err
	}

	// Get existing coverage records to skip already collected data (OPTIMIZATION)
	var existingCoverages []models.CodecovCoverage
	err = db.All(&existingCoverages, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return err
	}

	// Build a set of already collected commit+flag combinations.
	// Zero-data rows younger than 7 days are NOT marked as collected — they get
	// re-fetched in case the coverage upload arrived after the previous collection.
	retryWindow := time.Now().Add(-7 * 24 * time.Hour)
	collectedSet := make(map[string]bool)
	for _, cov := range existingCoverages {
		key := fmt.Sprintf("%s|%s", cov.CommitSha, cov.FlagName)
		if cov.LinesTotal > 0 {
			collectedSet[key] = true
		} else if cov.CommitTimestamp != nil && cov.CommitTimestamp.Before(retryWindow) {
			collectedSet[key] = true
		}
	}

	// Build iterator with only NEW commit × flag combinations
	iterator := helper.NewQueueIterator()
	skippedCount := 0
	addedCount := 0
	for _, commit := range commits {
		for _, flag := range flags {
			// Skip empty flag names - only collect per-flag coverage
			if flag.FlagName == "" {
				continue
			}
			key := fmt.Sprintf("%s|%s", commit.CommitSha, flag.FlagName)
			if !collectedSet[key] {
				iterator.Push(&CommitFlagInput{
					CommitSha: commit.CommitSha,
					FlagName:  flag.FlagName,
				})
				addedCount++
			} else {
				skippedCount++
			}
		}
	}

	logger.Info("[Codecov] CommitCoverage: Skipped %d already collected, collecting %d new", skippedCount, addedCount)

	// If nothing new to collect, return early
	if addedCount == 0 {
		logger.Info("[Codecov] CommitCoverage: All data already collected, skipping API calls")
		return nil
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMMIT_COVERAGES_TABLE,
		},
		Incremental: true, // ALWAYS preserve historical data
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: fmt.Sprintf("%s/totals/", RepoAPIPrefix(data.Service, owner, repo)),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*CommitFlagInput)
			query := url.Values{}
			query.Set("sha", input.CommitSha)
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
			// The response contains coverage data for the specific flag (or overall if no flag)
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
