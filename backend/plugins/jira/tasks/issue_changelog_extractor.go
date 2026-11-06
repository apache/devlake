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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/jira/models"
	"github.com/apache/devlake/plugins/jira/tasks/apiv2models"
)

var _ plugin.SubTaskEntryPoint = ExtractIssueChangelogs

var ExtractIssueChangelogsMeta = plugin.SubTaskMeta{
	Name:             "extractIssueChangelogs",
	EntryPoint:       ExtractIssueChangelogs,
	EnabledByDefault: true,
	Description:      "extract Jira Issue change logs",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET, plugin.DOMAIN_TYPE_CROSS},
}

func ExtractIssueChangelogs(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*JiraTaskData)
	if data.JiraServerInfo.DeploymentType == models.DeploymentServer {
		return nil
	}
	connectionId := data.Options.ConnectionId
	userFieldMap, err := getUserFieldMap(subtaskCtx.GetDal(), connectionId, subtaskCtx.GetLogger())
	if err != nil {
		return err
	}
	extractor, err := api.NewStatefulApiExtractor(&api.StatefulApiExtractorArgs[apiv2models.Changelog]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: subtaskCtx,
			Table:          RAW_CHANGELOG_TABLE,
			Params: JiraApiParams{
				ConnectionId: data.Options.ConnectionId,
				BoardId:      data.Options.BoardId,
			},
		},
		Extract: func(changelog *apiv2models.Changelog, row *api.RawData) ([]interface{}, errors.Error) {
			// process input
			var input apiv2models.Input
			err := errors.Convert(json.Unmarshal(row.Input, &input))
			if err != nil {
				return nil, err
			}
			// prepare output
			var result []interface{}
			cl, user := changelog.ToToolLayer(connectionId, input.IssueId, &input.UpdateTime)
			// this is crucial for incremental update
			cl.IssueUpdated = &input.UpdateTime
			// collect changelog / user information
			result = append(result, cl)
			if user != nil {
				result = append(result, user)
			}
			// collect changelog_items
			for _, indexed := range apiv2models.IndexChangelogItems(changelog.Items) {
				result = append(result, indexed.Item.ToToolLayer(connectionId, changelog.ID, indexed.ItemIndex))
				extractedUsersFromChangelogItem := indexed.Item.ExtractUser(connectionId, userFieldMap)
				for _, u := range extractedUsersFromChangelogItem {
					if u != nil && u.AccountId != "" {
						result = append(result, u)
					}
				}
			}
			return result, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
