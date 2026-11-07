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

// taigaPageSize is the largest page size Taiga accepts (MAX_PAGINATE_BY).
const taigaPageSize = 1000

// taigaProjectListQuery builds the query for a Taiga list endpoint filtered by
// project. Taiga paginates list endpoints (30 items per page by default), so
// the page and page size are always sent.
func taigaProjectListQuery(projectId uint64) func(reqData *api.RequestData) (url.Values, errors.Error) {
	return func(reqData *api.RequestData) (url.Values, errors.Error) {
		query := url.Values{}
		query.Set("project", fmt.Sprintf("%d", projectId))
		query.Set("page", fmt.Sprintf("%d", reqData.Pager.Page))
		query.Set("page_size", fmt.Sprintf("%d", reqData.Pager.Size))
		return query, nil
	}
}

// getNextTaigaPage stops the collection when Taiga reports no next page.
// Taiga answers a page past the last one with 404, so a full last page must
// not lead to another request.
func getNextTaigaPage(_ *api.RequestData, prevPageResponse *http.Response) (interface{}, errors.Error) {
	if prevPageResponse.Header.Get("X-Pagination-Next") == "" {
		return nil, api.ErrFinishCollect
	}
	return nil, nil
}
