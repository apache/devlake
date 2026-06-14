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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/gh-copilot/models"
)

// ExtractSeats parses raw seat assignment data into the GhCopilotSeat tool-layer model.
func ExtractSeats(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*GhCopilotTaskData)
	if !ok {
		return errors.Default.New("task data is not GhCopilotTaskData")
	}
	connection := data.Connection
	connection.Normalize()

	params := copilotRawParams{
		ConnectionId: data.Options.ConnectionId,
		ScopeId:      data.Options.ScopeId,
		Organization: connection.Organization,
		Endpoint:     connection.Endpoint,
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Table:   rawCopilotSeatsTable,
			Options: params,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			seat := &copilotSeatResponse{}
			if err := errors.Convert(json.Unmarshal(row.Data, seat)); err != nil {
				return nil, err
			}

			createdAt, parseErr := time.Parse(time.RFC3339, seat.CreatedAt)
			if parseErr != nil {
				return nil, errors.BadInput.Wrap(parseErr, "invalid seat created_at")
			}
			updatedAt, parseErr := time.Parse(time.RFC3339, seat.UpdatedAt)
			if parseErr != nil {
				return nil, errors.BadInput.Wrap(parseErr, "invalid seat updated_at")
			}

			parseOptional := func(v *string) (*time.Time, errors.Error) {
				if v == nil || *v == "" {
					return nil, nil
				}
				if t, parseErr := time.Parse(time.RFC3339, *v); parseErr == nil {
					return &t, nil
				}
				t, parseErr := time.Parse("2006-01-02", *v)
				if parseErr != nil {
					return nil, errors.BadInput.Wrap(parseErr, "invalid timestamp")
				}
				return &t, nil
			}

			lastAuth, err := parseOptional(seat.LastAuthenticatedAt)
			if err != nil {
				return nil, err
			}
			lastAct, err := parseOptional(seat.LastActivityAt)
			if err != nil {
				return nil, err
			}
			pendingCancel, err := parseOptional(seat.PendingCancellationDate)
			if err != nil {
				return nil, err
			}

			toolSeat := &models.GhCopilotSeat{
				ConnectionId:            data.Options.ConnectionId,
				Organization:            connection.Organization,
				UserLogin:               seat.Assignee.Login,
				UserId:                  seat.Assignee.Id,
				UserName:                seat.Assignee.Name,
				UserEmail:               seat.Assignee.Email,
				PlanType:                seat.PlanType,
				CreatedAt:               createdAt,
				LastActivityAt:          lastAct,
				LastActivityEditor:      seat.LastActivityEditor,
				LastAuthenticatedAt:     lastAuth,
				PendingCancellationDate: pendingCancel,
				UpdatedAt:               updatedAt,
			}
			if seat.AssigningTeam != nil {
				toolSeat.AssigningTeamId = seat.AssigningTeam.Id
				toolSeat.AssigningTeamName = seat.AssigningTeam.Name
				toolSeat.AssigningTeamSlug = seat.AssigningTeam.Slug
			}

			return []interface{}{toolSeat}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
