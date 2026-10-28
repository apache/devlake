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

var ConvertPredictionMetricsMeta = plugin.SubTaskMeta{
	Name:             "convertPredictionMetrics",
	EntryPoint:       ConvertPredictionMetrics,
	EnabledByDefault: true,
	Description:      "Convert tool-layer prediction metrics into project-scoped domain table ai_prediction_metrics",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&CalculatePredictionMetricsMeta},
}

// ConvertPredictionMetrics reads _tool_aireview_prediction_metrics scoped to
// the current project and writes project-stamped rows into ai_prediction_metrics.
// Only runs in project mode; no-ops silently in single-repo mode.
func ConvertPredictionMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	projectName := data.Options.ProjectName
	if projectName == "" {
		logger.Info("convertPredictionMetrics: skipping — no projectName set (single-repo mode)")
		return nil
	}

	if err := db.Delete(&domainCode.AiPredictionMetrics{}, dal.Where("project_name = ?", projectName)); err != nil {
		return errors.Default.Wrap(err, "failed to delete existing ai_prediction_metrics for project")
	}

	// _tool_aireview_prediction_metrics is keyed by repo_id, which matches project_mapping.row_id.
	cursor, err := db.Cursor(
		dal.From(&models.AiPredictionMetrics{}),
		dal.Join("JOIN project_mapping pm ON _tool_aireview_prediction_metrics.repo_id = pm.row_id AND pm.`table` = 'repos'"),
		dal.Where("pm.project_name = ?", projectName),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to cursor prediction metrics")
	}
	defer cursor.Close()

	batch := make([]*domainCode.AiPredictionMetrics, 0, 100)
	for cursor.Next() {
		var src models.AiPredictionMetrics
		if fetchErr := db.Fetch(cursor, &src); fetchErr != nil {
			return errors.Default.Wrap(fetchErr, "failed to fetch prediction metrics row")
		}

		batch = append(batch, &domainCode.AiPredictionMetrics{
			DomainEntity: domainlayer.DomainEntity{
				Id: generateAiDomainId("apm", projectName, src.Id),
			},
			ProjectName:              projectName,
			RepoId:                   src.RepoId,
			AiTool:                   src.AiTool,
			CiFailureSource:          src.CiFailureSource,
			PeriodStart:              src.PeriodStart,
			PeriodEnd:                src.PeriodEnd,
			PeriodType:               src.PeriodType,
			TruePositives:            src.TruePositives,
			FalsePositives:           src.FalsePositives,
			FalseNegatives:           src.FalseNegatives,
			TrueNegatives:            src.TrueNegatives,
			Precision:                src.Precision,
			Recall:                   src.Recall,
			Accuracy:                 src.Accuracy,
			F1Score:                  src.F1Score,
			Specificity:              src.Specificity,
			FprPct:                   src.FprPct,
			PrAuc:                    src.PrAuc,
			RocAuc:                   src.RocAuc,
			WarningThreshold:         src.WarningThreshold,
			TotalPrs:                 src.TotalPrs,
			FlaggedPrs:               src.FlaggedPrs,
			FailedPrs:                src.FailedPrs,
			RecommendedAutonomyLevel: src.RecommendedAutonomyLevel,
			CalculatedAt:             src.CalculatedAt,
		})

		if len(batch) >= 100 {
			if saveErr := savePredictionMetricsBatch(db, batch); saveErr != nil {
				return saveErr
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if saveErr := savePredictionMetricsBatch(db, batch); saveErr != nil {
			return saveErr
		}
	}

	logger.Info("convertPredictionMetrics: done for project %s", projectName)
	return nil
}

func savePredictionMetricsBatch(db dal.Dal, batch []*domainCode.AiPredictionMetrics) errors.Error {
	for _, m := range batch {
		if err := db.CreateOrUpdate(m); err != nil {
			return errors.Default.Wrap(err, "failed to save domain prediction metrics")
		}
	}
	return nil
}
