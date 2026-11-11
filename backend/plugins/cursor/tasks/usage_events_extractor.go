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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

type usageEventRecord struct {
	Timestamp          string           `json:"timestamp"`
	Model              string           `json:"model"`
	Kind               string           `json:"kind"`
	MaxMode            bool             `json:"maxMode"`
	RequestsCosts      float64          `json:"requestsCosts"`
	IsTokenBasedCall   bool             `json:"isTokenBasedCall"`
	TokenUsage         *usageTokenUsage `json:"tokenUsage"`
	UserEmail          string           `json:"userEmail"`
	IsChargeable       bool             `json:"isChargeable"`
	ServiceAccountId   string           `json:"serviceAccountId"`
	ServiceAccountName string           `json:"serviceAccountName"`
	IsHeadless         bool             `json:"isHeadless"`
	ChargedCents       float64          `json:"chargedCents"`
	CursorTokenFee     float64          `json:"cursorTokenFee"`
	HostingType        string           `json:"hostingType"`
	ConversationId     string           `json:"conversationId"`
}

type usageTokenUsage struct {
	InputTokens      int     `json:"inputTokens"`
	OutputTokens     int     `json:"outputTokens"`
	CacheWriteTokens int     `json:"cacheWriteTokens"`
	CacheReadTokens  int     `json:"cacheReadTokens"`
	TotalCents       float64 `json:"totalCents"`
}

// resolveUsageEventActor returns stored userEmail, the key used in computeEventId, and false when the row should be skipped.
func resolveUsageEventActor(userEmail, serviceAccountId string) (storedEmail, eventIdKey string, ok bool) {
	storedEmail = strings.TrimSpace(userEmail)
	serviceAccountId = normalizeNullableString(serviceAccountId)
	if storedEmail == "" && serviceAccountId == "" {
		return "", "", false
	}
	eventIdKey = storedEmail
	if eventIdKey == "" {
		eventIdKey = "sa:" + serviceAccountId
	}
	return storedEmail, eventIdKey, true
}

// ExtractUsageEvents parses raw usage event records into tool-layer tables.
func ExtractUsageEvents(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	logger := taskCtx.GetLogger()

	extractor, err := newCursorStatefulExtractor(&cursorStatefulExtractorArgs[usageEventRecord]{
		SubtaskCommonArgs: cursorSubtaskCommonArgs(taskCtx, data, rawUsageEventsTable),
		ConnectionId:      data.Options.ConnectionId,
		ScopeId:           data.Options.ScopeId,
		ToolTable:         models.CursorUsageEvent{}.TableName(),
		Extract: func(record *usageEventRecord, row *helper.RawData) ([]any, errors.Error) {
			eventTime, err := parseEventTimestampMs(record.Timestamp)
			if err != nil {
				return nil, err
			}

			userEmail, eventIdKey, ok := resolveUsageEventActor(record.UserEmail, record.ServiceAccountId)
			if !ok {
				rawID := uint64(0)
				if row != nil {
					rawID = row.ID
				}
				logger.Warn(nil, "skipping usage event raw row id=%d: missing userEmail and serviceAccountId", rawID)
				return nil, nil
			}

			serviceAccountId := normalizeNullableString(record.ServiceAccountId)

			event := &models.CursorUsageEvent{
				ConnectionId:       data.Options.ConnectionId,
				ScopeId:            data.Options.ScopeId,
				EventId:            computeEventId(record.Timestamp, eventIdKey, record.ConversationId, record.Model, record.ChargedCents, record.RequestsCosts),
				EventTime:          eventTime,
				UserEmail:          userEmail,
				Model:              strings.TrimSpace(record.Model),
				Kind:               strings.TrimSpace(record.Kind),
				ConversationId:     strings.TrimSpace(record.ConversationId),
				ChargedCents:       record.ChargedCents,
				RequestsCosts:      record.RequestsCosts,
				IsTokenBasedCall:   record.IsTokenBasedCall,
				IsChargeable:       record.IsChargeable,
				MaxMode:            record.MaxMode,
				IsHeadless:         record.IsHeadless,
				CursorTokenFee:     record.CursorTokenFee,
				HostingType:        strings.TrimSpace(record.HostingType),
				ServiceAccountId:   serviceAccountId,
				ServiceAccountName: normalizeNullableString(record.ServiceAccountName),
			}
			if record.TokenUsage != nil {
				event.InputTokens = record.TokenUsage.InputTokens
				event.OutputTokens = record.TokenUsage.OutputTokens
				event.CacheReadTokens = record.TokenUsage.CacheReadTokens
				event.CacheWriteTokens = record.TokenUsage.CacheWriteTokens
				event.TotalCents = record.TokenUsage.TotalCents
			}
			return []any{event}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
