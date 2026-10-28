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
	"regexp"
	"strings"

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	gitlabModels "github.com/apache/devlake/plugins/gitlab/models"
)

// gitlabBotTokenPattern matches GitLab project/group access token usernames,
// e.g. "project_78877_bot_ec63972b..." or "group_80889_bot_73ed0037...".
var gitlabBotTokenPattern = regexp.MustCompile(`^(project|group)_[0-9]+_bot`)

// isGitlabBotAccount reports whether a GitLab account identifies a bot.
// GitLab accounts have no API-reported type field, so detection relies on
// username conventions (access token prefixes, -bot/-robot suffixes) and
// the account's display name containing "Service Account".
func isGitlabBotAccount(username string, fullName string) bool {
	lowerUsername := strings.ToLower(username)
	if gitlabBotTokenPattern.MatchString(lowerUsername) {
		return true
	}
	if strings.HasSuffix(lowerUsername, "-bot") || strings.HasSuffix(lowerUsername, "-robot") {
		return true
	}
	return strings.Contains(strings.ToLower(fullName), "service account")
}

// buildGitlabDomainAccount converts a tool-layer GitlabAccount row into a
// domain Account, flagging bot identities via isGitlabBotAccount.
func buildGitlabDomainAccount(id string, gitlabAccount *gitlabModels.GitlabAccount) *crossdomain.Account {
	return &crossdomain.Account{
		DomainEntity: domainlayer.DomainEntity{Id: id},
		UserName:     gitlabAccount.Username,
		FullName:     gitlabAccount.Name,
		Email:        gitlabAccount.Email,
		AvatarUrl:    gitlabAccount.AvatarUrl,
		CreatedDate:  gitlabAccount.CreatedUserAt,
		IsBot:        isGitlabBotAccount(gitlabAccount.Username, gitlabAccount.Name),
	}
}

func init() {
	RegisterSubtaskMeta(&ConvertAccountsMeta)
}

var ConvertAccountsMeta = plugin.SubTaskMeta{
	Name:             "Convert Users",
	EntryPoint:       ConvertAccounts,
	EnabledByDefault: true,
	Description:      "Convert tool layer table gitlab_users into  domain layer table accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAccountsMeta},
}

func ConvertAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GitlabTaskData)

	cursor, err := db.Cursor(dal.From(gitlabModels.GitlabAccount{}), dal.Where("connection_id = ?", data.Options.ConnectionId))
	if err != nil {
		return err
	}
	defer cursor.Close()

	accountIdGen := didgen.NewDomainIdGenerator(&gitlabModels.GitlabAccount{})
	converter, err := api.NewDataConverter(api.DataConverterArgs{
		InputRowType: reflect.TypeOf(gitlabModels.GitlabAccount{}),
		Input:        cursor,
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: gitlabModels.GitlabApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_USER_TABLE,
		},
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			GitlabAccount := inputRow.(*gitlabModels.GitlabAccount)
			domainUser := buildGitlabDomainAccount(accountIdGen.Generate(data.Options.ConnectionId, GitlabAccount.GitlabId), GitlabAccount)

			return []interface{}{
				domainUser,
			}, nil
		},
	})

	if err != nil {
		return err
	}

	return converter.Execute()
}
