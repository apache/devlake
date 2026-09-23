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

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

var ExtractWorkflowStatesMeta = plugin.SubTaskMeta{
	Name:             "Extract Workflow States",
	EntryPoint:       ExtractWorkflowStates,
	EnabledByDefault: true,
	Description:      "Extract the State-bundle values of the project's custom fields into _tool_youtrack_workflow_states",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	DependencyTables: []string{RAW_WORKFLOW_STATES_TABLE},
	ProductTables:    []string{models.YoutrackWorkflowState{}.TableName()},
}

var _ plugin.SubTaskEntryPoint = ExtractWorkflowStates

// apiProjectCustomField mirrors the collector's fields param: one entry of
// the /admin/projects/{id}/customFields response.
type apiProjectCustomField struct {
	Id    string `json:"id"`
	Type  string `json:"$type"`
	Field struct {
		Id        string `json:"id"`
		Name      string `json:"name"`
		FieldType struct {
			Id           string `json:"id"`
			ValueType    string `json:"valueType"`
			IsMultiValue bool   `json:"isMultiValue"`
		} `json:"fieldType"`
	} `json:"field"`
	Bundle *struct {
		Id     string `json:"id"`
		Type   string `json:"$type"`
		Values []struct {
			Id         string `json:"id"`
			Name       string `json:"name"`
			IsResolved bool   `json:"isResolved"`
			Ordinal    int    `json:"ordinal"`
			Archived   bool   `json:"archived"`
		} `json:"values"`
	} `json:"bundle"`
}

// stateProjectCustomFieldType is the $type of a project-level State field.
// Only State bundles are persisted: isResolved feeds the
// zero-config status default; type-bundle values are read live by the
// mapping widget and never stored.
const stateProjectCustomFieldType = "StateProjectCustomField"

func ExtractWorkflowStates(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*YoutrackTaskData)
	db := taskCtx.GetDal()

	// Full refresh: the collector re-fetches the whole bundle set
	// each run, so replace the scope's rows — a state renamed or removed
	// upstream must not linger (this table feeds the changelog std-status
	// lookup in the changelog convertor).
	if err := db.Delete(
		&models.YoutrackWorkflowState{},
		dal.Where("connection_id = ? AND project_id = ?", data.Options.ConnectionId, data.Options.ProjectId),
	); err != nil {
		return err
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Options: models.YoutrackApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_WORKFLOW_STATES_TABLE,
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var payload []apiProjectCustomField
			if err := errors.Convert(json.Unmarshal(row.Data, &payload)); err != nil {
				return nil, err
			}
			if len(payload) == 0 {
				// [] means the token lacks Read Project on this project —
				// the API never answers 403 here. Warn, and
				// let the run continue with empty state columns downstream.
				taskCtx.GetLogger().Warn(nil,
					"project %s returned no custom fields — this usually means the token lacks permission, not that the project has no fields",
					data.Options.ProjectId)
			}
			var results []interface{}
			for _, field := range payload {
				if field.Type != stateProjectCustomFieldType || field.Bundle == nil {
					continue
				}
				for _, value := range field.Bundle.Values {
					results = append(results, &models.YoutrackWorkflowState{
						ConnectionId: data.Options.ConnectionId,
						ProjectId:    data.Options.ProjectId,
						Id:           value.Id,
						Name:         value.Name,
						IsResolved:   value.IsResolved,
						Ordinal:      value.Ordinal,
						Archived:     value.Archived,
					})
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
