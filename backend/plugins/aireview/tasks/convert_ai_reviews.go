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
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	domainCode "github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

var ConvertAiReviewsMeta = plugin.SubTaskMeta{
	Name:             "convertAiReviews",
	EntryPoint:       ConvertAiReviews,
	EnabledByDefault: true,
	Description:      "Convert tool-layer AI reviews into project-scoped domain table ai_reviews",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiReviewsMeta},
}

// ConvertAiReviews reads _tool_aireview_reviews scoped to the current project
// and writes project-stamped rows into the ai_reviews domain table.
// Only runs in project mode; no-ops silently in single-repo mode.
func ConvertAiReviews(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	projectName := data.Options.ProjectName
	if projectName == "" {
		logger.Info("convertAiReviews: skipping — no projectName set (single-repo mode)")
		return nil
	}

	// Clear existing domain records for this project so re-runs are idempotent.
	if err := db.Delete(&domainCode.AiReview{}, dal.Where("project_name = ?", projectName)); err != nil {
		return errors.Default.Wrap(err, "failed to delete existing ai_reviews for project")
	}

	cursor, err := db.Cursor(
		dal.From(&models.AiReview{}),
		dal.Join("JOIN project_mapping pm ON _tool_aireview_reviews.repo_id = pm.row_id AND pm.`table` = 'repos'"),
		dal.Where("pm.project_name = ? AND _tool_aireview_reviews.body NOT LIKE '%Review skipped%'", projectName),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to cursor ai reviews")
	}
	defer cursor.Close()

	batch := make([]*domainCode.AiReview, 0, 100)
	for cursor.Next() {
		var src models.AiReview
		if fetchErr := db.Fetch(cursor, &src); fetchErr != nil {
			return errors.Default.Wrap(fetchErr, "failed to fetch ai review row")
		}

		batch = append(batch, &domainCode.AiReview{
			DomainEntity: domainlayer.DomainEntity{
				Id: generateAiDomainId("ar", projectName, src.Id),
			},
			ProjectName:          projectName,
			PullRequestId:        src.PullRequestId,
			RepoId:               src.RepoId,
			AiTool:               src.AiTool,
			CreatedDate:          src.CreatedDate,
			RiskLevel:            src.RiskLevel,
			RiskScore:            src.RiskScore,
			IssuesFound:          src.IssuesFound,
			SuggestionsCount:     src.SuggestionsCount,
			PreMergeChecksPassed: src.PreMergeChecksPassed,
			PreMergeChecksFailed: src.PreMergeChecksFailed,
			ReactionsTotalCount:  src.ReactionsTotalCount,
			ReactionsThumbsUp:    src.ReactionsThumbsUp,
			ReactionsThumbsDown:  src.ReactionsThumbsDown,
			ReviewState:          src.ReviewState,
			SourceUrl:            src.SourceUrl,
		})

		if len(batch) >= 100 {
			if saveErr := saveAiReviewBatch(db, batch); saveErr != nil {
				return saveErr
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if saveErr := saveAiReviewBatch(db, batch); saveErr != nil {
			return saveErr
		}
	}

	logger.Info("convertAiReviews: done for project %s", projectName)
	return nil
}

func saveAiReviewBatch(db dal.Dal, batch []*domainCode.AiReview) errors.Error {
	for _, r := range batch {
		if err := db.CreateOrUpdate(r); err != nil {
			return errors.Default.Wrap(err, "failed to save domain ai review")
		}
	}
	return nil
}

// generateAiDomainId creates a stable, project-scoped domain ID from a prefix,
// project name, and the source tool-layer record ID.
func generateAiDomainId(prefix, projectName, sourceId string) string {
	hash := sha256.Sum256(fmt.Appendf(nil, "%s:%s:%s", prefix, projectName, sourceId))
	return prefix + ":" + hex.EncodeToString(hash[:16])
}
