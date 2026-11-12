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
	"net/http"
	"net/url"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/pluginhelper/api"
)

// asanaPageSize is the largest page Asana list endpoints return.
const asanaPageSize = 100

// setAsanaPageQuery sets the page size and, after the first page, the offset
// that the previous page returned in next_page.
func setAsanaPageQuery(query url.Values, reqData *api.RequestData) {
	query.Set("limit", fmt.Sprintf("%d", reqData.Pager.Size))
	if offset, ok := reqData.CustomData.(string); ok && offset != "" {
		query.Set("offset", offset)
	}
}

// getNextAsanaPageOffset returns the offset of the next page. Asana sets
// next_page to null when there are no more pages, so the collection ends
// there: requesting a page without an offset would start again at page 1.
func getNextAsanaPageOffset(_ *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
	var resp asanaListResponse
	err := api.UnmarshalResponse(prevPageResponse, &resp)
	if err != nil {
		return nil, err
	}
	if resp.NextPage == nil || resp.NextPage.Offset == "" {
		return nil, api.ErrFinishCollect
	}
	return resp.NextPage.Offset, nil
}
