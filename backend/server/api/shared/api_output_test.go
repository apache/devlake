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

package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apache/devlake/core/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestApiOutputErrorMapsUpstream401(t *testing.T) {
	tests := []struct {
		upstream int
		want     int
	}{
		{http.StatusUnauthorized, http.StatusBadGateway},
		{http.StatusNotFound, http.StatusNotFound},
	}
	for _, tt := range tests {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ApiOutputError(c, errors.Default.Wrap(errors.HttpStatus(tt.upstream).New("upstream"), "wrapped"))
		assert.Equal(t, tt.want, w.Code)
	}
}
