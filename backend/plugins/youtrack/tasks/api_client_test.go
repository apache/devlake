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
	"net/http"
	"testing"

	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A deleted issue lingers in the tool layer, so the per-issue collectors
// must skip its 404 rather than fail the subtask (and freeze the bookmark).
func TestIgnoreHTTPStatus404(t *testing.T) {
	t.Run("404 is skipped, not failed", func(t *testing.T) {
		err := ignoreHTTPStatus404(&http.Response{StatusCode: http.StatusNotFound})
		assert.Equal(t, helper.ErrIgnoreAndContinue, err)
	})

	t.Run("401 still fails as an authentication error", func(t *testing.T) {
		err := ignoreHTTPStatus404(&http.Response{StatusCode: http.StatusUnauthorized})
		require.NotNil(t, err)
		assert.NotEqual(t, helper.ErrIgnoreAndContinue, err)
	})

	t.Run("other statuses pass through to the framework", func(t *testing.T) {
		for _, code := range []int{http.StatusOK, http.StatusBadRequest, http.StatusGatewayTimeout} {
			assert.Nil(t, ignoreHTTPStatus404(&http.Response{StatusCode: code}), "status %d", code)
		}
	})
}
