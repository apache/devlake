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
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var SyncSprintsMeta = plugin.SubTaskMeta{
	Name:             "syncSprints",
	EntryPoint:       SyncSprints,
	EnabledByDefault: false,
	Description:      "Sync Jira sprints from Snowflake into _tool_jira_sprints and _tool_jira_board_sprints",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func SyncSprints(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	boardId := data.Options.BoardId
	projectKeys := data.Options.ProjectKeys

	inClause, args := buildProjectInClause(projectKeys)

	// Select distinct sprints associated with issues in the board's projects.
	// State is derived from STARTED/CLOSED booleans.
	const queryTmpl = `
SELECT DISTINCT
    sp.ID                                   AS sprint_id,
    sp.NAME                                 AS sprint_name,
    CASE
        WHEN sp.CLOSED  THEN 'closed'
        WHEN sp.STARTED THEN 'active'
        ELSE 'future'
    END                                     AS state,
    sp.STARTDATE,
    sp.ENDDATE,
    sp.COMPLETEDATE,
    sp.RAPID_VIEW_ID                        AS origin_board_id
FROM JIRA_SPRINT sp
JOIN JIRA_CUSTOMFIELDVALUE_NON_PII cf
    ON cf.CUSTOMFIELD_NAME = 'Sprint'
    AND TRY_CAST(cf.NUMBERVALUE AS BIGINT) = sp.ID
JOIN JIRA_ISSUE_NON_PII i ON i.ID = cf.ISSUE
WHERE i.PROJECT IN %s
`
	query := fmt.Sprintf(queryTmpl, inClause) //nolint:gosec // G201: inClause contains only '?' placeholders from buildProjectInClause; no user data is interpolated

	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for sprints")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			sprintId      uint64
			sprintName    string
			state         string
			startDate     *time.Time
			endDate       *time.Time
			completeDate  *time.Time
			originBoardId uint64
		)
		if scanErr := rows.Scan(&sprintId, &sprintName, &state, &startDate, &endDate, &completeDate, &originBoardId); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan Snowflake sprint row")
		}

		sprint := &jiramodels.JiraSprint{
			ConnectionId:  connectionId,
			SprintId:      sprintId,
			Name:          sprintName,
			State:         state,
			StartDate:     startDate,
			EndDate:       endDate,
			CompleteDate:  completeDate,
			OriginBoardID: originBoardId,
		}
		if dbErr := db.CreateOrUpdate(sprint); dbErr != nil {
			return dbErr
		}

		boardSprint := &jiramodels.JiraBoardSprint{
			ConnectionId: connectionId,
			BoardId:      boardId,
			SprintId:     sprintId,
		}
		if dbErr := db.CreateOrUpdate(boardSprint); dbErr != nil {
			return dbErr
		}

		count++
	}
	if goErr = rows.Err(); goErr != nil {
		return errors.Default.Wrap(goErr, "Snowflake sprint rows iteration error")
	}
	logger.Info("finished syncing %d sprints for board %d", count, boardId)
	return nil
}
