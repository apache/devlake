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
	"io"
	"net/http"
	"testing"

	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

func makeResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Request:    &http.Request{},
	}
}

func TestIgnoreInvalidTimelineResponse_404(t *testing.T) {
	res := makeResponse(http.StatusNotFound, "")
	err := ignoreInvalidTimelineResponse(res)
	assert.Equal(t, api.ErrIgnoreAndContinue, err, "404 should return ErrIgnoreAndContinue")
}

func TestIgnoreInvalidTimelineResponse_EmptyBody(t *testing.T) {
	res := makeResponse(http.StatusOK, "")
	err := ignoreInvalidTimelineResponse(res)
	assert.Equal(t, api.ErrIgnoreAndContinue, err, "empty body should return ErrIgnoreAndContinue")
}

func TestIgnoreInvalidTimelineResponse_NonJSONBody(t *testing.T) {
	res := makeResponse(http.StatusOK, "not json at all")
	err := ignoreInvalidTimelineResponse(res)
	assert.Equal(t, api.ErrIgnoreAndContinue, err, "non-JSON body should return ErrIgnoreAndContinue")
}

func TestIgnoreInvalidTimelineResponse_ValidJSON_ReturnsNil(t *testing.T) {
	validJSON := `{"records":[{"id":"abc","type":"Job","name":"Build"}]}`
	res := makeResponse(http.StatusOK, validJSON)
	err := ignoreInvalidTimelineResponse(res)
	assert.Nil(t, err, "valid JSON body should return nil")

	// Verify body is still readable after the handler restored it.
	remaining, readErr := io.ReadAll(res.Body)
	assert.NoError(t, readErr)
	assert.Equal(t, validJSON, string(remaining), "body should be restored for downstream parser")
}

func TestIgnoreInvalidTimelineResponse_ValidEmptyJSONObject(t *testing.T) {
	res := makeResponse(http.StatusOK, "{}")
	err := ignoreInvalidTimelineResponse(res)
	assert.Nil(t, err, "valid empty JSON object should return nil")
}
