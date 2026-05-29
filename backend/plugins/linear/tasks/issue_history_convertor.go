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
	"reflect"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

var ConvertIssueHistoryMeta = plugin.SubTaskMeta{
	Name:             "Convert Issue History",
	EntryPoint:       ConvertIssueHistory,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_linear_issue_history into domain layer table issue_changelogs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.LinearIssueHistory{}.TableName(), models.LinearIssue{}.TableName(), RAW_ISSUE_HISTORY_TABLE},
	ProductTables:    []string{ticket.IssueChangelogs{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertIssueHistory

func ConvertIssueHistory(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)
	connectionId := data.Options.ConnectionId

	issueIdGen := didgen.NewDomainIdGenerator(&models.LinearIssue{})
	historyIdGen := didgen.NewDomainIdGenerator(&models.LinearIssueHistory{})
	accountIdGen := didgen.NewDomainIdGenerator(&models.LinearAccount{})

	cursor, err := db.Cursor(
		dal.Select("h.*"),
		dal.From("_tool_linear_issue_history h"),
		dal.Join("LEFT JOIN _tool_linear_issues i ON (i.connection_id = h.connection_id AND i.id = h.issue_id)"),
		dal.Where("h.connection_id = ? AND i.team_id = ?", connectionId, data.Options.TeamId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: connectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ISSUE_HISTORY_TABLE,
		},
		InputRowType: reflect.TypeOf(models.LinearIssueHistory{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			event := inputRow.(*models.LinearIssueHistory)
			changelog := &ticket.IssueChangelogs{
				DomainEntity:      domainlayer.DomainEntity{Id: historyIdGen.Generate(connectionId, event.Id)},
				IssueId:           issueIdGen.Generate(connectionId, event.IssueId),
				FieldId:           "state",
				FieldName:         "status",
				OriginalFromValue: event.FromStateName,
				OriginalToValue:   event.ToStateName,
				CreatedDate:       event.CreatedAt,
			}
			if event.FromStateType != "" {
				changelog.FromValue = StatusFromStateType(event.FromStateType)
			}
			if event.ToStateType != "" {
				changelog.ToValue = StatusFromStateType(event.ToStateType)
			}
			if event.ActorId != "" {
				changelog.AuthorId = accountIdGen.Generate(connectionId, event.ActorId)
			}
			return []interface{}{changelog}, nil
		},
	})
	if err != nil {
		return err
	}
	if err := converter.Execute(); err != nil {
		return err
	}

	return deriveLeadTimeFromHistory(db, connectionId, data.Options.TeamId, issueIdGen)
}

// deriveLeadTimeFromHistory refines each issue's lead time from its recorded
// state transitions: the span from the issue's first transition into an
// in-progress state to its first transition into a done state thereafter (the
// active cycle time). This is the value that genuinely requires history and is
// more accurate than the coarse createdAt -> resolutionDate fallback set by
// ConvertIssues, so it overrides that fallback when the transitions exist.
// Issues whose history lacks an in-progress -> done sequence keep the fallback.
func deriveLeadTimeFromHistory(db dal.Dal, connectionId uint64, teamId string, issueIdGen *didgen.DomainIdGenerator) errors.Error {
	var events []models.LinearIssueHistory
	if err := db.All(&events,
		dal.Select("h.*"),
		dal.From("_tool_linear_issue_history h"),
		dal.Join("LEFT JOIN _tool_linear_issues i ON (i.connection_id = h.connection_id AND i.id = h.issue_id)"),
		dal.Where("h.connection_id = ? AND i.team_id = ?", connectionId, teamId),
		dal.Orderby("h.issue_id, h.created_at"),
	); err != nil {
		return err
	}

	type leadWindow struct {
		startedAt *time.Time
		doneAt    *time.Time
	}
	windows := map[string]*leadWindow{}
	for i := range events {
		event := events[i]
		window := windows[event.IssueId]
		if window == nil {
			window = &leadWindow{}
			windows[event.IssueId] = window
		}
		switch StatusFromStateType(event.ToStateType) {
		case ticket.IN_PROGRESS:
			if window.startedAt == nil {
				createdAt := event.CreatedAt
				window.startedAt = &createdAt
			}
		case ticket.DONE:
			if window.startedAt != nil && window.doneAt == nil {
				createdAt := event.CreatedAt
				window.doneAt = &createdAt
			}
		}
	}

	for issueId, window := range windows {
		if window.startedAt == nil || window.doneAt == nil || !window.doneAt.After(*window.startedAt) {
			continue
		}
		minutes := uint(window.doneAt.Sub(*window.startedAt).Minutes())
		if err := db.UpdateColumn(
			&ticket.Issue{}, "lead_time_minutes", minutes,
			dal.Where("id = ?", issueIdGen.Generate(connectionId, issueId)),
		); err != nil {
			return err
		}
	}
	return nil
}
