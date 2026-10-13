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
	goerrors "errors"
	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"reflect"
)

var IssuesToIncidentsMeta = plugin.SubTaskMeta{
	Name:             "ConvertIssuesToIncidents",
	EntryPoint:       ConvertIssuesToIncidents,
	EnabledByDefault: true,
	Description:      "Connect issue to incident",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

type issueWithBoardId struct {
	ticket.Issue
	BoardId string
}

// inScopeIncidentIssueClauses builds the query that selects the INCIDENT issues
// belonging to a single DevLake project's slice of a (possibly shared) Jira
// board.
//
// When several projects point at the same board, each project's Extra JQL
// records its slice on board_issues.project_name at collection time. This query
// therefore filters on BOTH the board-level project_mapping (pm.project_name)
// AND the issue-level slice (bi.project_name), so a project only turns its own
// issues into incidents. The empty/NULL bi.project_name fallback preserves
// pre-feature (single-project) data that was collected before project_name
// existed on board_issues.
func inScopeIncidentIssueClauses(db dal.Dal, projectName string) []dal.Clause {
	return []dal.Clause{
		dal.Select("i.*, bi.board_id as board_id"),
		dal.From(`issues i`),
		dal.Join(`join board_issues bi on bi.issue_id = i.id`),
		dal.Join("join project_mapping pm on pm.row_id = bi.board_id and " + quoteTableColumn(db, "pm") + " = 'boards'"),
		dal.Where(
			`i.type = ? and pm.project_name = ?
				and (
					bi.project_name = ?
					or not exists (
						select 1 from board_issues b2
						where b2.board_id = bi.board_id
						  and b2.project_name is not null and b2.project_name <> ''
					)
				)`,
			"INCIDENT", projectName, projectName,
		),
	}
}

// quoteTableColumn returns the `table` column of the given alias quoted for the
// current dialect. `table` is a reserved word: MySQL quotes identifiers with
// backticks, while Postgres (and SQLite, used by the unit tests) use double
// quotes.
func quoteTableColumn(db dal.Dal, alias string) string {
	if db.Dialect() == "mysql" {
		return alias + ".`table`"
	}
	return alias + `."table"`
}

func ConvertIssuesToIncidents(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*DoraTaskData)
	if data.DisableIssueToIncidentGenerator {
		return nil
	}
	pmTable := quoteTableColumn(db, "pm")
	// clear previous incidents and incident_assignees from the project
	deleteIncidentsSql := `
		DELETE
		FROM incidents
		WHERE id IN
			(SELECT i.id
			 FROM issues i
			 JOIN board_issues bi ON bi.issue_id = i.id
			 JOIN project_mapping pm ON pm.row_id = bi.board_id AND ` + pmTable + ` = 'boards'
			 WHERE i.type = ?
			   AND pm.project_name = ?
			   AND (
			     bi.project_name = ?
			     OR NOT EXISTS (
			       SELECT 1 FROM board_issues b2
			       WHERE b2.board_id = bi.board_id
			         AND b2.project_name IS NOT NULL AND b2.project_name <> ''
			     )
			   ))
	`
	if err := db.Exec(deleteIncidentsSql, "INCIDENT", data.Options.ProjectName, data.Options.ProjectName); err != nil {
		return errors.Default.Wrap(err, "error deleting previous incidents")

	}
	deleteIncidentAssigneesSql := `
		DELETE
		FROM incident_assignees
		WHERE incident_id IN
			(SELECT i.id
			 FROM issues i
			 JOIN board_issues bi ON bi.issue_id = i.id
			 JOIN project_mapping pm ON pm.row_id = bi.board_id AND ` + pmTable + ` = 'boards'
			 WHERE i.type = ?
			   AND pm.project_name = ?
			   AND (
			     bi.project_name = ?
			     OR NOT EXISTS (
			       SELECT 1 FROM board_issues b2
			       WHERE b2.board_id = bi.board_id
			         AND b2.project_name IS NOT NULL AND b2.project_name <> ''
			     )
			   ))
	`
	if err := db.Exec(deleteIncidentAssigneesSql, "INCIDENT", data.Options.ProjectName, data.Options.ProjectName); err != nil {
		return errors.Default.Wrap(err, "error deleting previous incident_assignees")
	}

	// select all issues belongs to the board, scoped to this project's slice
	cursor, err := db.Cursor(inScopeIncidentIssueClauses(db, data.Options.ProjectName)...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	enricher, err := api.NewDataConverter(api.DataConverterArgs{
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: DoraApiParams{
				ProjectName: data.Options.ProjectName,
			},
			Table: ticket.Issue{}.TableName(),
		},
		InputRowType: reflect.TypeOf(issueWithBoardId{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			issueWithBoardId := inputRow.(*issueWithBoardId)
			incident, err := issueWithBoardId.ToIncident(issueWithBoardId.BoardId)
			if err != nil {
				return nil, errors.Convert(err)
			}
			incidentAssignees, err := generateIncidentAssigneeFromIssue(db, taskCtx.GetLogger(), &issueWithBoardId.Issue)
			if err != nil {
				return nil, errors.Convert(err)
			}
			ret := []interface{}{incident}
			for _, assignee := range incidentAssignees {
				ret = append(ret, assignee)
			}
			return ret, nil
		},
	})
	if err != nil {
		return err
	}
	return enricher.Execute()
}

func generateIncidentAssigneeFromIssue(db dal.Dal, logger log.Logger, issue *ticket.Issue) ([]*ticket.IncidentAssignee, error) {
	if issue == nil {
		return nil, goerrors.New("issue is nil")
	}
	var issueAssignees []*ticket.IssueAssignee
	if err := db.All(&issueAssignees, dal.Where("issue_id = ?", issue.Id)); err != nil {
		logger.Error(err, "Failed to fetch issue assignees")
		return nil, err
	}
	incidentAssignee, err := issue.ToIncidentAssignee()
	if err != nil {
		return nil, err
	}
	incidentAssignees := []*ticket.IncidentAssignee{
		incidentAssignee,
	}
	for _, issueAssignee := range issueAssignees {
		incidentAssignees = append(incidentAssignees, &ticket.IncidentAssignee{
			IncidentId:   issueAssignee.IssueId,
			AssigneeId:   issueAssignee.AssigneeId,
			AssigneeName: issueAssignee.AssigneeName,
			NoPKModel:    common.NewNoPKModel(),
		})
	}
	return incidentAssignees, nil
}
