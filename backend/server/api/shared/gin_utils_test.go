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

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/gin-gonic/gin"
)

// TestGetUserReadsGinKeys covers the common, non-re-dispatched case where the
// user is set directly on the gin context.
func TestGetUserReadsGinKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set(common.USER, &common.User{Name: "alice"})

	user, ok := GetUser(c)
	if !ok || user == nil || user.Name != "alice" {
		t.Fatalf("GetUser() = %+v, %v; want alice, true", user, ok)
	}
}

// TestSetUserOnRequestSurvivesHandleContext is the regression test for the
// open-API key 401 bug: gin's Engine.HandleContext re-dispatch calls
// Context.reset(), which sets c.Keys = nil and therefore drops anything set
// via c.Set. The authenticated user must instead ride on the request's
// context.Context so the terminal RequireAuth gate can still see it after the
// /rest path-rewrite re-dispatch.
func TestSetUserOnRequestSurvivesHandleContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var (
		ginKeysVisible  bool
		requestVisible  bool
		reachedTerminal bool
	)

	router := gin.New()
	router.GET("/target", func(c *gin.Context) {
		reachedTerminal = true
		// gin Keys are wiped by reset() during HandleContext re-dispatch.
		if _, ok := c.Get(common.USER); ok {
			ginKeysVisible = true
		}
		// The user must survive on the request context.
		if user, ok := GetUser(c); ok && user != nil && user.Name == "bob" {
			requestVisible = true
		}
		c.Status(http.StatusOK)
	})

	// Entry handler mimics RestAuthentication: authenticate, stash the user,
	// rewrite the path, and re-dispatch through the engine.
	router.GET("/rest/target", func(c *gin.Context) {
		SetUserOnRequest(c, &common.User{Name: "bob"})
		c.Request.URL.Path = "/target"
		router.HandleContext(c)
		c.Abort()
	})

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/rest/target", nil)
	router.ServeHTTP(resp, req)

	if !reachedTerminal {
		t.Fatal("re-dispatched request never reached the terminal handler")
	}
	if ginKeysVisible {
		t.Error("gin Keys unexpectedly survived HandleContext; test no longer exercises the reset() bug")
	}
	if !requestVisible {
		t.Error("user did not survive HandleContext re-dispatch via request context (regression)")
	}
	if resp.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.Code, http.StatusOK)
	}
}
