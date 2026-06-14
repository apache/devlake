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

var ExtractWorkflowStatesMeta = plugin.SubTaskMeta{
	Name:             "Extract Workflow States",
	EntryPoint:       ExtractWorkflowStates,
	EnabledByDefault: true,
	Description:      "Extract raw workflow state data into tool layer table _tool_linear_workflow_states",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = ExtractWorkflowStates

func ExtractWorkflowStates(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LinearTaskData)
	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LinearApiParams{
				ConnectionId: data.Options.ConnectionId,
				TeamId:       data.Options.TeamId,
			},
			Table: RAW_WORKFLOW_STATES_TABLE,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			apiState := &GraphqlQueryWorkflowState{}
			if err := errors.Convert(json.Unmarshal(row.Data, apiState)); err != nil {
				return nil, err
			}
			state := &models.LinearWorkflowState{
				ConnectionId: data.Options.ConnectionId,
				Id:           apiState.Id,
				TeamId:       data.Options.TeamId,
				Name:         apiState.Name,
				Type:         apiState.Type,
				Color:        apiState.Color,
				Position:     apiState.Position,
			}
			return []interface{}{state}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
