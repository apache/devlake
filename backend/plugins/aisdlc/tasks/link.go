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
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aisdlc/models"
	"github.com/apache/devlake/plugins/aisdlc/tasks/parse"
)

var LinkWorkItemsMeta = plugin.SubTaskMeta{
	Name:             "linkWorkItems",
	EntryPoint:       LinkWorkItems,
	EnabledByDefault: true,
	Description:      "Match work-item keys to existing issues when the match is unique",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractObservationsMeta},
}

type issueRow struct {
	ID       string `gorm:"column:id"`
	IssueKey string `gorm:"column:issue_key"`
}

// LinkWorkItems sets domain_issue_id only when issues.issue_key matches one row.
// It does not insert issues.
func LinkWorkItems(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*AisdlcTaskData)
	var items []models.AisdlcWorkItem
	if err := db.All(&items, dal.Where("connection_id = ?", data.Connection.ID)); err != nil {
		return err
	}
	for i := range items {
		item := &items[i]
		if item.SourceSystem != parse.JiraSystem || item.ItemKey == "" {
			continue
		}
		var rows []issueRow
		if err := db.All(&rows, dal.From("issues"), dal.Where("issue_key = ?", item.ItemKey)); err != nil {
			return err
		}
		ids := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		draft := parse.WorkItem{ID: item.Id, MatchStatus: item.MatchStatus, DomainIssueID: item.DomainIssueId}
		parse.ApplyIssueMatch(&draft, ids)
		item.MatchStatus = draft.MatchStatus
		item.DomainIssueId = draft.DomainIssueID
		if err := db.Update(item); err != nil {
			return err
		}
	}
	return nil
}
