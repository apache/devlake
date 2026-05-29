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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/linear/models"
)

var ConvertAccountsMeta = plugin.SubTaskMeta{
	Name:             "Convert Users",
	EntryPoint:       ConvertAccounts,
	EnabledByDefault: true,
	Description:      "Convert tool layer table _tool_linear_accounts into domain layer table accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{models.LinearAccount{}.TableName()},
	ProductTables:    []string{crossdomain.Account{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ConvertAccounts

func ConvertAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*LinearTaskData)
	accountIdGen := didgen.NewDomainIdGenerator(&models.LinearAccount{})

	cursor, err := db.Cursor(
		dal.From(&models.LinearAccount{}),
		dal.Where("connection_id = ?", data.Options.ConnectionId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_ACCOUNTS_TABLE,
		},
		InputRowType: reflect.TypeOf(models.LinearAccount{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			account := inputRow.(*models.LinearAccount)
			status := 1
			if !account.Active {
				status = 0
			}
			fullName := account.Name
			if account.DisplayName != "" {
				fullName = account.DisplayName
			}
			domainAccount := &crossdomain.Account{
				DomainEntity: domainlayer.DomainEntity{
					Id: accountIdGen.Generate(data.Options.ConnectionId, account.Id),
				},
				UserName:  account.Name,
				FullName:  fullName,
				Email:     account.Email,
				AvatarUrl: account.AvatarUrl,
				Status:    status,
			}
			return []interface{}{domainAccount}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
