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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/cursor/models"
)

type aiCodeChangeRecord struct {
	ChangeId        string          `json:"changeId"`
	UserId          string          `json:"userId"`
	UserEmail       string          `json:"userEmail"`
	Source          string          `json:"source"`
	Model           string          `json:"model"`
	TotalLinesAdded   int           `json:"totalLinesAdded"`
	TotalLinesDeleted int           `json:"totalLinesDeleted"`
	CreatedAt       string          `json:"createdAt"`
	Metadata        json.RawMessage `json:"metadata"`
}

// ExtractAiCodeChanges parses raw AI code change records into tool-layer tables.
func ExtractAiCodeChanges(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}
	if data.Connection.KeyTier != models.KeyTierEnterprise {
		taskCtx.GetLogger().Info("Skipping AI code change extraction: key tier is %q, not enterprise", data.Connection.KeyTier)
		return nil
	}

	extractor, err := newCursorStatefulExtractor(&cursorStatefulExtractorArgs[aiCodeChangeRecord]{
		SubtaskCommonArgs: cursorSubtaskCommonArgs(taskCtx, data, rawAiCodeChangesTable),
		ConnectionId:      data.Options.ConnectionId,
		ScopeId:           data.Options.ScopeId,
		ToolTable:         models.CursorAiCodeChange{}.TableName(),
		Extract: func(record *aiCodeChangeRecord, row *helper.RawData) ([]any, errors.Error) {
			changeId := strings.TrimSpace(record.ChangeId)
			if changeId == "" {
				return nil, nil
			}

			var metadataStr string
			if len(record.Metadata) > 0 && string(record.Metadata) != "null" {
				metadataStr = string(record.Metadata)
			}

			change := &models.CursorAiCodeChange{
				ConnectionId:      data.Options.ConnectionId,
				ScopeId:           data.Options.ScopeId,
				ChangeId:          changeId,
				UserId:            strings.TrimSpace(record.UserId),
				UserEmail:         strings.TrimSpace(record.UserEmail),
				Source:            strings.TrimSpace(record.Source),
				Model:             strings.TrimSpace(record.Model),
				TotalLinesAdded:   record.TotalLinesAdded,
				TotalLinesDeleted: record.TotalLinesDeleted,
				Metadata:          metadataStr,
			}
			if ts := parseOptionalISOTime(record.CreatedAt); ts != nil {
				change.IngestedAt = ts
			}
			return []any{change}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
