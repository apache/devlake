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
	"net/url"
	"strconv"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/tempo/models"
)

var CollectWorklogsMeta = plugin.SubTaskMeta{
	Name:             "collect_worklogs",
	EntryPoint:       CollectWorklogs,
	EnabledByDefault: true,
	Description:      "Collect worklogs from Tempo API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_TICKET},
}

func CollectWorklogs(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*TempoTaskData)

	// Scope raw data and collector state per team, like the scope itself
	// (TempoTeam.GetParams): otherwise every team scope of a connection shares
	// one collector state, and the second team collected in a pipeline would
	// run incrementally from the first team's start time.
	rawDataSubTaskArgs := api.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: models.TempoApiParams{
			ConnectionId: data.Options.ConnectionId,
			TeamId:       data.Options.TeamId,
		},
		Table: RAW_WORKLOG_TABLE,
	}
	apiCollector, err := api.NewStatefulApiCollector(rawDataSubTaskArgs)
	if err != nil {
		return err
	}

	urlTemplate := "worklogs"
	if data.Options.TeamId != 0 {
		urlTemplate = fmt.Sprintf("worklogs/team/%d", data.Options.TeamId)
	}

	err = apiCollector.InitCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: rawDataSubTaskArgs,
		ApiClient:          data.ApiClient,
		UrlTemplate:        urlTemplate,
		PageSize:           1000,
		// No GetTotalPages: Tempo v4 pagination metadata (PageableMetadata)
		// has count/offset/limit/next/previous but no total, so pages are
		// fetched until one comes back short.
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			return buildWorklogQuery(data.Options, reqData.Pager,
				apiCollector.IsIncremental(), apiCollector.GetSince()), nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var response struct {
				Results []json.RawMessage `json:"results"`
			}
			err := api.UnmarshalResponse(res, &response)
			if err != nil {
				return nil, err
			}
			return response.Results, nil
		},
	})

	if err != nil {
		return err
	}

	return apiCollector.Execute()
}

// buildWorklogQuery builds the query for one page of worklogs. Explicit
// fromDate/toDate options win; otherwise an incremental run asks for what
// changed since the last successful collection (updatedFrom) and a full sync
// starts at the sync policy's timeAfter (from).
func buildWorklogQuery(opts *TempoOptions, pager *api.Pager, incremental bool, since *time.Time) url.Values {
	if pager == nil {
		pager = &api.Pager{Page: 1, Skip: 0, Size: 1000}
	}
	query := url.Values{}
	query.Set("offset", strconv.Itoa(pager.Skip))
	query.Set("limit", strconv.Itoa(pager.Size))

	switch {
	case opts.FromDate != "" || opts.ToDate != "":
		if opts.FromDate != "" {
			query.Set("from", opts.FromDate)
		}
		if opts.ToDate != "" {
			query.Set("to", opts.ToDate)
		}
	case since != nil && incremental:
		query.Set("updatedFrom", since.UTC().Format(time.RFC3339))
	case since != nil:
		query.Set("from", since.UTC().Format("2006-01-02"))
	}
	return query
}
