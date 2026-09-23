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
	"io"
	"net/http"
	"net/url"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/youtrack/models"
)

const RAW_WORKFLOW_STATES_TABLE = "youtrack_workflow_states"

// workflowStatesFields is the collector's fields param.
const workflowStatesFields = "id,$type,field(id,name,fieldType(id,valueType,isMultiValue)),bundle(id,values(id,name,isResolved,ordinal,archived))"

var CollectWorkflowStatesMeta = plugin.SubTaskMeta{
	Name:             "Collect Workflow States",
	EntryPoint:       CollectWorkflowStates,
	EnabledByDefault: true,
	Description:      "Collect the project's custom-field definitions with their bundles (full refresh each run)",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

var _ plugin.SubTaskEntryPoint = CollectWorkflowStates

func CollectWorkflowStates(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*YoutrackTaskData)
	apiCollector, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Options: models.YoutrackApiParams{
			ConnectionId: data.Options.ConnectionId,
			ProjectId:    data.Options.ProjectId,
		},
		Table: RAW_WORKFLOW_STATES_TABLE,
	})
	if err != nil {
		return err
	}

	// Full refresh: one unpaginated request returns every custom field with
	// its bundle. A 200 with an empty array means the token lacks
	// Read Project on this project — never "the project has no fields" —
	// so the extractor's warn and the config-ui banner carry the signal.
	err = apiCollector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		UrlTemplate: "admin/projects/{{ .Params.ProjectId }}/customFields",
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			return url.Values{"fields": []string{workflowStatesFields}}, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			blob, err := io.ReadAll(res.Body)
			if err != nil {
				return nil, errors.Convert(err)
			}
			// the whole payload is one raw record: the extractor resolves
			// fields against the bundle in the same response
			return []json.RawMessage{blob}, nil
		},
	})
	if err != nil {
		return err
	}
	return apiCollector.Execute()
}
