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

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/opsgenie/models"
)

var ConvertIncidentsMeta = plugin.SubTaskMeta{
	Name:             "convertIncidents",
	EntryPoint:       ConvertIncidents,
	EnabledByDefault: true,
	Description:      "Convert Incidents into domain layer table issues",
	Dependencies: []*plugin.SubTaskMeta{
		&ExtractUsersMeta,
		&ExtractTeamsMeta,
		&CollectUsersMeta,
		&CollectTeamsMeta,
		&ConvertUsersMeta,
		&ConvertTeamsMeta,
	},
	DependencyTables: []string{
		models.User{}.TableName(), // cursor
		models.Team{}.TableName(), // cursor
	},
	DomainTypes: []string{plugin.DOMAIN_TYPE_TICKET},
}

type (
	IncidentWithResponder struct {
		common.NoPKModel
		models.Incident
		*models.Responder
	}
)

func ConvertIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*OpsgenieTaskData)

	// Correlates responder id with it's user or team name
	cursor, err := db.Cursor(
		dal.Select("incidents.*, responders.id, responders.type, users.full_name, teams.name"),
		dal.From("_tool_opsgenie_incidents AS incidents"),
		dal.Join(`LEFT JOIN _tool_opsgenie_assignments AS assignments ON assignments.incident_id = incidents.id`),
		dal.Join(`LEFT JOIN _tool_opsgenie_responders AS responders ON assignments.responder_id = responders.id`),
		dal.Join(`LEFT JOIN _tool_opsgenie_users AS users ON users.id = responders.id`),
		dal.Join(`LEFT JOIN _tool_opsgenie_teams AS teams ON teams.id = responders.id`),
		dal.Where("incidents.connection_id = ? AND incidents.service_id = ?", data.Options.ConnectionId, data.Options.ServiceId),
	)

	if err != nil {
		return err
	}

	defer cursor.Close()

	idGen := didgen.NewDomainIdGenerator(&models.Incident{})
	serviceIdGen := didgen.NewDomainIdGenerator(&models.Service{})
	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		InputRowType: reflect.TypeOf(IncidentWithResponder{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			combined := inputRow.(*IncidentWithResponder)
			incident := combined.Incident
			status := getStatus(&incident)
			leadTime, resolutionDate := getTimes(&incident)
			domainIssue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: idGen.Generate(data.Options.ConnectionId, incident.Id),
				},
				Url:             incident.Url,
				IssueKey:        incident.Id,
				Title:           incident.Message,
				Description:     incident.Description,
				Type:            ticket.INCIDENT,
				Status:          status,
				OriginalStatus:  string(incident.Status),
				ResolutionDate:  resolutionDate,
				CreatedDate:     &incident.CreatedDate,
				UpdatedDate:     &incident.UpdatedDate,
				LeadTimeMinutes: leadTime,
				Priority:        string(incident.Priority),
			}
			var result []interface{}
			if combined.Responder != nil {
				var assigneeName string
				if combined.Responder.Type == "user" {
					assigneeName = combined.Responder.FullName
				}
				if combined.Responder.Type == "team" {
					assigneeName = combined.Responder.Name
				}
				issueAssignee := &ticket.IssueAssignee{
					IssueId:      domainIssue.Id,
					AssigneeId:   combined.Responder.Id,
					AssigneeName: resolve(&assigneeName),
				}
				domainIssue.AssigneeName = issueAssignee.AssigneeName
				domainIssue.AssigneeId = issueAssignee.AssigneeId
				result = append(result, issueAssignee)
			}
			result = append(result, domainIssue)
			boardIssue := &ticket.BoardIssue{
				BoardId: serviceIdGen.Generate(data.Options.ConnectionId, data.Options.ServiceId),
				IssueId: domainIssue.Id,
			}
			result = append(result, boardIssue)
			return result, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

func getStatus(incident *models.Incident) string {
	if incident.Status == models.IncidentStatusOpen {
		return ticket.IN_PROGRESS
	}
	if incident.Status == models.IncidentStatusResolved || incident.Status == models.IncidentStatusClosed {
		return ticket.DONE
	}
	panic("unknown incident status encountered")
}

func getTimes(incident *models.Incident) (*uint, *time.Time) {
	var leadTime *uint
	var resolutionDate *time.Time
	if incident.Status == models.IncidentStatusResolved || incident.Status == models.IncidentStatusClosed {
		resolutionDate = &incident.UpdatedDate
		temp := uint(resolutionDate.Sub(incident.CreatedDate).Minutes())
		leadTime = &temp
	}
	return leadTime, resolutionDate
}
