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
	"encoding/json"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/core/utils"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

var ExtractCommentsMeta = plugin.SubTaskMeta{
	Name:             "Extract Comments",
	EntryPoint:       ExtractComments,
	EnabledByDefault: true,
	Description:      "Extract raw issue comments into _tool_youtrack_issue_comments (keeping the deleted flag) and _tool_youtrack_accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{RAW_ISSUE_COMMENTS_TABLE},
	ProductTables: []string{
		models.YoutrackIssueComment{}.TableName(),
		models.YoutrackAccount{}.TableName(),
	},
}

var _ plugin.SubTaskEntryPoint = ExtractComments

// apiComment mirrors the comment collector's fields param (commentFields).
// YouTrack calls the body `text`; the tool column is Body.
type apiComment struct {
	Id      string   `json:"id"`
	Text    string   `json:"text"`
	Created int64    `json:"created"`
	Updated *int64   `json:"updated"`
	Deleted bool     `json:"deleted"`
	Author  *apiUser `json:"author"`
}

func ExtractComments(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*YoutrackTaskData)
	connectionId := data.Options.ConnectionId
	projectId := data.Options.ProjectId
	// comment authors derive into accounts only when CROSS is collected;
	// the guest account is skipped at conversion
	deriveAccounts := utils.StringsContains(data.ScopeConfig.Entities, plugin.DOMAIN_TYPE_CROSS)

	extractor, err := helper.NewStatefulApiExtractor(&helper.StatefulApiExtractorArgs[apiComment]{
		SubtaskCommonArgs: &helper.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_ISSUE_COMMENTS_TABLE,
			Params: models.YoutrackApiParams{
				ConnectionId: connectionId,
				ProjectId:    projectId,
			},
			// every stateful extractor receives the whole scope config, so a
			// config change re-extracts from the raw layer without new
			// YouTrack API calls
			SubtaskConfig: data.ScopeConfig,
		},
		Extract: func(apiComment *apiComment, row *helper.RawData) ([]interface{}, errors.Error) {
			input := &youtrackCommentInput{}
			if err := errors.Convert(json.Unmarshal(row.Input, input)); err != nil {
				return nil, err
			}
			comment := &models.YoutrackIssueComment{
				ConnectionId: connectionId,
				Id:           apiComment.Id,
				IssueId:      input.Id,
				Body:         apiComment.Text,
				Created:      time.UnixMilli(apiComment.Created).UTC(),
				Deleted:      apiComment.Deleted,
			}
			if apiComment.Updated != nil {
				updated := time.UnixMilli(*apiComment.Updated).UTC()
				comment.Updated = &updated
			}
			if apiComment.Author != nil {
				comment.AuthorId = apiComment.Author.Id
			}
			results := []interface{}{comment}
			if deriveAccounts {
				if account := deriveAccount(connectionId, apiComment.Author); account != nil {
					results = append(results, account)
				}
			}
			return results, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
