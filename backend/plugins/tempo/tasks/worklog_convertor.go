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
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/tempo/models"
)

var ConvertWorklogsMeta = plugin.SubTaskMeta{
	Name:             "convert_worklogs",
	EntryPoint:       ConvertWorklogs,
	EnabledByDefault: true,
	Description:      "Convert worklogs to domain layer",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertWorklogs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TempoTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	connectionId := data.Options.ConnectionId

	logger.Info("converting Tempo worklogs to domain layer")

	clauses := []dal.Clause{
		dal.Select("*"),
		dal.From("_tool_tempo_worklogs"),
		dal.Where("connection_id = ?", connectionId),
	}
	if data.Options.TeamId != 0 {
		clauses = append(clauses, dal.Where("team_id = ?", data.Options.TeamId))
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query Tempo worklogs")
	}
	defer cursor.Close()

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: models.TempoApiParams{
				ConnectionId: connectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_WORKLOG_TABLE,
		},
		InputRowType: reflect.TypeOf(models.TempoWorklog{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			return []interface{}{convertWorklog(connectionId, inputRow.(*models.TempoWorklog))}, nil
		},
	})

	if err != nil {
		return errors.Default.Wrap(err, "failed to create data converter")
	}

	return converter.Execute()
}

// convertWorklog maps a Tempo worklog to the domain layer.
func convertWorklog(connectionId uint64, tempoWorklog *models.TempoWorklog) *ticket.IssueWorklog {
	// Must match the domain ids the jira plugin generates with didgen
	// for JiraIssue / JiraAccount (plugins may not import each other),
	// so worklogs join issues and accounts. Like before, this assumes
	// the Tempo connection id equals the Jira connection id.
	domainIssueId := fmt.Sprintf("jira:JiraIssue:%d:%d", connectionId, tempoWorklog.IssueId)
	authorId := ""
	if tempoWorklog.AuthorAccountId != "" {
		authorId = fmt.Sprintf("jira:JiraAccount:%d:%s", connectionId, tempoWorklog.AuthorAccountId)
	}

	// Tempo worklog ids are only unique within one Tempo instance, so the id
	// carries the connection id like the tool layer primary key (and like
	// didgen would generate it): otherwise two connections that share a
	// worklog id overwrite each other's issue_worklogs row.
	domainWorklogId := fmt.Sprintf("tempo:TempoWorklog:%d:%d", connectionId, tempoWorklog.TempoWorklogId)
	timeSpentMinutes := tempoWorklog.TimeSpentSeconds / 60

	var startedDate *time.Time
	if tempoWorklog.StartDate != "" && tempoWorklog.StartTime != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", tempoWorklog.StartDate+" "+tempoWorklog.StartTime); err == nil {
			startedDate = &t
		}
	}

	var loggedDate *time.Time
	if tempoWorklog.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, tempoWorklog.CreatedAt); err == nil {
			loggedDate = &t
		}
	}

	return &ticket.IssueWorklog{
		DomainEntity: domainlayer.DomainEntity{
			Id: domainWorklogId,
		},
		IssueId:          domainIssueId,
		AuthorId:         authorId,
		TimeSpentMinutes: timeSpentMinutes,
		StartedDate:      startedDate,
		LoggedDate:       loggedDate,
		Comment:          tempoWorklog.Description,
	}
}
