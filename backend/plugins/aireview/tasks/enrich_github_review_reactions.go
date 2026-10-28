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
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

// reactions holds the parsed GitHub reactions JSON.
// GitLab notes do not embed reactions inline, so this only applies to GitHub raw data.
type reactions struct {
	TotalCount int `json:"total_count"`
	ThumbsUp   int `json:"+1"`
	ThumbsDown int `json:"-1"`
}

var EnrichGithubReviewReactionsMeta = plugin.SubTaskMeta{
	Name:             "enrichGithubReviewReactions",
	EntryPoint:       EnrichGithubReviewReactions,
	EnabledByDefault: true,
	Description:      "Enrich AI reviews with developer reaction data from GitHub raw API tables",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiReviewsMeta},
}

// reviewRawLink holds the mapping between a review and its raw data source
type reviewRawLink struct {
	Id           string `gorm:"column:id"`
	RawDataTable string `gorm:"column:_raw_data_table"`
	RawDataId    uint64 `gorm:"column:_raw_data_id"`
}

// EnrichGithubReviewReactions enriches AI reviews with reaction counts from raw GitHub data
func EnrichGithubReviewReactions(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	logger.Info("Starting review reaction enrichment")

	// Step 1: Query reviews joined with pull_request_comments to get raw data links
	var clauses []dal.Clause
	if data.Options.ProjectName != "" {
		clauses = []dal.Clause{
			dal.Select("ar.id, prc._raw_data_table, prc._raw_data_id"),
			dal.From("_tool_aireview_reviews ar"),
			dal.Join("JOIN pull_request_comments prc ON ar.review_id = prc.id"),
			dal.Join("JOIN pull_requests pr ON prc.pull_request_id = pr.id"),
			dal.Join("JOIN project_mapping pm ON pr.base_repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ? AND prc._raw_data_table != ''", data.Options.ProjectName, "repos"),
		}
	} else {
		clauses = []dal.Clause{
			dal.Select("ar.id, prc._raw_data_table, prc._raw_data_id"),
			dal.From("_tool_aireview_reviews ar"),
			dal.Join("JOIN pull_request_comments prc ON ar.review_id = prc.id"),
			dal.Join("JOIN pull_requests pr ON prc.pull_request_id = pr.id"),
			dal.Where("pr.base_repo_id = ? AND prc._raw_data_table != ''", data.Options.RepoId),
		}
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query review-to-raw-data links")
	}
	defer cursor.Close()

	// Group links by raw data table name for batched queries
	tableLinks := make(map[string][]reviewRawLink)
	for cursor.Next() {
		var link reviewRawLink
		if err := db.Fetch(cursor, &link); err != nil {
			return errors.Default.Wrap(err, "failed to fetch review raw link")
		}
		tableLinks[link.RawDataTable] = append(tableLinks[link.RawDataTable], link)
	}

	if len(tableLinks) == 0 {
		logger.Info("No reviews with raw data links found, skipping reaction enrichment")
		return nil
	}

	totalEnriched := 0
	totalWithReactions := 0

	// Step 2: For each raw table, batch-query reaction data
	// GitLab notes don't embed reactions inline (GitLab uses separate award_emoji API),
	// so skip GitLab raw tables as they won't have $.reactions in the JSON
	for rawTable, links := range tableLinks {
		if strings.Contains(rawTable, "gitlab") {
			logger.Info("Skipping %d reviews from GitLab raw table %s (reactions not embedded in GitLab notes)", len(links), rawTable)
			continue
		}
		logger.Info("Processing %d reviews from raw table: %s", len(links), rawTable)

		// Build lookup: raw_data_id -> review_id
		rawIdToReviewId := make(map[uint64]string)
		rawIds := make([]uint64, 0, len(links))
		for _, link := range links {
			rawIdToReviewId[link.RawDataId] = link.Id
			rawIds = append(rawIds, link.RawDataId)
		}

		// Process in batches of 500
		batchSize := 500
		for i := 0; i < len(rawIds); i += batchSize {
			end := i + batchSize
			if end > len(rawIds) {
				end = len(rawIds)
			}
			batch := rawIds[i:end]

			// Query raw table for reaction data as JSON string
			// Note: data column is stored as binary, must convert to utf8mb4 for JSON_EXTRACT
			// We extract $.reactions as a full JSON object and parse in Go to avoid
			// MySQL JSON path issues with special-character keys like "+1" and "-1"
			rows, err := db.Cursor(
				dal.Select("id, JSON_EXTRACT(CONVERT(data USING utf8mb4), '$.reactions') as reactions_json"),
				dal.From(rawTable),
				dal.Where("id IN (?)", batch),
			)
			if err != nil {
				logger.Warn(err, "failed to query reactions from %s, skipping", rawTable)
				continue
			}

			for rows.Next() {
				var id uint64
				var reactionsJSON *string
				if scanErr := rows.Scan(&id, &reactionsJSON); scanErr != nil {
					logger.Warn(errors.Default.WrapRaw(scanErr), "failed to scan reaction row")
					continue
				}

				reviewId, ok := rawIdToReviewId[id]
				if !ok {
					continue
				}

				var r reactions
				if reactionsJSON != nil {
					if parseErr := json.Unmarshal([]byte(*reactionsJSON), &r); parseErr != nil {
						logger.Warn(errors.Default.WrapRaw(parseErr), "failed to parse reactions JSON for raw id %d", id)
						continue
					}
				}

				// Update the review with reaction data
				updateErr := db.Exec(
					"UPDATE _tool_aireview_reviews SET reactions_total_count = ?, reactions_thumbs_up = ?, reactions_thumbs_down = ? WHERE id = ?",
					r.TotalCount, r.ThumbsUp, r.ThumbsDown, reviewId,
				)
				if updateErr != nil {
					logger.Warn(updateErr, "failed to update reactions for review %s", reviewId)
					continue
				}

				totalEnriched++
				if r.TotalCount > 0 {
					totalWithReactions++
				}
			}
			rows.Close()
		}
	}

	logger.Info("Reaction enrichment complete: %d reviews enriched, %d had reactions", totalEnriched, totalWithReactions)
	return nil
}
