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
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keepTopLevelFields mimics Bitbucket's partial response: only the top-level
// keys named in `fields` (e.g. `values` for `values.hash`) are returned.
func keepTopLevelFields(t *testing.T, full string, fields string) []byte {
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(full), &body))
	keep := map[string]bool{}
	for _, f := range strings.Split(fields, ",") {
		keep[strings.SplitN(f, ".", 2)[0]] = true
	}
	for k := range body {
		if !keep[k] {
			delete(body, k)
		}
	}
	filtered, err := json.Marshal(body)
	require.NoError(t, err)
	return filtered
}

func TestGetPrCommitsQueryFollowsNextPage(t *testing.T) {
	query, err := getPrCommitsQuery(&api.RequestData{Pager: &api.Pager{Page: 1, Size: 100}})
	require.NoError(t, err)
	assert.Equal(t, "100", query.Get("pagelen"))
	assert.Empty(t, query.Get("page"))

	full := `{
		"values": [{"hash": "a50685cd98297edd1b248049f199396434726c97"}],
		"pagelen": 100,
		"next": "https://api.bitbucket.org/2.0/repositories/ws/repo/pullrequests/1/commits?pagelen=100&page=E8Ux"
	}`
	res := &http.Response{Body: io.NopCloser(bytes.NewReader(keepTopLevelFields(t, full, query.Get("fields"))))}
	next, err := GetNextPageCustomData(nil, res)
	require.NoError(t, err)
	assert.Equal(t, "E8Ux", next)

	query, err = getPrCommitsQuery(&api.RequestData{Pager: &api.Pager{Page: 2, Size: 100}, CustomData: next})
	require.NoError(t, err)
	assert.Equal(t, "E8Ux", query.Get("page"))
}
