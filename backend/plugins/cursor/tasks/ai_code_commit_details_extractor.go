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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

type aiCodeCommitDetailResponse struct {
	Commits       []aiCodeCommitDetailCommit       `json:"commits"`
	Conversations []aiCodeCommitDetailConversation `json:"conversations"`
}

type aiCodeCommitDetailCommit struct {
	CommitHash       string                         `json:"commitHash"`
	CommitSource     string                         `json:"commitSource"`
	RangeAnnotations []aiCodeCommitDetailFileBlame  `json:"rangeAnnotations"`
}

type aiCodeCommitDetailFileBlame struct {
	FilePath string                      `json:"filePath"`
	Groups   []aiCodeCommitDetailGroup   `json:"groups"`
}

type aiCodeCommitDetailGroup struct {
	ConversationId *string                   `json:"conversationId"`
	Model          *string                   `json:"model"`
	OperationType  string                    `json:"operationType"`
	Ranges         []aiCodeCommitDetailRange `json:"ranges"`
}

type aiCodeCommitDetailRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type aiCodeCommitDetailConversation struct {
	Id              string   `json:"id"`
	Title           *string  `json:"title"`
	Tldr            *string  `json:"tldr"`
	Overview        *string  `json:"overview"`
	SummaryBullets  []string `json:"summaryBullets"`
}

// ExtractAiCodeCommitDetails parses commit-detail payloads into conversation and range annotation tables.
func ExtractAiCodeCommitDetails(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if data.Connection.KeyTier != models.KeyTierEnterprise {
		taskCtx.GetLogger().Info("Skipping AI code commit detail extraction: key tier is %q, not enterprise", data.Connection.KeyTier)
		return nil
	}
	if !data.Connection.HasAiCodeCommitDetails {
		taskCtx.GetLogger().Info("Skipping AI code commit detail extraction: connection lacks commit-details access")
		return nil
	}

	extractor, err := newCursorStatefulExtractor(&cursorStatefulExtractorArgs[aiCodeCommitDetailResponse]{
		SubtaskCommonArgs: cursorSubtaskCommonArgs(taskCtx, data, rawAiCodeCommitDetailsTable),
		ConnectionId:      data.Options.ConnectionId,
		ScopeId:           data.Options.ScopeId,
		ToolTable:         models.CursorAiCodeRangeAnnotation{}.TableName(),
		Extract: func(record *aiCodeCommitDetailResponse, _ *helper.RawData) ([]any, errors.Error) {
			return flattenAiCodeCommitDetailResponse(data.Options.ConnectionId, data.Options.ScopeId, record)
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func flattenAiCodeCommitDetailResponse(connectionId uint64, scopeId string, record *aiCodeCommitDetailResponse) ([]any, errors.Error) {
	if record == nil {
		return nil, nil
	}

	var rows []any
	for _, conversation := range record.Conversations {
		conversationId := strings.TrimSpace(conversation.Id)
		if conversationId == "" {
			continue
		}
		summaryBullets := ""
		if len(conversation.SummaryBullets) > 0 {
			encoded, jsonErr := json.Marshal(conversation.SummaryBullets)
			if jsonErr != nil {
				return nil, errors.Default.Wrap(errors.Convert(jsonErr), "failed to encode conversation summary bullets")
			}
			summaryBullets = string(encoded)
		}
		rows = append(rows, &models.CursorAiCodeConversation{
			ConnectionId:   connectionId,
			ScopeId:        scopeId,
			ConversationId: conversationId,
			Title:          stringPtrValue(conversation.Title),
			Tldr:           stringPtrValue(conversation.Tldr),
			Overview:       stringPtrValue(conversation.Overview),
			SummaryBullets: summaryBullets,
		})
	}

	for _, commit := range record.Commits {
		commitHash := strings.TrimSpace(commit.CommitHash)
		if commitHash == "" {
			continue
		}
		for _, fileBlame := range commit.RangeAnnotations {
			filePath := strings.TrimSpace(fileBlame.FilePath)
			if filePath == "" {
				continue
			}
			for _, group := range fileBlame.Groups {
				conversationId := ""
				if group.ConversationId != nil {
					conversationId = strings.TrimSpace(*group.ConversationId)
				}
				model := ""
				if group.Model != nil {
					model = strings.TrimSpace(*group.Model)
				}
				for _, lineRange := range group.Ranges {
					if lineRange.End < lineRange.Start {
						continue
					}
					lineCount := lineRange.End - lineRange.Start + 1
					if lineRange.End == 0 && lineRange.Start == 0 {
						lineCount = 0
					}
					operationType := strings.TrimSpace(group.OperationType)
					rows = append(rows, &models.CursorAiCodeRangeAnnotation{
						ConnectionId:   connectionId,
						ScopeId:        scopeId,
						AnnotationId: computeAiCodeRangeAnnotationId(
							commitHash, filePath, conversationId, model, operationType, lineRange.Start, lineRange.End,
						),
						CommitHash:     commitHash,
						FilePath:       filePath,
						ConversationId: conversationId,
						RangeStart:     lineRange.Start,
						RangeEnd:       lineRange.End,
						Model:          model,
						OperationType:  operationType,
						LineCount:      lineCount,
					})
				}
			}
		}
	}

	return rows, nil
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func computeAiCodeRangeAnnotationId(
	commitHash, filePath, conversationId, model, operationType string,
	rangeStart, rangeEnd int,
) string {
	payload := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%d|%d",
		commitHash, filePath, conversationId, model, operationType, rangeStart, rangeEnd,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
