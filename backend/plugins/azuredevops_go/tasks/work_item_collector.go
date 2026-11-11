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
	"fmt"
	"net/url"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
)

func init() {
	RegisterSubtaskMeta(&CollectApiWorkItemsMeta)
}

const RawWorkItemTable = "azuredevops_go_api_work_items"

var CollectApiWorkItemsMeta = plugin.SubTaskMeta{
	Name:             "collectApiWorkItems",
	EntryPoint:       CollectApiWorkItems,
	EnabledByDefault: true,
	Description:      "Collect Azure Boards work items with a WIQL query, then batch-load each work item.",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
	ProductTables:    []string{RawWorkItemTable},
}

func CollectApiWorkItems(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*AzuredevopsTaskData)
	rawArgs := &api.RawDataSubTaskArgs{
		Ctx:     taskCtx,
		Table:   RawWorkItemTable,
		Options: data.Options,
	}
	collector, err := api.NewStatefulApiCollector(*rawArgs)
	if err != nil {
		return err
	}

	var createdAfter, changedSince *time.Time
	if collector.IsIncremental() {
		changedSince = collector.GetSince()
	} else {
		createdAfter = collector.GetSince()
	}
	ids, err := queryWorkItemIds(data, createdAfter, changedSince)
	if err != nil {
		return err
	}
	iterator := api.NewQueueIterator()
	for _, chunk := range chunkInts(ids, 200) {
		iterator.Push(chunk)
	}

	err = collector.InitCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: *rawArgs,
		ApiClient:          data.ApiClient,
		Input:              iterator,
		UrlTemplate:        adoOrgPrefix(data) + "_apis/wit/workitems",
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("ids", joinInts(reqData.Input.([]int)))
			query.Set("$expand", "all")
			query.Set("api-version", "7.1")
			return query, nil
		},
		ResponseParser: ParseRawMessageFromValue,
		AfterResponse:  change203To401,
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}

func queryWorkItemIds(data *AzuredevopsTaskData, createdAfter, changedSince *time.Time) ([]int, errors.Error) {
	query := projectWorkItemsWIQL(data.Options.ProjectId, createdAfter, changedSince)
	path := fmt.Sprintf("%s/_apis/wit/wiql?api-version=7.1", adoScopePrefix(data))
	res, err := data.ApiClient.Post(path, nil, map[string]string{"query": query}, nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		WorkItems []struct {
			ID int `json:"id"`
		} `json:"workItems"`
	}
	if err = readJSON(res, &body); err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(body.WorkItems))
	for _, item := range body.WorkItems {
		if item.ID != 0 {
			ids = append(ids, item.ID)
		}
	}
	return ids, nil
}
