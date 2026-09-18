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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString("")),
		Request:    &http.Request{},
	}
}

func TestIgnoreHTTPStatus404(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantIgnore bool
		wantUnauth bool
		wantNil    bool
	}{
		{
			name:       "404 returns ErrIgnoreAndContinue",
			statusCode: http.StatusNotFound,
			wantIgnore: true,
		},
		{
			name:       "401 returns unauthorized error",
			statusCode: http.StatusUnauthorized,
			wantUnauth: true,
		},
		{
			name:       "200 returns nil",
			statusCode: http.StatusOK,
			wantNil:    true,
		},
		{
			name:       "403 returns nil",
			statusCode: http.StatusForbidden,
			wantNil:    true,
		},
		{
			name:       "500 returns nil",
			statusCode: http.StatusInternalServerError,
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := makeTestResponse(tt.statusCode)
			err := ignoreHTTPStatus404(res)
			if tt.wantIgnore {
				assert.Equal(t, api.ErrIgnoreAndContinue, err)
			} else if tt.wantUnauth {
				require.NotNil(t, err)
				assert.Equal(t, errors.Unauthorized, err.GetType())
			} else if tt.wantNil {
				assert.Nil(t, err)
			}
		})
	}
}

func TestIgnoreHTTPStatus422(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantIgnore bool
		wantNil    bool
	}{
		{
			name:       "422 returns ErrIgnoreAndContinue",
			statusCode: http.StatusUnprocessableEntity,
			wantIgnore: true,
		},
		{
			name:       "200 returns nil",
			statusCode: http.StatusOK,
			wantNil:    true,
		},
		{
			name:       "404 returns nil",
			statusCode: http.StatusNotFound,
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := makeTestResponse(tt.statusCode)
			err := ignoreHTTPStatus422(res)
			if tt.wantIgnore {
				assert.Equal(t, api.ErrIgnoreAndContinue, err)
			} else if tt.wantNil {
				assert.Nil(t, err)
			}
		})
	}
}
