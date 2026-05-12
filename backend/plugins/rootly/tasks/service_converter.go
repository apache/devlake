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
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/rootly/models"
)

var ConvertServicesMeta = plugin.SubTaskMeta{
	Name:             "convertServices",
	EntryPoint:       ConvertServices,
	EnabledByDefault: true,
	Description:      "Convert Rootly services",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertServices(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*RootlyTaskData)
	rawDataSubTaskArgs := &helper.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Options: data.Options,
		Table:   RAW_SERVICES_TABLE,
	}
	clauses := []dal.Clause{
		dal.Select("services.*"),
		dal.From("_tool_rootly_services services"),
		dal.Where("id = ? and connection_id = ?", data.Options.ServiceId, data.Options.ConnectionId),
	}
	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.Service{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			service := inputRow.(*models.Service)
			domainBoard := &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{
					Id: didgen.NewDomainIdGenerator(service).Generate(service.ConnectionId, service.Id),
				},
				Name: service.Name,
				Url:  service.Url,
			}
			return []interface{}{
				domainBoard,
			}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
