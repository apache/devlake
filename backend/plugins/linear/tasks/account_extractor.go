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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

var ExtractAccountsMeta = plugin.SubTaskMeta{
	Name:             "Extract Users",
	EntryPoint:       ExtractAccounts,
	EnabledByDefault: true,
	Description:      "Extract raw user data into tool layer table _tool_linear_accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}

var _ plugin.SubTaskEntryPoint = ExtractAccounts

func ExtractAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LinearTaskData)
	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ACCOUNTS_TABLE,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			apiAccount := &GraphqlQueryAccount{}
			if err := errors.Convert(json.Unmarshal(row.Data, apiAccount)); err != nil {
				return nil, err
			}
			if apiAccount.Id == "" {
				return nil, nil
			}
			account := &models.LinearAccount{
				ConnectionId: data.Options.ConnectionId,
				Id:           apiAccount.Id,
				Name:         apiAccount.Name,
				DisplayName:  apiAccount.DisplayName,
				Email:        apiAccount.Email,
				AvatarUrl:    apiAccount.AvatarUrl,
				Active:       apiAccount.Active,
			}
			return []interface{}{account}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
