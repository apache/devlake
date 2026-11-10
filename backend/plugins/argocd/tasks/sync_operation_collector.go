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
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/argocd/models"
)

const RAW_SYNC_OPERATION_TABLE = "argocd_api_sync_operations"

var _ plugin.SubTaskEntryPoint = CollectSyncOperations

var CollectSyncOperationsMeta = plugin.SubTaskMeta{
	Name:             "collectSyncOperations",
	EntryPoint:       CollectSyncOperations,
	EnabledByDefault: true,
	Description:      "Collect sync operations (deployment history) from ArgoCD API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CICD},
	DependencyTables: []string{models.ArgocdApplication{}.TableName()},
}

func CollectSyncOperations(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*ArgocdTaskData)

	collector, err := api.NewStatefulApiCollector(api.RawDataSubTaskArgs{
		Ctx:   taskCtx,
		Table: RAW_SYNC_OPERATION_TABLE,
		Params: models.ArgocdApiParams{
			ConnectionId: data.Options.ConnectionId,
			Name:         data.Options.ApplicationName,
		},
	})

	if err != nil {
		return err
	}

	err = collector.InitCollector(api.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		UrlTemplate: "/applications/{{.Params.Name}}",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("refresh", "false")
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var appResponse struct {
				Status struct {
					Health struct {
						Status string `json:"status"`
					} `json:"health"`
					History        []json.RawMessage `json:"history"`
					OperationState json.RawMessage   `json:"operationState"`
				} `json:"status"`
			}

			err := api.UnmarshalResponse(res, &appResponse)
			if err != nil {
				return nil, err
			}

			recordHealthOnLatestSync(appResponse.Status.OperationState, appResponse.Status.History, appResponse.Status.Health.Status)
			return sanitizeOperationEntries(appResponse.Status.OperationState, appResponse.Status.History), nil
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}

// recordHealthOnLatestSync stores the application's current health on the newest
// history entry. Argo CD only reports health for the application as a whole, so
// this is the one moment the value can be tied to a sync. When the operation in
// progress failed or is still running, that newer sync owns the current health
// and the last history entry is left untouched.
func recordHealthOnLatestSync(operationState json.RawMessage, history []json.RawMessage, health string) {
	if health == "" {
		return
	}
	var operation struct {
		Phase string `json:"phase"`
	}
	if err := json.Unmarshal(operationState, &operation); err == nil && operation.Phase != "" && !isRedundantOperationState(operation.Phase) {
		return
	}
	for i := len(history) - 1; i >= 0; i-- {
		var entry map[string]json.RawMessage
		if err := json.Unmarshal(history[i], &entry); err != nil || entry == nil {
			continue
		}
		entry["applicationHealth"], _ = json.Marshal(health)
		if tagged, err := json.Marshal(entry); err == nil {
			history[i] = tagged
		}
		return
	}
}

func sanitizeOperationEntries(operationState json.RawMessage, history []json.RawMessage) []json.RawMessage {
	results := make([]json.RawMessage, 0, len(history)+1)

	if op := bytes.TrimSpace(operationState); len(op) > 0 && !bytes.Equal(op, []byte("null")) {
		results = append(results, operationState)
	}

	for _, entry := range history {
		if h := bytes.TrimSpace(entry); len(h) == 0 || bytes.Equal(h, []byte("null")) {
			continue
		}
		results = append(results, entry)
	}

	return results
}
