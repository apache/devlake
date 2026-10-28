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

var SyncLabelsMeta = plugin.SubTaskMeta{
	Name:             "syncLabels",
	EntryPoint:       SyncLabels,
	EnabledByDefault: false,
	Description:      "Sync Jira issue labels from Snowflake into _tool_jira_issue_labels",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func SyncLabels(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	projectKeys := data.Options.ProjectKeys

	inClause, args := buildProjectInClause(projectKeys)

	const queryTmpl = `
SELECT
    l.ISSUE     AS issue_id,
    l.LABELNAME AS label_name
FROM JIRA_LABEL l
JOIN JIRA_ISSUE_NON_PII i ON i.ID = l.ISSUE
WHERE i.PROJECT IN %s
`
	query := fmt.Sprintf(queryTmpl, inClause) //nolint:gosec // G201: inClause contains only '?' placeholders from buildProjectInClause; no user data is interpolated

	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for labels")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var issueId uint64
		var labelName string
		if scanErr := rows.Scan(&issueId, &labelName); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan Snowflake label row")
		}

		label := &jiramodels.JiraIssueLabel{
			ConnectionId: connectionId,
			IssueId:      issueId,
			LabelName:    labelName,
		}
		if dbErr := db.CreateOrUpdate(label); dbErr != nil {
			return dbErr
		}
		count++
	}
	if goErr = rows.Err(); goErr != nil {
		return errors.Default.Wrap(goErr, "Snowflake label rows iteration error")
	}
	logger.Info("finished syncing %d issue labels", count)
	return nil
}
