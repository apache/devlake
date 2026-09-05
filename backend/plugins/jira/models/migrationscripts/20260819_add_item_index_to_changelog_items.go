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

package migrationscripts

import (
	"fmt"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*addItemIndexToChangelogItems)(nil)

const jiraIssueChangelogItemsTable20260819 = "_tool_jira_issue_changelog_items"

type jiraIssueChangelogItems20260819 struct {
	ItemIndex uint64 `gorm:"column:item_index;not null;default:0"`
}

func (jiraIssueChangelogItems20260819) TableName() string {
	return jiraIssueChangelogItemsTable20260819
}

type addItemIndexToChangelogItems struct{}

func (script *addItemIndexToChangelogItems) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	if !db.HasColumn(jiraIssueChangelogItemsTable20260819, "item_index") {
		err := migrationhelper.AutoMigrateTables(basicRes, &jiraIssueChangelogItems20260819{})
		if err != nil {
			return err
		}
	}

	var dropPK string
	if db.Dialect() == "mysql" {
		dropPK = fmt.Sprintf("ALTER TABLE %s DROP PRIMARY KEY", jiraIssueChangelogItemsTable20260819)
	} else {
		dropPK = fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s_pkey", jiraIssueChangelogItemsTable20260819, jiraIssueChangelogItemsTable20260819)
	}
	if err := db.Exec(dropPK); err != nil {
		return err
	}
	addPK := fmt.Sprintf(
		"ALTER TABLE %s ADD PRIMARY KEY (connection_id, changelog_id, field, item_index)",
		jiraIssueChangelogItemsTable20260819,
	)
	if err := db.Exec(addPK); err != nil {
		return err
	}

	if err := db.Delete(
		"_devlake_subtask_states",
		dal.Where(
			"plugin = ? AND subtask IN ?",
			"jira",
			[]string{"extractIssues", "extractIssueChangelogs", "convertIssueChangelogs"},
		),
	); err != nil {
		return err
	}
	return db.Delete("issue_changelogs", dal.Where("id LIKE ?", "jira:JiraIssueChangelogItems:%"))
}

func (*addItemIndexToChangelogItems) Version() uint64 {
	return 20260819120000
}

func (*addItemIndexToChangelogItems) Name() string {
	return "add item_index to _tool_jira_issue_changelog_items primary key"
}
