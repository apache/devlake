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
	"strings"
	"testing"
	"time"

	"github.com/apache/devlake/core/errors"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	"github.com/apache/devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDetermineAutonomyLevel(t *testing.T) {
	tests := []struct {
		name      string
		precision float64
		recall    float64
		want      string
	}{
		{"auto_block: high precision and recall", 0.85, 0.75, models.AutonomyAutoBlock},
		{"auto_block: exact boundary", 0.80, 0.70, models.AutonomyAutoBlock},
		{"mandatory_review: medium precision and recall", 0.65, 0.55, models.AutonomyMandatoryReview},
		{"mandatory_review: exact boundary", 0.60, 0.50, models.AutonomyMandatoryReview},
		{"mandatory_review: high precision low recall", 0.90, 0.55, models.AutonomyMandatoryReview},
		{"advisory_only: low precision", 0.50, 0.80, models.AutonomyAdvisoryOnly},
		{"advisory_only: low recall", 0.70, 0.40, models.AutonomyAdvisoryOnly},
		{"advisory_only: both low", 0.30, 0.30, models.AutonomyAdvisoryOnly},
		{"advisory_only: zero values", 0.0, 0.0, models.AutonomyAdvisoryOnly},
		{"auto_block: perfect scores", 1.0, 1.0, models.AutonomyAutoBlock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, determineAutonomyLevel(tt.precision, tt.recall))
		})
	}
}

func TestGenerateMetricsId(t *testing.T) {
	ts := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	id1 := generateMetricsId("repo1", "CodeRabbit", "test_cases", "weekly", ts)
	id2 := generateMetricsId("repo1", "CodeRabbit", "test_cases", "weekly", ts)
	id3 := generateMetricsId("repo1", "CodeRabbit", "test_cases", "monthly", ts)
	id4 := generateMetricsId("repo2", "CodeRabbit", "test_cases", "weekly", ts)
	id5 := generateMetricsId("repo1", "Qodo", "test_cases", "weekly", ts)

	assert.Equal(t, id1, id2, "same inputs must produce same ID")
	assert.NotEqual(t, id1, id3, "different period type must produce different ID")
	assert.NotEqual(t, id1, id4, "different repo must produce different ID")
	assert.NotEqual(t, id1, id5, "different tool must produce different ID")
	assert.True(t, strings.HasPrefix(id1, "aimetrics:"))
}

func TestComputeAucs(t *testing.T) {
	t.Run("fewer than 2 points returns zeros", func(t *testing.T) {
		prAuc, rocAuc := computeAucs([]predictionPoint{
			{RiskScore: 80, HadCiFailure: true},
		})
		assert.Equal(t, 0.0, prAuc)
		assert.Equal(t, 0.0, rocAuc)
	})

	t.Run("empty points returns zeros", func(t *testing.T) {
		prAuc, rocAuc := computeAucs(nil)
		assert.Equal(t, 0.0, prAuc)
		assert.Equal(t, 0.0, rocAuc)
	})

	t.Run("perfect classifier", func(t *testing.T) {
		points := []predictionPoint{
			{RiskScore: 90, HadCiFailure: true},
			{RiskScore: 85, HadCiFailure: true},
			{RiskScore: 10, HadCiFailure: false},
			{RiskScore: 5, HadCiFailure: false},
		}
		prAuc, rocAuc := computeAucs(points)
		assert.Greater(t, prAuc, 0.0, "PR-AUC should be positive for a good classifier")
		assert.Greater(t, rocAuc, 0.0, "ROC-AUC should be positive for a good classifier")
	})

	t.Run("random classifier has lower AUC", func(t *testing.T) {
		goodPoints := []predictionPoint{
			{RiskScore: 90, HadCiFailure: true},
			{RiskScore: 85, HadCiFailure: true},
			{RiskScore: 10, HadCiFailure: false},
			{RiskScore: 5, HadCiFailure: false},
		}
		randomPoints := []predictionPoint{
			{RiskScore: 90, HadCiFailure: false},
			{RiskScore: 85, HadCiFailure: true},
			{RiskScore: 10, HadCiFailure: true},
			{RiskScore: 5, HadCiFailure: false},
		}
		goodPr, goodRoc := computeAucs(goodPoints)
		randPr, randRoc := computeAucs(randomPoints)

		assert.Greater(t, goodPr, randPr, "good classifier should have higher PR-AUC")
		assert.Greater(t, goodRoc, randRoc, "good classifier should have higher ROC-AUC")
	})

	t.Run("all same risk score", func(t *testing.T) {
		points := []predictionPoint{
			{RiskScore: 50, HadCiFailure: true},
			{RiskScore: 50, HadCiFailure: false},
			{RiskScore: 50, HadCiFailure: true},
		}
		prAuc, rocAuc := computeAucs(points)
		assert.GreaterOrEqual(t, prAuc, 0.0)
		assert.GreaterOrEqual(t, rocAuc, 0.0)
	})
}

