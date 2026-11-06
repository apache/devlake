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
	"testing"

	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A deleted issue comes back as an empty `issue` object: the collector
// skips its "Entity not found: Issue" error, but must not then fail on the
// missing page info.
func TestPageInfo_DeletedIssueEndsPagination(t *testing.T) {
	comments := commentsPageInfo(&GraphqlQueryCommentWrapper{})
	require.NotNil(t, comments)
	assert.False(t, comments.HasNextPage)

	history := historyPageInfo(&GraphqlQueryHistoryWrapper{})
	require.NotNil(t, history)
	assert.False(t, history.HasNextPage)
}

func TestPageInfo_KeepsLinearPagination(t *testing.T) {
	query := &GraphqlQueryCommentWrapper{}
	query.Issue.Comments.PageInfo = &helper.GraphqlQueryPageInfo{EndCursor: "c1", HasNextPage: true}

	pageInfo := commentsPageInfo(query)
	assert.True(t, pageInfo.HasNextPage)
	assert.Equal(t, "c1", pageInfo.EndCursor)
}
