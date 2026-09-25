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

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

var ConvertAccountsMeta = plugin.SubTaskMeta{
	Name:             "Convert Accounts",
	EntryPoint:       ConvertAccounts,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_youtrack_accounts into domain layer table accounts (skips the built-in guest)",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{models.YoutrackAccount{}.TableName()},
	ProductTables:    []string{crossdomain.Account{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertAccounts

func ConvertAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*YoutrackTaskData)
	connectionId := data.Options.ConnectionId
	accountIdGen := didgen.NewDomainIdGenerator(&models.YoutrackAccount{})

	cursor, err := db.Cursor(
		dal.From(&models.YoutrackAccount{}),
		dal.Where("connection_id = ?", connectionId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Options: models.YoutrackApiParams{
				ConnectionId: connectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_ISSUES_TABLE,
		},
		InputRowType: reflect.TypeOf(models.YoutrackAccount{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			account := inputRow.(*models.YoutrackAccount)
			// the built-in guest account never converts
			if account.Guest {
				return nil, nil
			}
			domainAccount := &crossdomain.Account{
				DomainEntity: domainlayer.DomainEntity{Id: accountIdGen.Generate(connectionId, account.Id)},
				UserName:     account.Login,
				FullName:     account.FullName,
				Email:        account.Email,
				AvatarUrl:    account.AvatarUrl,
			}
			return []interface{}{domainAccount}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
