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
	"fmt"
	"reflect"
	"time"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/datadog/models"
)

// defaultIncidentPrefix is what Datadog names incidents of a type that
// declares no prefix of its own.
const defaultIncidentPrefix = "IR"

var _ plugin.SubTaskEntryPoint = ConvertIncidents

var ConvertIncidentsMeta = plugin.SubTaskMeta{
	Name:             "convertIncidents",
	EntryPoint:       ConvertIncidents,
	EnabledByDefault: true,
	Description:      "Convert Datadog incidents into domain-layer ticket issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*DatadogTaskData)
	logger := taskCtx.GetLogger()

	cursor, err := db.Cursor(
		dal.From(&models.Incident{}),
		dal.Where("connection_id = ? AND incident_type_id = ?", data.Options.ConnectionId, data.Options.IncidentTypeId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	prefix := defaultIncidentPrefix
	incidentType := &models.IncidentType{}
	if err := db.First(incidentType, dal.Where(
		"connection_id = ? AND id = ?", data.Options.ConnectionId, data.Options.IncidentTypeId,
	)); err == nil && incidentType.Prefix != "" {
		prefix = incidentType.Prefix
	}

	idGen := didgen.NewDomainIdGenerator(&models.Incident{})
	incidentTypeIdGen := didgen.NewDomainIdGenerator(&models.IncidentType{})
	boardId := incidentTypeIdGen.Generate(data.Options.ConnectionId, data.Options.IncidentTypeId)

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENTS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.Incident{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			incident := inputRow.(*models.Incident)

			status, known := mapState(incident.State)
			if !known {
				logger.Warn(nil, "unknown datadog incident state: %s", incident.State)
			}

			leadTime := restoreMinutes(incident)
			domainIssueId := idGen.Generate(data.Options.ConnectionId, incident.Id)

			domainIssue := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: domainIssueId,
				},
				Url:             incident.Url,
				IssueKey:        fmt.Sprintf("%s-%d", prefix, incident.PublicId),
				Title:           incident.Title,
				Type:            ticket.INCIDENT,
				Status:          status,
				OriginalStatus:  incident.State,
				CreatedDate:     &incident.CreatedDate,
				UpdatedDate:     incident.ModifiedDate,
				ResolutionDate:  incident.ResolvedDate,
				LeadTimeMinutes: leadTime,
				Severity:        incident.Severity,
				Component:       incident.Component,
			}

			return []interface{}{
				domainIssue,
				&ticket.BoardIssue{
					BoardId: boardId,
					IssueId: domainIssueId,
				},
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}

// Datadog's incident states are a small fixed set. An unknown value falls
// through to IN_PROGRESS with a warning rather than failing the pipeline.
func mapState(state string) (mapped string, known bool) {
	switch state {
	case "active", "stable":
		return ticket.IN_PROGRESS, true
	case "resolved", "completed":
		return ticket.DONE, true
	default:
		return ticket.IN_PROGRESS, false
	}
}

// restoreMinutes prefers Datadog's own time_to_repair: it measures when
// service was restored, whereas created-to-resolved also counts the
// post-incident paperwork and so overstates MTTR.
func restoreMinutes(incident *models.Incident) *uint {
	if incident.TimeToRepair != nil && *incident.TimeToRepair >= 0 {
		minutes := uint(*incident.TimeToRepair / 60)
		return &minutes
	}
	if incident.ResolvedDate == nil {
		return nil
	}
	// A retrospectively declared incident can resolve before it was
	// created. A naive uint cast of a negative duration wraps to garbage
	// and silently corrupts MTTR, so drop the value instead.
	if incident.ResolvedDate.Before(incident.CreatedDate) {
		return nil
	}
	minutes := uint(incident.ResolvedDate.Sub(incident.CreatedDate) / time.Minute)
	return &minutes
}
