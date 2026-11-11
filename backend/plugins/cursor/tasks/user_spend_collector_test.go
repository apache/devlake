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
	"io"
	"net/http"
	"strings"
	"testing"

	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

func spendResponse(body string) *http.Response {
	return &http.Response{Body: io.NopCloser(strings.NewReader(body))}
}

func TestNextSpendPageFollowsTotalPages(t *testing.T) {
	// POST /teams/spend reports pagination as top-level totalPages/totalMembers,
	// without the pagination.hasNextPage object used by filtered-usage-events.
	body := `{"teamMemberSpend":[{"userId":1}],"subscriptionCycleStart":1708992000000,"totalMembers":250,"totalPages":3}`

	next, err := nextSpendPage(&helper.RequestData{Pager: &helper.Pager{Page: 1, Size: 100}}, spendResponse(body))
	assert.Nil(t, err)
	assert.Equal(t, cursorPageState{Page: 2}, next)

	next, err = nextSpendPage(&helper.RequestData{CustomData: cursorPageState{Page: 2}, Pager: &helper.Pager{Page: 2, Size: 100}}, spendResponse(body))
	assert.Nil(t, err)
	assert.Equal(t, cursorPageState{Page: 3}, next)
}

func TestNextSpendPageStopsOnLastPage(t *testing.T) {
	body := `{"teamMemberSpend":[{"userId":1}],"subscriptionCycleStart":1708992000000,"totalMembers":250,"totalPages":3}`
	_, err := nextSpendPage(&helper.RequestData{CustomData: cursorPageState{Page: 3}, Pager: &helper.Pager{Page: 3, Size: 100}}, spendResponse(body))
	assert.Equal(t, helper.ErrFinishCollect, err)

	single := `{"teamMemberSpend":[{"userId":1}],"subscriptionCycleStart":1708992000000,"totalMembers":15,"totalPages":1}`
	_, err = nextSpendPage(&helper.RequestData{Pager: &helper.Pager{Page: 1, Size: 100}}, spendResponse(single))
	assert.Equal(t, helper.ErrFinishCollect, err)

	empty := `{"teamMemberSpend":[],"subscriptionCycleStart":1708992000000,"totalMembers":0,"totalPages":0}`
	_, err = nextSpendPage(&helper.RequestData{Pager: &helper.Pager{Page: 1, Size: 100}}, spendResponse(empty))
	assert.Equal(t, helper.ErrFinishCollect, err)
}
