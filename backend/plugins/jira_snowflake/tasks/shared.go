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

// Adapted from plugins/jira/tasks/shared.go and issue_extractor.go.
// TODO: long-term these helpers should be extracted to helpers/jiraconvertors/
// so both jira and jira_snowflake import from a neutral package.

import (
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	jiramodels "github.com/apache/devlake/plugins/jira/models"
)

// buildProjectInClause builds a parameterized IN clause fragment "(?, ?, ...)"
// and the corresponding argument slice for the given project keys.
// Usage: clause, args := buildProjectInClause(keys); db.QueryContext(ctx, "...IN "+clause, args...)
func buildProjectInClause(projectKeys []string) (string, []interface{}) {
	placeholders := make([]string, len(projectKeys))
	args := make([]interface{}, len(projectKeys))
	for i, k := range projectKeys {
		placeholders[i] = "?"
		args[i] = k
	}
	return "(" + strings.Join(placeholders, ", ") + ")", args
}

// getStdStatus maps a Jira status category key to a DevLake standard status.
// Copied from plugins/jira/tasks/shared.go.
func getStdStatus(statusKey string) string {
	if statusKey == "done" {
		return ticket.DONE
	} else if statusKey == "new" {
		return ticket.TODO
	}
	return ticket.IN_PROGRESS
}

type typeMappings struct {
	StdTypeMappings        map[string]string
	StandardStatusMappings map[string]jiramodels.StatusMappings
}

// getTypeMappings builds type/status mappings from the scope config.
// In jira_snowflake, JiraIssue.Type is already the type name (not an ID),
// so TypeIdMappings are not needed.
// Adapted from plugins/jira/tasks/issue_extractor.go getTypeMappings().
func getTypeMappings(data *JiraSnowflakeTaskData, _ dal.Dal) (*typeMappings, errors.Error) {
	stdTypeMappings := make(map[string]string)
	standardStatusMappings := make(map[string]jiramodels.StatusMappings)
	if data.Options.ScopeConfig != nil {
		for userType, stdType := range data.Options.ScopeConfig.TypeMappings {
			stdTypeMappings[userType] = strings.ToUpper(stdType.StandardType)
			standardStatusMappings[userType] = stdType.StatusMappings
		}
	}
	return &typeMappings{
		StdTypeMappings:        stdTypeMappings,
		StandardStatusMappings: standardStatusMappings,
	}, nil
}
