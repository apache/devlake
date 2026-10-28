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
	"math"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aireview/models"
)

var CalculatePredictionMetricsMeta = plugin.SubTaskMeta{
	Name:             "calculatePredictionMetrics",
	EntryPoint:       CalculatePredictionMetrics,
	EnabledByDefault: true,
	Description:      "Calculate aggregated AI prediction metrics (precision, recall, AUC) from CI outcomes",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&CalculateFailurePredictionsMeta},
}

// aucThresholds are the risk_score cut-points used when computing PR-AUC and ROC-AUC.
// They match the sensitivity levels used in the Grafana dashboard.
var aucThresholds = []int{0, 10, 20, 50, 80, 100}

// predictionPoint is the minimal data needed for AUC computation.
type predictionPoint struct {
	RiskScore    int
	HadCiFailure bool
}

// CalculatePredictionMetrics aggregates prediction data into period metrics.
func CalculatePredictionMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	warningThreshold := data.Options.ScopeConfig.WarningThreshold
	if warningThreshold == 0 {
		warningThreshold = 50
	}

	logger.Info("Calculating prediction metrics for repo: %s", data.Options.RepoId)

	// Get distinct (repo_id, ai_tool, ci_failure_source) triplets that have completed predictions.
	// Supports both single-repo mode and project mode (repoId empty).
	var toolRows []struct {
		RepoId          string `gorm:"column:repo_id"`
		AiTool          string `gorm:"column:ai_tool"`
		CiFailureSource string `gorm:"column:ci_failure_source"`
	}
	toolQuery := []dal.Clause{
		dal.Select("DISTINCT repo_id, ai_tool, ci_failure_source"),
		dal.From(&models.AiFailurePrediction{}),
	}
	if data.Options.RepoId != "" {
		toolQuery = append(toolQuery, dal.Where("repo_id = ? AND prediction_outcome != ''", data.Options.RepoId))
	} else {
		toolQuery = append(toolQuery, dal.Where("prediction_outcome != ''"))
	}
	if err := db.All(&toolRows, toolQuery...); err != nil {
		return errors.Default.Wrap(err, "failed to get repo/tool/source triplets")
	}

	now := time.Now()
	periods := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{"daily", now.AddDate(0, 0, -1), now},
		{"weekly", now.AddDate(0, 0, -7), now},
		{"monthly", now.AddDate(0, -1, 0), now},
		{"rolling_60d", now.AddDate(0, 0, -60), now},
	}

	for _, tr := range toolRows {
		if tr.AiTool == "" || tr.RepoId == "" {
			continue
		}

		// Load all prediction points for this (repo, tool, source) triplet for AUC.
		allPoints, err := loadPredictionPoints(db, tr.RepoId, tr.AiTool, tr.CiFailureSource, time.Time{}, now)
		if err != nil {
			logger.Warn(err, "Failed to load prediction points for %s/%s/%s", tr.RepoId, tr.AiTool, tr.CiFailureSource)
			continue
		}

		for _, period := range periods {
			periodPoints, err := loadPredictionPoints(db, tr.RepoId, tr.AiTool, tr.CiFailureSource, period.start, period.end)
			if err != nil {
				logger.Warn(err, "Failed to load period prediction points for %s/%s/%s/%s", tr.RepoId, tr.AiTool, tr.CiFailureSource, period.name)
				continue
			}
			if len(periodPoints) == 0 {
				continue
			}

			aucPoints := periodPoints
			if len(periodPoints) < 5 {
				aucPoints = allPoints
			}

			metrics := computeMetrics(tr.RepoId, tr.AiTool, tr.CiFailureSource, period.name, period.start, period.end, periodPoints, aucPoints, warningThreshold)

			if err := db.CreateOrUpdate(metrics); err != nil {
				return errors.Default.Wrap(err, "failed to save prediction metrics")
			}
		}
	}

	logger.Info("Completed prediction metrics calculation")
	return nil
}

// loadPredictionPoints fetches risk_score + had_ci_failure for all completed
// predictions for a given (repo, tool, ci_failure_source). When start is zero,
// no time filter is applied (all-time).
func loadPredictionPoints(db dal.Dal, repoId, aiTool, ciFailureSource string, start, end time.Time) ([]predictionPoint, errors.Error) {
	var rows []struct {
		RiskScore    int  `gorm:"column:risk_score"`
		HadCiFailure bool `gorm:"column:had_ci_failure"`
	}

	var err errors.Error
	if start.IsZero() {
		err = db.All(&rows,
			dal.Select("risk_score, had_ci_failure"),
			dal.From(&models.AiFailurePrediction{}),
			dal.Where("repo_id = ? AND ai_tool = ? AND ci_failure_source = ? AND prediction_outcome != ''", repoId, aiTool, ciFailureSource),
		)
	} else {
		err = db.All(&rows,
			dal.Select("risk_score, had_ci_failure"),
			dal.From(&models.AiFailurePrediction{}),
			dal.Where("repo_id = ? AND ai_tool = ? AND ci_failure_source = ? AND prediction_outcome != '' AND created_at BETWEEN ? AND ?", repoId, aiTool, ciFailureSource, start, end),
		)
	}
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query prediction points")
	}

	points := make([]predictionPoint, len(rows))
	for i, r := range rows {
		points[i] = predictionPoint{RiskScore: r.RiskScore, HadCiFailure: r.HadCiFailure}
	}
	return points, nil
}

