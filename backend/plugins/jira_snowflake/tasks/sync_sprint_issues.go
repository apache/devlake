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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

var SyncSprintIssuesMeta = plugin.SubTaskMeta{
	Name:             "syncSprintIssues",
	EntryPoint:       SyncSprintIssues,
	EnabledByDefault: false,
	Description:      "Sync sprint-issue memberships from Snowflake into _tool_jira_sprint_issues",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func SyncSprintIssues(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	projectKeys := data.Options.ProjectKeys

	inClause, args := buildProjectInClause(projectKeys)

	const queryTmpl = `
SELECT
    TRY_CAST(cf.NUMBERVALUE AS BIGINT) AS sprint_id,
    i.ID                               AS issue_id
FROM JIRA_CUSTOMFIELDVALUE_NON_PII cf
JOIN JIRA_ISSUE_NON_PII i ON i.ID = cf.ISSUE
WHERE cf.CUSTOMFIELD_NAME = 'Sprint'
  AND cf.NUMBERVALUE IS NOT NULL
  AND i.PROJECT IN %s
`
	query := fmt.Sprintf(queryTmpl, inClause) //nolint:gosec // G201: inClause contains only '?' placeholders from buildProjectInClause; no user data is interpolated

	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for sprint issues")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var sprintId, issueId uint64
		if scanErr := rows.Scan(&sprintId, &issueId); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan Snowflake sprint issue row")
		}
		si := &jiramodels.JiraSprintIssue{
			ConnectionId: connectionId,
			SprintId:     sprintId,
			IssueId:      issueId,
		}
		if dbErr := db.CreateOrUpdate(si); dbErr != nil {
			return dbErr
		}
		count++
	}
	if goErr = rows.Err(); goErr != nil {
		return errors.Default.Wrap(goErr, "Snowflake sprint issue rows iteration error")
	}
	logger.Info("finished syncing %d sprint-issue records", count)
	return nil
}
