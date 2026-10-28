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
	"fmt"
	"net/http"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

var CollectFlagsMeta = plugin.SubTaskMeta{
	Name:             "CollectFlags",
	EntryPoint:       CollectFlags,
	EnabledByDefault: true,
	Description:      "Collect flags data from Codecov API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

func CollectFlags(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	// Extract owner and repo from FullName (format: "owner/repo")
	owner, repo, err := ParseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_FLAGS_TABLE,
		},
		Incremental: true, // ALWAYS preserve historical data
		ApiClient:   data.ApiClient,
		UrlTemplate: fmt.Sprintf("%s/flags", RepoAPIPrefix(data.Service, owner, repo)),
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var response struct {
				Results []json.RawMessage `json:"results"`
			}
			err := helper.UnmarshalResponse(res, &response)
			if err != nil {
				return nil, err
			}
			return response.Results, nil
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}