// computeMetrics builds an AiPredictionMetrics record from prediction points.
func computeMetrics(repoId, aiTool, ciFailureSource, periodType string, periodStart, periodEnd time.Time,
	periodPoints, aucPoints []predictionPoint, warningThreshold int) *models.AiPredictionMetrics {

	// Confusion matrix at warning_threshold.
	var tp, fp, fn, tn int
	for _, p := range periodPoints {
		flagged := p.RiskScore >= warningThreshold
		switch {
		case flagged && p.HadCiFailure:
			tp++
		case flagged && !p.HadCiFailure:
			fp++
		case !flagged && p.HadCiFailure:
			fn++
		default:
			tn++
		}
	}

	ftpF := float64(tp)
	ffpF := float64(fp)
	ffnF := float64(fn)
	ftnF := float64(tn)
	total := ftpF + ffpF + ffnF + ftnF

	precision, recall, accuracy, f1, specificity, fprPct := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0

	if ftpF+ffpF > 0 {
		precision = ftpF / (ftpF + ffpF)
	}
	if ftpF+ffnF > 0 {
		recall = ftpF / (ftpF + ffnF)
	}
	if total > 0 {
		accuracy = (ftpF + ftnF) / total
	}
	if precision+recall > 0 {
		f1 = 2 * (precision * recall) / (precision + recall)
	}
	if ftnF+ffpF > 0 {
		specificity = ftnF / (ftnF + ffpF)
		fprPct = ffpF / (ftnF + ffpF) * 100
	}

	// AUC metrics from the broader point set.
	prAuc, rocAuc := computeAucs(aucPoints)

	// Sample sizes.
	flaggedPrs := tp + fp
	failedPrs := tp + fn

	return &models.AiPredictionMetrics{
		Id:                       generateMetricsId(repoId, aiTool, ciFailureSource, periodType, periodStart),
		RepoId:                   repoId,
		AiTool:                   aiTool,
		CiFailureSource:          ciFailureSource,
		PeriodStart:              periodStart,
		PeriodEnd:                periodEnd,
		PeriodType:               periodType,
		TruePositives:            tp,
		FalsePositives:           fp,
		FalseNegatives:           fn,
		TrueNegatives:            tn,
		Precision:                precision,
		Recall:                   recall,
		Accuracy:                 accuracy,
		F1Score:                  f1,
		Specificity:              specificity,
		FprPct:                   fprPct,
		PrAuc:                    prAuc,
		RocAuc:                   rocAuc,
		TotalPrs:                 len(periodPoints),
		FlaggedPrs:               flaggedPrs,
		FailedPrs:                failedPrs,
		ObservedPrs:              len(periodPoints),
		WarningThreshold:         warningThreshold,
		RecommendedAutonomyLevel: determineAutonomyLevel(precision, recall),
		CalculatedAt:             time.Now(),
	}
}

// curvePoint holds precision-recall and ROC coordinates for one threshold.
type curvePoint struct {
	threshold int
	precision float64
	recall    float64
	fpr       float64
	tpr       float64
}

// computeAucs computes PR-AUC and ROC-AUC using a trapezoidal rule over
// aucThresholds. Returns (0, 0) when there are fewer than 2 points.
func computeAucs(points []predictionPoint) (prAuc, rocAuc float64) {
	if len(points) < 2 {
		return 0, 0
	}

	curve := make([]curvePoint, len(aucThresholds))
	for i, t := range aucThresholds {
		var tp, fp, fn, tn int
		for _, p := range points {
			flagged := p.RiskScore >= t
			switch {
			case flagged && p.HadCiFailure:
				tp++
			case flagged && !p.HadCiFailure:
				fp++
			case !flagged && p.HadCiFailure:
				fn++
			default:
				tn++
			}
		}

		prec := 1.0 // by convention when nothing is flagged
		if tp+fp > 0 {
			prec = float64(tp) / float64(tp+fp)
		}
		rec := 0.0
		if tp+fn > 0 {
			rec = float64(tp) / float64(tp+fn)
		}
		tpr := rec
		fpr := 0.0
		if fp+tn > 0 {
			fpr = float64(fp) / float64(fp+tn)
		}

		curve[i] = curvePoint{threshold: t, precision: prec, recall: rec, fpr: fpr, tpr: tpr}
	}

	// PR-AUC: trapezoidal over recall axis (thresholds ascending → recall descending).
	// Sum |Δrecall| × average precision between consecutive points.
	for i := 1; i < len(curve); i++ {
		prev, curr := curve[i-1], curve[i]
		deltaRecall := math.Abs(curr.recall - prev.recall)
		prAuc += deltaRecall * (curr.precision + prev.precision) / 2
	}

	// ROC-AUC: trapezoidal over FPR axis (thresholds descending → FPR increasing).
	// Reverse the curve so FPR goes 0 → 1.
	for i := len(curve) - 2; i >= 0; i-- {
		curr, next := curve[i+1], curve[i]
		deltaFPR := next.fpr - curr.fpr
		if deltaFPR > 0 {
			rocAuc += deltaFPR * (next.tpr + curr.tpr) / 2
		}
	}

	return prAuc, rocAuc
}

// determineAutonomyLevel recommends AI autonomy based on precision and recall.
func determineAutonomyLevel(precision, recall float64) string {
	if precision >= 0.80 && recall >= 0.70 {
		return models.AutonomyAutoBlock
	}
	if precision >= 0.60 && recall >= 0.50 {
		return models.AutonomyMandatoryReview
	}
	return models.AutonomyAdvisoryOnly
}

// generateMetricsId creates a deterministic ID for a metrics record.
func generateMetricsId(repoId, aiTool, ciFailureSource, periodType string, periodStart time.Time) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%s:%s", repoId, aiTool, ciFailureSource, periodType, periodStart.Format("2006-01-02"))))
	return "aimetrics:" + hex.EncodeToString(hash[:16])
}
