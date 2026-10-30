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
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/didgen"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/azuredevops_go/models"
)

func projectBoardID(data *AzuredevopsTaskData) string {
	repo := models.AzuredevopsRepo{Id: data.Options.RepositoryId, Type: data.Options.RepositoryType}
	repo.OrganizationId = data.Options.OrganizationId
	repo.ProjectId = data.Options.ProjectId
	return didgen.NewDomainIdGenerator(&models.AzuredevopsRepo{}).Generate(data.Options.ConnectionId, repo.ProjectDomainScopeID())
}

func adoScopePrefix(data *AzuredevopsTaskData) string {
	org := url.PathEscape(data.Options.OrganizationId)
	project := url.PathEscape(data.Options.ProjectId)
	if org != "" {
		return org + "/" + project
	}
	return project
}

func adoOrgPrefix(data *AzuredevopsTaskData) string {
	if data.Options.OrganizationId == "" {
		return ""
	}
	return url.PathEscape(data.Options.OrganizationId) + "/"
}

func chunkInts(ids []int, size int) [][]int {
	if size <= 0 || len(ids) == 0 {
		return nil
	}
	chunks := make([][]int, 0, (len(ids)+size-1)/size)
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[start:end])
	}
	return chunks
}

func joinInts(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprint(id)
	}
	return strings.Join(parts, ",")
}

func readJSON(res *http.Response, target interface{}) errors.Error {
	if res == nil {
		return errors.Default.New("empty Azure DevOps response")
	}
	if res.StatusCode == http.StatusNonAuthoritativeInfo || res.StatusCode == http.StatusUnauthorized {
		return errors.Unauthorized.New("authentication failed, please check your AccessToken")
	}
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return errors.Default.New(fmt.Sprintf("Azure DevOps API returned %d for %s: %s", res.StatusCode, res.Request.URL.String(), string(body)))
	}
	return api.UnmarshalResponse(res, target)
}

type continuationBody struct {
	Value             []json.RawMessage `json:"value"`
	ContinuationToken string            `json:"continuationToken"`
}

func collectContinuation(rawArgs *api.RawDataSubTaskArgs, client api.RateLimitedApiClient, urlTemplate string, query func(reqData *api.RequestData) (url.Values, errors.Error)) errors.Error {
	var token string
	collector, err := api.NewStatefulApiCollector(*rawArgs)
	if err != nil {
		return err
	}
	err = collector.InitCollector(api.ApiCollectorArgs{
		RawDataSubTaskArgs: *rawArgs,
		ApiClient:          client,
		PageSize:           1,
		UrlTemplate:        urlTemplate,
		Query: func(reqData *api.RequestData) (url.Values, errors.Error) {
			values, queryErr := query(reqData)
			if queryErr != nil {
				return nil, queryErr
			}
			if values == nil {
				values = url.Values{}
			}
			if reqData.CustomData != nil {
				values.Set("continuationToken", fmt.Sprint(reqData.CustomData))
			}
			return values, nil
		},
		GetNextPageCustomData: func(prevReqData *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
			if token == "" {
				return nil, api.ErrFinishCollect
			}
			next := token
			token = ""
			return next, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			body := &continuationBody{}
			parseErr := api.UnmarshalResponse(res, body)
			if parseErr != nil {
				return nil, parseErr
			}
			token = body.ContinuationToken
			if token == "" && res != nil {
				token = res.Header.Get("X-Ms-Continuationtoken")
			}
			return body.Value, nil
		},
		AfterResponse: change203To401,
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
