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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

var ConvertSprintIssuesMeta = plugin.SubTaskMeta{
	Name:             "Convert Sprint Issues",
	EntryPoint:       ConvertSprintIssues,
	EnabledByDefault: true,
	Description:      "Link issues to their cycle (sprint) in the domain layer table sprint_issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{models.LinearIssue{}.TableName(), RAW_ISSUES_TABLE},
	ProductTables:    []string{ticket.SprintIssue{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertSprintIssues

func ConvertSprintIssues(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)
	connectionId := data.Options.ConnectionId

	issueIdGen := didgen.NewDomainIdGenerator(&models.LinearIssue{})
	cycleIdGen := didgen.NewDomainIdGenerator(&models.LinearCycle{})

	// Sprint membership is derived from each issue's cycle_id. Clear this team's
	// existing sprint_issues up front so issues that have since left their cycle
	// leave no stale rows: the batch divider only deletes outdated records when
	// it produces at least one row of the type, which misses the case where
	// every issue has been removed from its cycle.
	var teamIssues []models.LinearIssue
	if err := db.All(&teamIssues,
		dal.Select("id"),
		dal.Where("connection_id = ? AND team_id = ?", connectionId, data.Options.TeamId),
	); err != nil {
		return err
	}
	if len(teamIssues) > 0 {
		issueIds := make([]string, len(teamIssues))
		for i, issue := range teamIssues {
			issueIds[i] = issueIdGen.Generate(connectionId, issue.Id)
		}
		if err := db.Delete(&ticket.SprintIssue{}, dal.Where("issue_id IN ?", issueIds)); err != nil {
			return err
		}
	}

	cursor, err := db.Cursor(
		dal.From(&models.LinearIssue{}),
		dal.Where("connection_id = ? AND team_id = ? AND cycle_id != ''", connectionId, data.Options.TeamId),
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
			Table: RAW_ISSUES_TABLE,
		},
		InputRowType: reflect.TypeOf(models.LinearIssue{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			issue := inputRow.(*models.LinearIssue)
			sprintIssue := &ticket.SprintIssue{
				SprintId: cycleIdGen.Generate(connectionId, issue.CycleId),
				IssueId:  issueIdGen.Generate(connectionId, issue.Id),
			}
			return []interface{}{sprintIssue}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
