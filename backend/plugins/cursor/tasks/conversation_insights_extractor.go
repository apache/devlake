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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/cursor/models"
)

type conversationInsightsPayload struct {
	Data   conversationInsightsData   `json:"data"`
	Params conversationInsightsParams `json:"params"`
}

type conversationInsightsData struct {
	Intents         *conversationInsightBlock `json:"intents"`
	Complexity      *conversationInsightBlock `json:"complexity"`
	Categories      *conversationInsightBlock `json:"categories"`
	GuidanceLevels  *conversationInsightBlock `json:"guidanceLevels"`
	WorkTypes       *conversationInsightBlock `json:"workTypes"`
}

type conversationInsightsParams struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type conversationInsightBlock struct {
	Distribution  []json.RawMessage            `json:"distribution"`
	TopValues     []json.RawMessage            `json:"topValues"`
	TimeSeries    []json.RawMessage            `json:"timeSeries"`
	Subcategories map[string][]json.RawMessage `json:"subcategories"`
}

type insightSpec struct {
	insightType   string
	valueField    string
	timeField     string
	block         *conversationInsightBlock
	includeTop    bool
	includeSubcat bool
}

// ExtractConversationInsights parses raw Conversation Insights payloads into tool-layer rows.
func ExtractConversationInsights(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if data.Connection.KeyTier != models.KeyTierEnterprise {
		taskCtx.GetLogger().Info("Skipping Conversation Insights extraction: key tier is %q, not enterprise", data.Connection.KeyTier)
		return nil
	}
	if !data.Connection.HasConversationInsights {
		taskCtx.GetLogger().Info("Skipping Conversation Insights extraction: connection lacks conversation-insights access")
		return nil
	}

	extractor, err := newCursorStatefulExtractor(&cursorStatefulExtractorArgs[conversationInsightsPayload]{
		SubtaskCommonArgs: cursorSubtaskCommonArgs(taskCtx, data, rawConversationInsightsTable),
		ConnectionId:      data.Options.ConnectionId,
		ScopeId:           data.Options.ScopeId,
		ToolTable:         models.CursorConversationInsight{}.TableName(),
		Extract: func(record *conversationInsightsPayload, _ *helper.RawData) ([]any, errors.Error) {
			periodStart, periodEnd, err := parseConversationInsightPeriod(record.Params.StartDate, record.Params.EndDate)
			if err != nil {
				return nil, err
			}

			specs := []insightSpec{
				{
					insightType:   models.ConversationInsightTypeIntents,
					valueField:    "intent",
					timeField:     "intent",
					block:         record.Data.Intents,
					includeTop:    true,
					includeSubcat: true,
				},
				{
					insightType: models.ConversationInsightTypeComplexity,
					valueField:  "complexity",
					timeField:   "complexity",
					block:       record.Data.Complexity,
				},
				{
					insightType: models.ConversationInsightTypeCategories,
					valueField:  "category",
					timeField:   "category",
					block:       record.Data.Categories,
				},
				{
					insightType: models.ConversationInsightTypeGuidanceLevels,
					valueField:  "guidanceLevel",
					timeField:   "guidanceLevel",
					block:       record.Data.GuidanceLevels,
				},
				{
					insightType: models.ConversationInsightTypeWorkTypes,
					valueField:  "workType",
					timeField:   "workType",
					block:       record.Data.WorkTypes,
				},
			}

			var rows []any
			for _, spec := range specs {
				if spec.block == nil {
					continue
				}
				extracted, extractErr := flattenConversationInsightBlock(
					data.Options.ConnectionId,
					data.Options.ScopeId,
					periodStart,
					periodEnd,
					spec,
				)
				if extractErr != nil {
					return nil, extractErr
				}
				rows = append(rows, extracted...)
			}
			return rows, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func flattenConversationInsightBlock(
	connectionId uint64,
	scopeId string,
	periodStart, periodEnd time.Time,
	spec insightSpec,
) ([]any, errors.Error) {
	var rows []any

	appendRows := func(aggregationType, dimensionValue, subcategoryGroup, metricDateKey string, metricDate *time.Time, count int) errors.Error {
		dimensionValue = strings.TrimSpace(dimensionValue)
		if dimensionValue == "" || count < 0 {
			return nil
		}
		row := &models.CursorConversationInsight{
			ConnectionId:     connectionId,
			ScopeId:          scopeId,
			RecordId: computeConversationInsightRecordId(
				periodStart, periodEnd, spec.insightType, aggregationType,
				dimensionValue, subcategoryGroup, metricDateKey,
			),
			PeriodStart:      periodStart,
			PeriodEnd:        periodEnd,
			InsightType:      spec.insightType,
			AggregationType:  aggregationType,
			DimensionValue:   dimensionValue,
			SubcategoryGroup: subcategoryGroup,
			MetricDate:       metricDate,
			Count:            count,
		}
		rows = append(rows, row)
		return nil
	}

	for _, item := range spec.block.Distribution {
		value, count, err := parseConversationInsightCountItem(item, spec.valueField)
		if err != nil {
			return nil, err
		}
		if err := appendRows(models.ConversationInsightAggDistribution, value, "", "", nil, count); err != nil {
			return nil, err
		}
	}

	if spec.includeTop {
		for _, item := range spec.block.TopValues {
			value, count, err := parseConversationInsightCountItem(item, spec.valueField)
			if err != nil {
				return nil, err
			}
			if err := appendRows(models.ConversationInsightAggTopValues, value, "", "", nil, count); err != nil {
				return nil, err
			}
		}
	}

	for _, item := range spec.block.TimeSeries {
		value, metricDate, count, err := parseConversationInsightTimeSeriesItem(item, spec.timeField)
		if err != nil {
			return nil, err
		}
		metricDateKey := ""
		if metricDate != nil {
			metricDateKey = metricDate.Format("2006-01-02")
		}
		if err := appendRows(models.ConversationInsightAggTimeSeries, value, "", metricDateKey, metricDate, count); err != nil {
			return nil, err
		}
	}

	if spec.includeSubcat {
		for group, items := range spec.block.Subcategories {
			for _, item := range items {
				value, count, err := parseConversationInsightCountItem(item, "subcategory")
				if err != nil {
					return nil, err
				}
				if err := appendRows(models.ConversationInsightAggSubcategory, value, group, "", nil, count); err != nil {
					return nil, err
				}
			}
		}
	}

	return rows, nil
}

func parseConversationInsightPeriod(startDate, endDate string) (time.Time, time.Time, errors.Error) {
	start, err := parseConversationInsightDate(startDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := parseConversationInsightDate(endDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, errors.BadInput.New("conversation insights period end is before start")
	}
	return start, end, nil
}

func parseConversationInsightDate(raw string) (time.Time, errors.Error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.BadInput.New("conversation insights period date is empty")
	}
	if ts, err := time.Parse("2006-01-02", raw); err == nil {
		return ts.UTC(), nil
	}
	return time.Time{}, errors.BadInput.New(fmt.Sprintf("invalid conversation insights period date %q", raw))
}

func parseConversationInsightCountItem(raw json.RawMessage, valueField string) (string, int, errors.Error) {
	var item map[string]json.RawMessage
	if err := json.Unmarshal(raw, &item); err != nil {
		return "", 0, errors.Default.Wrap(errors.Convert(err), "failed to decode conversation insight count item")
	}
	value, count, err := parseConversationInsightFields(item, valueField)
	if err != nil {
		return "", 0, err
	}
	return value, count, nil
}

func parseConversationInsightTimeSeriesItem(raw json.RawMessage, valueField string) (string, *time.Time, int, errors.Error) {
	var item map[string]json.RawMessage
	if err := json.Unmarshal(raw, &item); err != nil {
		return "", nil, 0, errors.Default.Wrap(errors.Convert(err), "failed to decode conversation insight time series item")
	}
	value, count, err := parseConversationInsightFields(item, valueField)
	if err != nil {
		return "", nil, 0, err
	}
	dateRaw, err := decodeJSONString(item["date"])
	if err != nil {
		return "", nil, 0, err
	}
	metricDate, err := parseConversationInsightDate(dateRaw)
	if err != nil {
		return "", nil, 0, err
	}
	return value, &metricDate, count, nil
}

func parseConversationInsightFields(item map[string]json.RawMessage, valueField string) (string, int, errors.Error) {
	valueRaw, ok := item[valueField]
	if !ok {
		return "", 0, errors.BadInput.New(fmt.Sprintf("conversation insight item missing %q field", valueField))
	}
	value, err := decodeJSONString(valueRaw)
	if err != nil {
		return "", 0, err
	}
	countRaw, ok := item["count"]
	if !ok {
		return "", 0, errors.BadInput.New("conversation insight item missing count field")
	}
	var count int
	if err := json.Unmarshal(countRaw, &count); err != nil {
		return "", 0, errors.Default.Wrap(errors.Convert(err), "failed to decode conversation insight count")
	}
	return value, count, nil
}

func decodeJSONString(raw json.RawMessage) (string, errors.Error) {
	if len(raw) == 0 {
		return "", errors.BadInput.New("conversation insight string field is empty")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", errors.Default.Wrap(errors.Convert(err), "failed to decode conversation insight string field")
	}
	return strings.TrimSpace(value), nil
}

func computeConversationInsightRecordId(
	periodStart, periodEnd time.Time,
	insightType, aggregationType, dimensionValue, subcategoryGroup, metricDateKey string,
) string {
	payload := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s",
		periodStart.Format("2006-01-02"),
		periodEnd.Format("2006-01-02"),
		insightType,
		aggregationType,
		dimensionValue,
		subcategoryGroup,
		metricDateKey,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:16])
}
