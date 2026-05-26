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
	"encoding/base64"
	"github.com/apache/incubator-devlake/core/config"
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
	"github.com/apache/incubator-devlake/plugins/jira/models/migrationscripts/archived"
	"strings"
	"time"
)

type jiraConnection20220716After struct {
	archived.RestConnection `mapstructure:",squash"`
	archived.BasicAuth      `mapstructure:",squash"`
}

type jiraConnection20220716Before struct {
	ID                         uint64    `gorm:"primaryKey" json:"id"`
	CreatedAt                  time.Time `json:"createdAt"`
	UpdatedAt                  time.Time `json:"updatedAt"`
	Name                       string    `gorm:"type:varchar(100);uniqueIndex" json:"name" validate:"required"`
	Endpoint                   string    `json:"endpoint" validate:"required"`
	BasicAuthEncoded           string    `json:"basicAuthEncoded" validate:"required"`
	EpicKeyField               string    `gorm:"type:varchar(50);" json:"epicKeyField"`
	StoryPointField            string    `gorm:"type:varchar(50);" json:"storyPointField"`
	RemotelinkCommitShaPattern string    `gorm:"type:varchar(255);comment='golang regexp, the first group will be recognized as commit sha, ref https://github.com/google/re2/wiki/Syntax'" json:"remotelinkCommitShaPattern"`
	Proxy                      string    `json:"proxy"`
	RateLimit                  int       `comment:"api request rate limit per hour" json:"rateLimit"`
}

type addInitTables20220716 struct{}

func (script *addInitTables20220716) Up(basicRes context.BasicRes) errors.Error {
	var err errors.Error
	if err = basicRes.GetDal().DropTables(
		// history table
		"_raw_jira_api_users",
		"_raw_jira_api_boards",
		"_raw_jira_api_changelogs",
		"_raw_jira_api_issues",
		"_raw_jira_api_projects",
		"_raw_jira_api_remotelinks",
		"_raw_jira_api_sprints",
		"_raw_jira_api_status",
		"_raw_jira_api_worklogs",
		"_tool_jira_accounts",
		"_tool_jira_issue_type_mappings",
		"_tool_jira_issue_status_mappings",
		"_tool_jira_changelogs",
		"_tool_jira_changelog_items",
		&archived.JiraProject{},
		&archived.JiraIssue{},
		&archived.JiraBoard{},
		&archived.JiraBoardIssue{},
		&archived.JiraRemotelink{},
		&archived.JiraIssueCommit{},
		&archived.JiraSprint{},
		&archived.JiraBoardSprint{},
		&archived.JiraSprintIssue{},
		&archived.JiraWorklog{},
	); err != nil {
		return err
	}
	encKey := config.GetConfig().GetString(plugin.EncodeKeyEnvStr)
	if encKey == "" {
		return errors.BadInput.New("jira v0.11 invalid encKey")
	}

	err = migrationhelper.TransformColumns(
		basicRes,
		script,
		"_tool_jira_connections",
		[]string{
			"rate_limit",
			"basic_auth_encoded",
		},
		func(old *jiraConnection20220716Before) (*jiraConnection20220716After, errors.Error) {
			conn := &jiraConnection20220716After{}
			conn.ID = old.ID
			conn.Name = old.Name
			conn.Endpoint = old.Endpoint
			conn.Proxy = old.Proxy
			conn.RateLimitPerHour = old.RateLimit

			var auth string
			if auth, err = plugin.Decrypt(encKey, old.BasicAuthEncoded); err != nil {
				return nil, err
			}
			pk, err1 := base64.StdEncoding.DecodeString(auth)
			if err1 != nil {
				return nil, errors.Convert(err1)
			}
			originInfo := strings.Split(string(pk), ":")
			if len(originInfo) == 2 {
				conn.Username = originInfo[0]
				conn.Password, err = plugin.Encrypt(encKey, originInfo[1])
				if err != nil {
					return nil, err
				}
				return conn, nil
			}
			return nil, errors.Default.New("invalid BasicAuthEncoded")
		})
	if err != nil {
		return err
	}
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&archived.JiraAccount{},
		&archived.JiraBoardIssue{},
		&archived.JiraBoard{},
		&archived.JiraIssueChangelogItems{},
		&archived.JiraIssueChangelogs{},
		&archived.JiraIssueCommit{},
		&archived.JiraIssueLabel{},
		&archived.JiraIssue{},
		&archived.JiraProject{},
		&archived.JiraRemotelink{},
		&archived.JiraSprint{},
		&archived.JiraBoardSprint{},
		&archived.JiraSprintIssue{},
		&archived.JiraStatus{},
		&archived.JiraWorklog{},
		&archived.JiraIssueType{},
	)
}

func (*addInitTables20220716) Version() uint64 {
	return 20220716201138
}

func (*addInitTables20220716) Name() string {
	return "Jira init schemas"
}
