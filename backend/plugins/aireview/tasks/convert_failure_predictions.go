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
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	domainCode "github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

var ConvertFailurePredictionsMeta = plugin.SubTaskMeta{
	Name:             "convertFailurePredictions",
	EntryPoint:       ConvertFailurePredictions,
	EnabledByDefault: true,
	Description:      "Convert tool-layer AI failure predictions into project-scoped domain table ai_failure_predictions",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&CalculateFailurePredictionsMeta},
}

// ConvertFailurePredictions reads _tool_aireview_failure_predictions scoped to
// the current project and writes project-stamped rows into ai_failure_predictions.
// Only runs in project mode; no-ops silently in single-repo mode.
func ConvertFailurePredictions(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	projectName := data.Options.ProjectName
	if projectName == "" {
		logger.Info("convertFailurePredictions: skipping — no projectName set (single-repo mode)")
		return nil
	}

	if err := db.Delete(&domainCode.AiFailurePrediction{}, dal.Where("project_name = ?", projectName)); err != nil {
		return errors.Default.Wrap(err, "failed to delete existing ai_failure_predictions for project")
	}

	// _tool_aireview_failure_predictions stores repo_id which matches project_mapping.row_id.
	cursor, err := db.Cursor(
		dal.From(&models.AiFailurePrediction{}),
		dal.Join("JOIN project_mapping pm ON _tool_aireview_failure_predictions.repo_id = pm.row_id AND pm.`table` = 'repos'"),
		dal.Where("pm.project_name = ?", projectName),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to cursor failure predictions")
	}
	defer cursor.Close()

	batch := make([]*domainCode.AiFailurePrediction, 0, 100)
	for cursor.Next() {
		var src models.AiFailurePrediction
		if fetchErr := db.Fetch(cursor, &src); fetchErr != nil {
			return errors.Default.Wrap(fetchErr, "failed to fetch failure prediction row")
		}

		batch = append(batch, &domainCode.AiFailurePrediction{
			DomainEntity: domainlayer.DomainEntity{
				Id: generateAiDomainId("afp", projectName, src.Id),
			},
			ProjectName:       projectName,
			PullRequestId:     src.PullRequestId,
			PullRequestKey:    src.PullRequestKey,
			RepoId:            src.RepoId,
			RepoName:          src.RepoName,
			AiTool:            src.AiTool,
			CiFailureSource:   src.CiFailureSource,
			PrTitle:           src.PrTitle,
			PrUrl:             src.PrUrl,
			PrAuthor:          src.PrAuthor,
			PrCreatedAt:       src.PrCreatedAt,
			Additions:         src.Additions,
			Deletions:         src.Deletions,
			WasFlaggedRisky:   src.WasFlaggedRisky,
			RiskScore:         src.RiskScore,
			FlaggedAt:         src.FlaggedAt,
			HadCiFailure:      src.HadCiFailure,
			PredictionOutcome: src.PredictionOutcome,
		})

		if len(batch) >= 100 {
			if saveErr := saveFailurePredictionBatch(db, batch); saveErr != nil {
				return saveErr
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if saveErr := saveFailurePredictionBatch(db, batch); saveErr != nil {
			return saveErr
		}
	}

	logger.Info("convertFailurePredictions: done for project %s", projectName)
	return nil
}

func saveFailurePredictionBatch(db dal.Dal, batch []*domainCode.AiFailurePrediction) errors.Error {
	for _, p := range batch {
		if err := db.CreateOrUpdate(p); err != nil {
			return errors.Default.Wrap(err, "failed to save domain failure prediction")
		}
	}
	return nil
}
