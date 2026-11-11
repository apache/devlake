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
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
	"gorm.io/datatypes"
)

func commentCreated(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func convertProjectRows(taskCtx plugin.SubTaskContext, table string, sample interface{}, convert func(interface{}) ([]interface{}, errors.Error)) errors.Error {
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, table)
	cursor, err := taskCtx.GetDal().Cursor(
		dal.From(sample),
		dal.Where("connection_id = ? AND project_id = ?", data.Options.ConnectionId, data.Options.ProjectId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()
	converter, err := api.NewDataConverter(api.DataConverterArgs{
		InputRowType:       reflect.TypeOf(sample).Elem(),
		Input:              cursor,
		RawDataSubTaskArgs: *rawArgs,
		Convert:            convert,
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

func ExtractApiWorkItemUpdates(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	if data.Options.ScopeConfig == nil || !data.Options.ScopeConfig.CollectChangelogs {
		return nil
	}
	rawArgs, data := CreateRawDataSubTaskArgs(taskCtx, RawWorkItemUpdateTable)
	extractor, err := api.NewApiExtractor(api.ApiExtractorArgs{
		RawDataSubTaskArgs: *rawArgs,
		Extract: func(row *api.RawData) ([]interface{}, errors.Error) {
			var raw struct {
				ID          int                    `json:"id"`
				Rev         int                    `json:"rev"`
				RevisedBy   map[string]interface{} `json:"revisedBy"`
				RevisedDate interface{}            `json:"revisedDate"`
				Fields      map[string]interface{} `json:"fields"`
				WorkItemId  int                    `json:"workItemId"`
			}
			if err := errors.Convert(json.Unmarshal(row.Data, &raw)); err != nil {
				return nil, err
			}
			workItemID := raw.WorkItemId
			if workItemID == 0 {
				var input workItemRefInput
				_ = json.Unmarshal(row.Input, &input)
				workItemID = input.WorkItemId
			}
			authorID, authorName := identityFromRef(raw.RevisedBy)
			created := updateCreatedDate(raw.RevisedDate, raw.Fields)
			revision := raw.Rev
			if revision == 0 {
				revision = raw.ID
			}
			workItemType := fieldChangeString(raw.Fields, "System.WorkItemType", true)
			results := make([]interface{}, 0)
			for fieldRef, rawChange := range raw.Fields {
				change, _ := rawChange.(map[string]interface{})
				if _, mapped := changelogFieldNames[fieldRef]; !mapped || change == nil {
					continue
				}
				fromValue := formatFieldValue(change["oldValue"])
				toValue := formatFieldValue(change["newValue"])
				if fromValue == "" && toValue == "" {
					continue
				}
				results = append(results, &models.AzuredevopsWorkItemUpdate{
					ConnectionId: data.Options.ConnectionId,
					WorkItemId:   workItemID,
					Revision:     revision,
					FieldRef:     fieldRef,
					ProjectId:    data.Options.ProjectId,
					WorkItemType: workItemType,
					AuthorId:     authorID,
					AuthorName:   authorName,
					OriginalFrom: fromValue,
					OriginalTo:   toValue,
					CreatedDate:  created,
				})
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

func ConvertWorkItemUpdates(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	if data.Options.ScopeConfig == nil || !data.Options.ScopeConfig.CollectChangelogs {
		return nil
	}
	mapper := newWorkItemMapper(taskCtx, data)
	var statusMappings datatypes.JSONMap
	if data.Options.ScopeConfig != nil {
		statusMappings = data.Options.ScopeConfig.StatusMappings
	}
	workItemIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsWorkItem{})
	updateIdGen := didgen.NewDomainIdGenerator(&models.AzuredevopsWorkItemUpdate{})
	return convertProjectRows(taskCtx, models.AzuredevopsWorkItemUpdate{}.TableName(), &models.AzuredevopsWorkItemUpdate{}, func(row interface{}) ([]interface{}, errors.Error) {
		update := row.(*models.AzuredevopsWorkItemUpdate)
		fieldName := changelogFieldNames[update.FieldRef]
		if fieldName == "" || (update.OriginalFrom == "" && update.OriginalTo == "") {
			return nil, nil
		}
		if update.WorkItemType != "" && !shouldConvertWorkItem(update.WorkItemType) {
			return nil, nil
		}
		fromValue, toValue := update.OriginalFrom, update.OriginalTo
		if fieldName == "status" {
			mapper.loadStateCategory(data, update.WorkItemType)
			if fromValue != "" {
				fromValue = resolveWorkItemStatus(update.WorkItemType, fromValue, statusMappings, mapper.categories)
			}
			if toValue != "" {
				toValue = resolveWorkItemStatus(update.WorkItemType, toValue, statusMappings, mapper.categories)
			}
		}
		var created time.Time
		if update.CreatedDate != nil {
			created = *update.CreatedDate
		}
		return []interface{}{&ticket.IssueChangelogs{
			DomainEntity:      domainlayer.NewDomainEntity(updateIdGen.Generate(data.Options.ConnectionId, update.WorkItemId, update.Revision, update.FieldRef)),
			IssueId:           workItemIdGen.Generate(data.Options.ConnectionId, update.WorkItemId),
			AuthorId:          update.AuthorId,
			AuthorName:        update.AuthorName,
			FieldId:           update.FieldRef,
			FieldName:         fieldName,
			OriginalFromValue: update.OriginalFrom,
			OriginalToValue:   update.OriginalTo,
			FromValue:         fromValue,
			ToValue:           toValue,
			CreatedDate:       created,
		}}, nil
	})
}

func formatFieldValue(value interface{}) string {
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, key := range []string{"uniqueName", "displayName", "id"} {
			if text := asString(typed[key]); text != "" {
				return text
			}
		}
		return ""
	default:
		return asString(value)
	}
}

func fieldChangeString(fields map[string]interface{}, fieldRef string, preferNew bool) string {
	change, _ := fields[fieldRef].(map[string]interface{})
	if change == nil {
		return ""
	}
	key := "oldValue"
	if preferNew {
		key = "newValue"
	}
	value := formatFieldValue(change[key])
	if value == "" {
		if preferNew {
			return formatFieldValue(change["oldValue"])
		}
		return formatFieldValue(change["newValue"])
	}
	return value
}

func updateCreatedDate(revised interface{}, fields map[string]interface{}) *time.Time {
	parsed := parseAdoTime(revised)
	if parsed != nil && !isAdoOpenRevisionSentinel(parsed) {
		return parsed
	}
	fallbacks := []struct {
		field string
		key   string
	}{
		{"System.ChangedDate", "newValue"},
		{"System.AuthorizedDate", "newValue"},
		{"System.RevisedDate", "oldValue"},
		{"Microsoft.VSTS.Common.StateChangeDate", "newValue"},
	}
	for _, fallback := range fallbacks {
		change, _ := fields[fallback.field].(map[string]interface{})
		if change == nil {
			continue
		}
		candidate := parseAdoTime(change[fallback.key])
		if candidate != nil && !isAdoOpenRevisionSentinel(candidate) {
			return candidate
		}
	}
	if isAdoOpenRevisionSentinel(parsed) {
		return nil
	}
	return parsed
}
