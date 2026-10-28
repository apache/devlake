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

var SyncWorklogsMeta = plugin.SubTaskMeta{
	Name:             "syncWorklogs",
	EntryPoint:       SyncWorklogs,
	EnabledByDefault: false,
	Description:      "Sync Jira worklogs from Snowflake into _tool_jira_worklogs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func SyncWorklogs(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	projectKeys := data.Options.ProjectKeys

	inClause, args := buildProjectInClause(projectKeys)
	syncPolicy := subtaskCtx.TaskContext().SyncPolicy()
	timeFilter := ""
	if syncPolicy != nil && syncPolicy.TimeAfter != nil {
		timeFilter = "AND w.UPDATED > ?"
		args = append(args, *syncPolicy.TimeAfter)
	}

	// TIMEWORKED is in seconds — store directly into TimeSpentSeconds.
	// The worklog convertor (convert_worklogs.go) divides by 60 when writing to the domain layer.
	// Assignee/author fields are null (PII stripped in Snowflake).
	const queryTmpl = `
SELECT
    TO_VARCHAR(w.ID)    AS worklog_id,
    w.ISSUEID           AS issue_id,
    w.TIMEWORKED        AS time_spent_seconds,
    w.STARTDATE         AS started,
    w.UPDATED
FROM JIRA_WORKLOG w
JOIN JIRA_ISSUE_NON_PII i ON i.ID = w.ISSUEID
WHERE i.PROJECT IN %s
%s
`
	query := fmt.Sprintf(queryTmpl, inClause, timeFilter) //nolint:gosec // G201: inClause contains only '?' placeholders from buildProjectInClause; timeFilter is a static string

	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for worklogs")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			worklogId        string
			issueId          uint64
			timeSpentSeconds int
			started          time.Time
			updated          time.Time
		)
		if scanErr := rows.Scan(&worklogId, &issueId, &timeSpentSeconds, &started, &updated); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan Snowflake worklog row")
		}

		wl := &jiramodels.JiraWorklog{
			ConnectionId:     connectionId,
			IssueId:          issueId,
			WorklogId:        worklogId,
			TimeSpentSeconds: timeSpentSeconds,
			Started:          started,
			Updated:          updated,
		}
		if dbErr := db.CreateOrUpdate(wl); dbErr != nil {
			return dbErr
		}
		count++
	}
	if goErr = rows.Err(); goErr != nil {
		return errors.Default.Wrap(goErr, "Snowflake worklog rows iteration error")
	}
	logger.Info("finished syncing %d worklogs", count)
	return nil
}
