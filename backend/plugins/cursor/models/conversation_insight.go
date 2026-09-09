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

package models

import (
	"time"

	"github.com/apache/devlake/core/models/common"
)

const (
	ConversationInsightTypeIntents         = "intents"
	ConversationInsightTypeComplexity      = "complexity"
	ConversationInsightTypeCategories      = "categories"
	ConversationInsightTypeGuidanceLevels  = "guidanceLevels"
	ConversationInsightTypeWorkTypes       = "workTypes"
	ConversationInsightTypeSubcategory     = "subcategory"

	ConversationInsightAggDistribution = "distribution"
	ConversationInsightAggTopValues    = "topValues"
	ConversationInsightAggTimeSeries     = "timeSeries"
	ConversationInsightAggSubcategory  = "subcategory"
)

// CursorConversationInsight stores flattened Conversation Insights metrics from
// GET /analytics/team/conversation-insights (Enterprise Analytics API).
type CursorConversationInsight struct {
	ConnectionId     uint64    `gorm:"primaryKey" json:"connectionId"`
	ScopeId          string    `gorm:"primaryKey;type:varchar(255)" json:"scopeId"`
	RecordId         string    `gorm:"primaryKey;type:varchar(64)" json:"recordId"`
	PeriodStart      time.Time `gorm:"index" json:"periodStart"`
	PeriodEnd        time.Time `gorm:"index" json:"periodEnd"`
	InsightType      string    `gorm:"type:varchar(32);index" json:"insightType"`
	AggregationType  string    `gorm:"type:varchar(32);index" json:"aggregationType"`
	DimensionValue   string    `gorm:"type:varchar(255);index" json:"dimensionValue"`
	SubcategoryGroup string    `gorm:"type:varchar(32);index" json:"subcategoryGroup"`
	MetricDate       *time.Time `gorm:"index" json:"metricDate"`
	Count            int       `json:"count"`

	common.NoPKModel
}

func (CursorConversationInsight) TableName() string {
	return "_tool_cursor_conversation_insights"
}
