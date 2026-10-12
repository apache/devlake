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

package apiv2models

import (
	"sort"
	"time"

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/plugins/jira/models"
)

type Changelog struct {
	ID      uint64             `json:"id,string"`
	Author  Account            `json:"author"`
	Created common.Iso8601Time `json:"created"`
	Items   []ChangelogItem    `json:"items"`
}

func (c Changelog) ToToolLayer(connectionId, issueId uint64, issueUpdated *time.Time) (*models.JiraIssueChangelogs, *models.JiraAccount) {
	changelog := &models.JiraIssueChangelogs{
		ConnectionId:      connectionId,
		ChangelogId:       c.ID,
		IssueId:           issueId,
		AuthorAccountId:   c.Author.getAccountId(),
		AuthorDisplayName: c.Author.DisplayName,
		AuthorActive:      c.Author.Active,
		Created:           c.Created.ToTime(),
		IssueUpdated:      issueUpdated,
	}
	return changelog, c.Author.ToToolLayer(connectionId)
}

type ChangelogItem struct {
	Field     string `json:"field"`
	Fieldtype string `json:"fieldtype"`
	FieldId   string `json:"fieldId"`

	FromValue  string `json:"from"`
	FromString string `json:"fromString"`

	ToValue  string `json:"to"`
	ToString string `json:"toString"`

	TmpFromAccountId string `json:"tmpFromAccountId,omitempty"`
	TmpToAccountId   string `json:"tmpToAccountId,omitempty"`
}

// IndexedChangelogItem is a changelog item with a stable per-field ordinal.
type IndexedChangelogItem struct {
	Item      ChangelogItem
	ItemIndex uint64
}

// IndexChangelogItems copies items and assigns a canonical per-field ItemIndex.
// Index 0 is the lexicographically smallest item for that field by
// (FromValue, ToValue, FromString, ToString, FieldId), not JSON array order.
func IndexChangelogItems(items []ChangelogItem) []IndexedChangelogItem {
	copied := make([]ChangelogItem, len(items))
	copy(copied, items)
	sort.SliceStable(copied, func(i, j int) bool {
		a, b := copied[i], copied[j]
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		if a.FromValue != b.FromValue {
			return a.FromValue < b.FromValue
		}
		if a.ToValue != b.ToValue {
			return a.ToValue < b.ToValue
		}
		if a.FromString != b.FromString {
			return a.FromString < b.FromString
		}
		if a.ToString != b.ToString {
			return a.ToString < b.ToString
		}
		return a.FieldId < b.FieldId
	})
	indexed := make([]IndexedChangelogItem, 0, len(copied))
	counts := make(map[string]uint64)
	for _, item := range copied {
		idx := counts[item.Field]
		counts[item.Field]++
		indexed = append(indexed, IndexedChangelogItem{Item: item, ItemIndex: idx})
	}
	return indexed
}

func (c ChangelogItem) ToToolLayer(connectionId, changelogId, itemIndex uint64) *models.JiraIssueChangelogItems {
	item := &models.JiraIssueChangelogItems{
		ConnectionId:     connectionId,
		ChangelogId:      changelogId,
		Field:            c.Field,
		ItemIndex:        itemIndex,
		FieldType:        c.Fieldtype,
		FieldId:          c.FieldId,
		FromValue:        c.FromValue,
		FromString:       c.FromString,
		ToValue:          c.ToValue,
		ToString:         c.ToString,
		TmpFromAccountId: c.TmpFromAccountId,
		TmpToAccountId:   c.TmpToAccountId,
	}
	return item
}

func (c ChangelogItem) ExtractUser(connectionId uint64, userFieldMaps map[string]struct{}) []*models.JiraAccount {
	var result []*models.JiraAccount
	_, ok := userFieldMaps[c.Field]
	if c.Field == "assignee" || c.Field == "reporter" || ok {
		if c.FromValue != "" {
			result = append(result, &models.JiraAccount{ConnectionId: connectionId, AccountId: c.FromValue})
		}
		if c.ToValue != "" {
			result = append(result, &models.JiraAccount{ConnectionId: connectionId, AccountId: c.ToValue})
		}
	}
	return result
}