func TestComputeMetrics(t *testing.T) {
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	t.Run("all true positives", func(t *testing.T) {
		points := []predictionPoint{
			{RiskScore: 80, HadCiFailure: true},
			{RiskScore: 90, HadCiFailure: true},
			{RiskScore: 70, HadCiFailure: true},
		}
		m := computeMetrics("repo1", "CodeRabbit", "test_cases", "weekly", weekAgo, now, points, points, 50)

		assert.Equal(t, 3, m.TruePositives)
		assert.Equal(t, 0, m.FalsePositives)
		assert.Equal(t, 0, m.FalseNegatives)
		assert.Equal(t, 0, m.TrueNegatives)
		assert.Equal(t, 1.0, m.Precision)
		assert.Equal(t, 1.0, m.Recall)
		assert.Equal(t, 1.0, m.Accuracy)
		assert.Equal(t, 1.0, m.F1Score)
		assert.Equal(t, "repo1", m.RepoId)
		assert.Equal(t, "CodeRabbit", m.AiTool)
		assert.Equal(t, "weekly", m.PeriodType)
		assert.Equal(t, 50, m.WarningThreshold)
	})

	t.Run("all true negatives", func(t *testing.T) {
		points := []predictionPoint{
			{RiskScore: 10, HadCiFailure: false},
			{RiskScore: 20, HadCiFailure: false},
		}
		m := computeMetrics("repo1", "CodeRabbit", "test_cases", "daily", weekAgo, now, points, points, 50)

		assert.Equal(t, 0, m.TruePositives)
		assert.Equal(t, 0, m.FalsePositives)
		assert.Equal(t, 0, m.FalseNegatives)
		assert.Equal(t, 2, m.TrueNegatives)
		assert.Equal(t, 0.0, m.Precision)
		assert.Equal(t, 0.0, m.Recall)
		assert.Equal(t, 1.0, m.Accuracy)
		assert.Equal(t, 1.0, m.Specificity)
	})

	t.Run("mixed confusion matrix", func(t *testing.T) {
		points := []predictionPoint{
			{RiskScore: 80, HadCiFailure: true},  // TP
			{RiskScore: 70, HadCiFailure: false}, // FP
			{RiskScore: 20, HadCiFailure: true},  // FN
			{RiskScore: 10, HadCiFailure: false}, // TN
		}
		m := computeMetrics("repo1", "CodeRabbit", "job_result", "monthly", weekAgo, now, points, points, 50)

		assert.Equal(t, 1, m.TruePositives)
		assert.Equal(t, 1, m.FalsePositives)
		assert.Equal(t, 1, m.FalseNegatives)
		assert.Equal(t, 1, m.TrueNegatives)
		assert.InDelta(t, 0.5, m.Precision, 0.001)
		assert.InDelta(t, 0.5, m.Recall, 0.001)
		assert.InDelta(t, 0.5, m.Accuracy, 0.001)
		assert.InDelta(t, 0.5, m.F1Score, 0.001)
		assert.InDelta(t, 0.5, m.Specificity, 0.001)
		assert.InDelta(t, 50.0, m.FprPct, 0.001)
		assert.Equal(t, 4, m.TotalPrs)
		assert.Equal(t, 2, m.FlaggedPrs)
		assert.Equal(t, 2, m.FailedPrs)
	})

	t.Run("zero division safety with empty points", func(t *testing.T) {
		points := []predictionPoint{}
		m := computeMetrics("repo1", "CodeRabbit", "test_cases", "daily", weekAgo, now, points, points, 50)

		assert.Equal(t, 0.0, m.Precision)
		assert.Equal(t, 0.0, m.Recall)
		assert.Equal(t, 0.0, m.Accuracy)
		assert.Equal(t, 0.0, m.F1Score)
		assert.Equal(t, 0, m.TotalPrs)
	})

	t.Run("autonomy level propagated correctly", func(t *testing.T) {
		highPrecisionPoints := []predictionPoint{
			{RiskScore: 80, HadCiFailure: true},
			{RiskScore: 90, HadCiFailure: true},
			{RiskScore: 85, HadCiFailure: true},
			{RiskScore: 95, HadCiFailure: true},
			{RiskScore: 75, HadCiFailure: true},
			{RiskScore: 10, HadCiFailure: false},
			{RiskScore: 5, HadCiFailure: false},
		}
		m := computeMetrics("repo1", "CodeRabbit", "test_cases", "weekly", weekAgo, now, highPrecisionPoints, highPrecisionPoints, 50)
		assert.Equal(t, models.AutonomyAutoBlock, m.RecommendedAutonomyLevel)
	})

	t.Run("metrics ID is deterministic", func(t *testing.T) {
		points := []predictionPoint{{RiskScore: 50, HadCiFailure: true}}
		m1 := computeMetrics("repo1", "CodeRabbit", "test_cases", "weekly", weekAgo, now, points, points, 50)
		m2 := computeMetrics("repo1", "CodeRabbit", "test_cases", "weekly", weekAgo, now, points, points, 50)
		assert.Equal(t, m1.Id, m2.Id)
		assert.True(t, strings.HasPrefix(m1.Id, "aimetrics:"))
	})

	t.Run("ci_failure_source preserved", func(t *testing.T) {
		points := []predictionPoint{{RiskScore: 50, HadCiFailure: true}}
		m := computeMetrics("repo1", "CodeRabbit", "job_result", "daily", weekAgo, now, points, points, 50)
		assert.Equal(t, "job_result", m.CiFailureSource)
	})
}

