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
	"github.com/apache/incubator-devlake/plugins/asana/models"
)

var _ plugin.SubTaskEntryPoint = ConvertUser

var ConvertUserMeta = plugin.SubTaskMeta{
	Name:             "ConvertUser",
	EntryPoint:       ConvertUser,
	EnabledByDefault: true,
	Description:      "Convert tool layer Asana users into domain layer accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}

func ConvertUser(taskCtx plugin.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, rawUserTable)
	db := taskCtx.GetDal()
	connectionId := data.Options.ConnectionId

	clauses := []dal.Clause{
		dal.From(&models.AsanaUser{}),
		dal.Where("connection_id = ?", connectionId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	accountIdGen := didgen.NewDomainIdGenerator(&models.AsanaUser{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.AsanaUser{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			toolUser := inputRow.(*models.AsanaUser)
			domainAccount := &crossdomain.Account{
				DomainEntity: domainlayer.DomainEntity{Id: accountIdGen.Generate(toolUser.ConnectionId, toolUser.Gid)},
				Email:        toolUser.Email,
				FullName:     toolUser.Name,
				UserName:     toolUser.Name,
				AvatarUrl:    toolUser.PhotoUrl,
			}
			return []interface{}{domainAccount}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
