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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apache/devlake/plugins/aisdlc/tasks/parse"
)

func TestListBlobsPaginatesAndFilters(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.Contains(r.URL.Path, "/repository/tree") && r.URL.Query().Get("ref") != "a1b2c3d4" {
			t.Errorf("tree ref = %q", r.URL.Query().Get("ref"))
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("X-Next-Page", "2")
			_, _ = w.Write([]byte(`[
				{"id":"b1","path":"RHAIRFE/20260719-041307/RHAIRFE-1.result.md","type":"blob","mode":"100644"},
				{"id":"b2","path":"RHAIRFE/20260719-041307/scores.csv","type":"blob","mode":"100644"}
			]`))
			return
		}
		w.Header().Set("X-Next-Page", "")
		_, _ = w.Write([]byte(`[
			{"id":"b3","path":"RHAIRFE/current/RHAIRFE-2.result.md","type":"blob","mode":"120000"},
			{"id":"b4","path":"RHAIRFE/20261020-041115/RHAIRFE-2.result.md","type":"blob","mode":"100644"}
		]`))
	}))
	defer server.Close()

	client, err := NewGitlabClient(context.Background(), server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := client.ListBlobs(context.Background(), "group/repo", "a1b2c3d4", "", parse.ParserRFE)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestNewGitlabClientFallsBackToPrivateToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("PRIVATE-TOKEN") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, err := NewGitlabClient(context.Background(), server.URL, "private")
	if err != nil {
		t.Fatal(err)
	}
	if client.Header.Get("PRIVATE-TOKEN") != "private" {
		t.Fatalf("header = %v", client.Header)
	}
}