func TestLoadPredictionPoints(t *testing.T) {
	t.Run("all-time (start is zero)", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]struct {
				RiskScore    int  `gorm:"column:risk_score"`
				HadCiFailure bool `gorm:"column:had_ci_failure"`
			})
			*dst = []struct {
				RiskScore    int  `gorm:"column:risk_score"`
				HadCiFailure bool `gorm:"column:had_ci_failure"`
			}{
				{RiskScore: 80, HadCiFailure: true},
				{RiskScore: 20, HadCiFailure: false},
				{RiskScore: 60, HadCiFailure: true},
			}
		}).Return(nil)

		points, err := loadPredictionPoints(mockDal, "repo-1", "CodeRabbit", "test_cases", time.Time{}, time.Now())
		assert.Nil(t, err)
		assert.Len(t, points, 3)
		assert.Equal(t, 80, points[0].RiskScore)
		assert.True(t, points[0].HadCiFailure)
		assert.Equal(t, 20, points[1].RiskScore)
		assert.False(t, points[1].HadCiFailure)
	})

	t.Run("with time range", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			dst := args.Get(0).(*[]struct {
				RiskScore    int  `gorm:"column:risk_score"`
				HadCiFailure bool `gorm:"column:had_ci_failure"`
			})
			*dst = []struct {
				RiskScore    int  `gorm:"column:risk_score"`
				HadCiFailure bool `gorm:"column:had_ci_failure"`
			}{
				{RiskScore: 50, HadCiFailure: false},
			}
		}).Return(nil)

		now := time.Now()
		start := now.AddDate(0, 0, -7)
		points, err := loadPredictionPoints(mockDal, "repo-1", "CodeRabbit", "job_result", start, now)
		assert.Nil(t, err)
		assert.Len(t, points, 1)
		assert.Equal(t, 50, points[0].RiskScore)
		assert.False(t, points[0].HadCiFailure)
	})

	t.Run("error", func(t *testing.T) {
		mockDal := new(mockdal.Dal)
		mockDal.On("All", mock.Anything, mock.Anything).
			Return(errors.Default.New("db error"))

		points, err := loadPredictionPoints(mockDal, "repo-1", "CodeRabbit", "test_cases", time.Time{}, time.Now())
		assert.NotNil(t, err)
		assert.Nil(t, points)
		assert.Contains(t, err.Error(), "prediction points")
	})
}
