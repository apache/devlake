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
	"strconv"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

var ExtractIssueChangelogsMeta = plugin.SubTaskMeta{
	Name:             "Extract Issue Changelogs",
	EntryPoint:       ExtractIssueChangelogs,
	EnabledByDefault: true,
	Description:      "Extract raw activity items into _tool_youtrack_issue_changelogs (multi-value added/removed JSON-encoded) and _tool_youtrack_accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{RAW_ISSUE_CHANGELOGS_TABLE},
	ProductTables: []string{
		models.YoutrackIssueChangelog{}.TableName(),
		models.YoutrackAccount{}.TableName(),
	},
}

var _ plugin.SubTaskEntryPoint = ExtractIssueChangelogs

// apiActivityItem decodes the changelog collector's fields param
// (changelogFields). Added/Removed stay raw because their shape depends on
// the category: arrays of bundle elements (state/enum), arrays of users
// (assignee), arrays of tags, plain strings (summary), an epoch-ms number
// (resolved), or null (created).
type apiActivityItem struct {
	Id        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Category  *struct {
		Id string `json:"id"`
	} `json:"category"`
	Author       *apiUser `json:"author"`
	TargetMember *string  `json:"targetMember"`
	Field        *struct {
		Name        string `json:"name"`
		CustomField *struct {
			Name string `json:"name"`
		} `json:"customField"`
	} `json:"field"`
	Added   json.RawMessage `json:"added"`
	Removed json.RawMessage `json:"removed"`
}

func ExtractIssueChangelogs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*YoutrackTaskData)
	connectionId := data.Options.ConnectionId
	projectId := data.Options.ProjectId
	// activity authors (and the users inside assignee changes) derive into
	// accounts only when CROSS is collected; the guest
	// account is skipped at conversion
	deriveAccounts := utils.StringsContains(data.ScopeConfig.Entities, plugin.DOMAIN_TYPE_CROSS)

	extractor, err := helper.NewStatefulApiExtractor(&helper.StatefulApiExtractorArgs[apiActivityItem]{
		SubtaskCommonArgs: &helper.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_ISSUE_CHANGELOGS_TABLE,
			Params: models.YoutrackApiParams{
				ConnectionId: connectionId,
				ProjectId:    projectId,
			},
			// every stateful extractor receives the whole scope config, so a
			// config change re-extracts from the raw layer without new
			// YouTrack API calls
			SubtaskConfig: data.ScopeConfig,
		},
		Extract: func(item *apiActivityItem, row *helper.RawData) ([]interface{}, errors.Error) {
			input := &youtrackChangelogInput{}
			if err := errors.Convert(json.Unmarshal(row.Input, input)); err != nil {
				return nil, err
			}
			fromValue, originalFromValue, err := encodeActivityValues(item.Removed)
			if err != nil {
				return nil, err
			}
			toValue, originalToValue, err := encodeActivityValues(item.Added)
			if err != nil {
				return nil, err
			}
			changelog := &models.YoutrackIssueChangelog{
				ConnectionId:      connectionId,
				Id:                item.Id,
				IssueId:           input.Id,
				FieldName:         activityFieldName(item),
				FromValue:         fromValue,
				ToValue:           toValue,
				OriginalFromValue: originalFromValue,
				OriginalToValue:   originalToValue,
				Created:           time.UnixMilli(item.Timestamp).UTC(),
			}
			if item.Category != nil {
				changelog.Category = item.Category.Id
			}
			if item.Author != nil {
				changelog.AuthorId = item.Author.Id
				changelog.AuthorName = item.Author.FullName
			}
			results := []interface{}{changelog}
			if deriveAccounts {
				if account := deriveAccount(connectionId, item.Author); account != nil {
					results = append(results, account)
				}
				for _, user := range activityItemUsers(item.Added) {
					if account := deriveAccount(connectionId, user); account != nil {
						results = append(results, account)
					}
				}
				for _, user := range activityItemUsers(item.Removed) {
					if account := deriveAccount(connectionId, user); account != nil {
						results = append(results, account)
					}
				}
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}

// activityFieldName resolves the changed field's stable name: the custom
// field's own name for custom-field changes (what the scope config's slots
// name); the target member for predefined targets (tags, summary,
// resolved). field.name is unusable for predefined targets — YouTrack
// localizes it ("тег", "заголовок", "создана").
func activityFieldName(item *apiActivityItem) string {
	if item.Field != nil && item.Field.CustomField != nil && item.Field.CustomField.Name != "" {
		return item.Field.CustomField.Name
	}
	if item.TargetMember != nil {
		return *item.TargetMember
	}
	return ""
}

// encodeActivityValues converts one activity item's added/removed payload
// into the tool layer's (value-ids, display-strings) column pair: scalar
// when the change is single-value, a JSON array string when multi-value.
// Display strings prefer fullName (users), then name (bundle
// elements, tags), then login, then text.
func encodeActivityValues(raw json.RawMessage) (string, string, errors.Error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", "", nil
	}
	var payload interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", errors.Convert(err)
	}
	if list, ok := payload.([]interface{}); ok {
		ids := make([]string, 0, len(list))
		displays := make([]string, 0, len(list))
		for _, element := range list {
			id, display := activityValueElement(element)
			ids = append(ids, id)
			displays = append(displays, display)
		}
		values, err := marshalActivityValues(ids)
		if err != nil {
			return "", "", err
		}
		displayValues, err := marshalActivityValues(displays)
		if err != nil {
			return "", "", err
		}
		return values, displayValues, nil
	}
	scalar := activityScalarString(payload)
	return scalar, scalar, nil
}

// activityValueElement maps one added/removed element to its (id, display)
// pair. Elements are bundle elements, users or tags (objects) — or plain
// scalars for simple-value items.
func activityValueElement(element interface{}) (string, string) {
	obj, ok := element.(map[string]interface{})
	if !ok {
		scalar := activityScalarString(element)
		return scalar, scalar
	}
	id := activityScalarString(obj["id"])
	for _, key := range []string{"fullName", "name", "login", "text"} {
		if display, ok := obj[key].(string); ok && display != "" {
			return id, display
		}
	}
	return id, id
}

// activityScalarString renders a scalar payload (string, number) as a
// string; anything else as empty.
func activityScalarString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// marshalActivityValues is the scalar/JSON-array encoding shared by the
// value-ids and display-strings columns: zero values -> "", one -> scalar,
// more -> a JSON array string.
func marshalActivityValues(values []string) (string, errors.Error) {
	switch len(values) {
	case 0:
		return "", nil
	case 1:
		return values[0], nil
	}
	data, err := errors.Convert01(json.Marshal(values))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// activityItemUsers extracts the user objects from an added/removed payload
// for account derivation. Only arrays of $type=User elements qualify —
// bundle elements, tags and scalar payloads yield nothing.
func activityItemUsers(raw json.RawMessage) []*apiUser {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil {
		return nil
	}
	var users []*apiUser
	for _, element := range elements {
		user := &apiUser{}
		if err := json.Unmarshal(element, user); err != nil {
			continue
		}
		if user.Type == "User" && user.Id != "" {
			users = append(users, user)
		}
	}
	return users
}
