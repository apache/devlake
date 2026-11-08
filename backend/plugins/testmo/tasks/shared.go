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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/testmo/models"
)

func DecodeTaskOptions(options map[string]interface{}) (*TestmoOptions, errors.Error) {
	var op TestmoOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, err
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New("connectionId is invalid")
	}
	return &op, nil
}

func CreateTestmoApiClient(taskCtx plugin.TaskContext, connection *models.TestmoConnection) (*helper.ApiAsyncClient, errors.Error) {
	return CreateApiClient(taskCtx, connection)
}

// GetTotalPagesFromResponse reads the page count of a Testmo list response.
// Testmo returns the pagination fields (page, prev_page, next_page,
// last_page, per_page, total) at the top level of the body, not in a meta
// object; last_page is nullable.
func GetTotalPagesFromResponse(res *http.Response, args *helper.ApiCollectorArgs) (int, errors.Error) {
	body := &struct {
		LastPage *int `json:"last_page"`
	}{}
	err := helper.UnmarshalResponse(res, body)
	if err != nil {
		return 0, err
	}
	if body.LastPage != nil && *body.LastPage > 1 {
		return *body.LastPage, nil
	}
	return 1, nil
}

func GetRawMessageFromResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	// Read the response body first
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to read response body")
	}
	defer res.Body.Close()

	// Try to parse as a general JSON object first
	var genericResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &genericResponse); err != nil {
		return nil, errors.Default.Wrap(err, "failed to unmarshal response")
	}

	var results []json.RawMessage

	// Check for "result" field (single item response - Testmo format)
	if result, hasResult := genericResponse["result"]; hasResult && result != nil {
		// Check if result is an array (for list endpoints)
		if resultArray, ok := result.([]interface{}); ok {
			for _, item := range resultArray {
				if rawBytes, err := json.Marshal(item); err == nil {
					results = append(results, rawBytes)
				}
			}
		} else {
			// Single item in result field
			if rawBytes, err := json.Marshal(result); err == nil {
				results = append(results, rawBytes)
			}
		}
		return results, nil
	}

	// Check for "results" field (list response - Testmo format)
	if resultsList, hasResults := genericResponse["results"]; hasResults {
		if resultsArray, ok := resultsList.([]interface{}); ok {
			for _, item := range resultsArray {
				if rawBytes, err := json.Marshal(item); err == nil {
					results = append(results, rawBytes)
				}
			}
		}
		return results, nil
	}

	// Fallback: Try standard DevLake format with "data" field
	if data, hasData := genericResponse["data"]; hasData {
		if dataArray, ok := data.([]interface{}); ok {
			// Array in data field
			for _, item := range dataArray {
				if rawBytes, err := json.Marshal(item); err == nil {
					results = append(results, rawBytes)
				}
			}
		} else if data != nil {
			// Single item in data field
			if rawBytes, err := json.Marshal(data); err == nil {
				results = append(results, rawBytes)
			}
		}
		return results, nil
	}

	// If no recognized structure, return empty results
	return results, nil
}

func GetQuery(reqData *helper.RequestData) (url.Values, errors.Error) {
	query := url.Values{}
	query.Set("page", fmt.Sprintf("%v", reqData.Pager.Page))
	query.Set("per_page", fmt.Sprintf("%v", reqData.Pager.Size))
	return query, nil
}
