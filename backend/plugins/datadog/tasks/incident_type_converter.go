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
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/datadog/models"
)

var _ plugin.SubTaskEntryPoint = ConvertIncidentTypes

var ConvertIncidentTypesMeta = plugin.SubTaskMeta{
	Name:             "convertIncidentTypes",
	EntryPoint:       ConvertIncidentTypes,
	EnabledByDefault: true,
	Description:      "Convert Datadog incident types into domain-layer boards",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func ConvertIncidentTypes(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*DatadogTaskData)

	cursor, err := db.Cursor(
		dal.From(&models.IncidentType{}),
		dal.Where("connection_id = ? AND id = ?", data.Options.ConnectionId, data.Options.IncidentTypeId),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	idGen := didgen.NewDomainIdGenerator(&models.IncidentType{})

	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Options: data.Options,
			Table:   RAW_INCIDENT_TYPES_TABLE,
		},
		InputRowType: reflect.TypeOf(models.IncidentType{}),
		Input:        cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			incidentType := inputRow.(*models.IncidentType)
			// Built by hand rather than with ticket.NewBoard, which stamps
			// CreatedDate with the row's insert time. Datadog reports when
			// the incident type was actually created, which is both truer
			// and stable across collections.
			board := &ticket.Board{
				DomainEntity: domainlayer.DomainEntity{
					Id: idGen.Generate(data.Options.ConnectionId, incidentType.Id),
				},
				Name:        incidentType.Name,
				CreatedDate: incidentType.CreatedDate,
			}
			return []interface{}{board}, nil
		},
	})
	if err != nil {
		return err
	}
	return converter.Execute()
}
