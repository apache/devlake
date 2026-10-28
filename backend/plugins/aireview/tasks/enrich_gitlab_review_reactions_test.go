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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGitlabDomainId(t *testing.T) {
	tests := []struct {
		name       string
		domainId   string
		wantConn   uint64
		wantGitlab int
		wantErr    bool
	}{
		{
			name:       "valid GitLab MR comment ID",
			domainId:   "gitlab:GitlabMrComment:1:123456",
			wantConn:   1,
			wantGitlab: 123456,
		},
		{
			name:       "valid with large IDs",
			domainId:   "gitlab:GitlabMrComment:42:999999999",
			wantConn:   42,
			wantGitlab: 999999999,
		},
		{
			name:     "too few parts",
			domainId: "gitlab:GitlabMrComment",
			wantErr:  true,
		},
		{
			name:     "invalid connection ID",
			domainId: "gitlab:GitlabMrComment:abc:123",
			wantErr:  true,
		},
		{
			name:     "invalid gitlab ID",
			domainId: "gitlab:GitlabMrComment:1:xyz",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connId, gitlabId, err := parseGitlabDomainId(tt.domainId)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantConn, connId)
			assert.Equal(t, tt.wantGitlab, gitlabId)
		})
	}
}

func TestCountReactionEmojis(t *testing.T) {
	tests := []struct {
		name           string
		emojis         []awardEmoji
		wantThumbsUp   int
		wantThumbsDown int
		wantTotal      int
	}{
		{
			name:           "empty",
			emojis:         nil,
			wantThumbsUp:   0,
			wantThumbsDown: 0,
			wantTotal:      0,
		},
		{
			name: "mixed reactions",
			emojis: []awardEmoji{
				{Name: "thumbsup"},
				{Name: "thumbsup"},
				{Name: "thumbsdown"},
				{Name: "heart"},
				{Name: "rocket"},
			},
			wantThumbsUp:   2,
			wantThumbsDown: 1,
			wantTotal:      5,
		},
		{
			name: "only thumbsup",
			emojis: []awardEmoji{
				{Name: "thumbsup"},
				{Name: "thumbsup"},
			},
			wantThumbsUp:   2,
			wantThumbsDown: 0,
			wantTotal:      2,
		},
		{
			name: "no thumbs reactions",
			emojis: []awardEmoji{
				{Name: "heart"},
				{Name: "eyes"},
			},
			wantThumbsUp:   0,
			wantThumbsDown: 0,
			wantTotal:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			up, down, total := countReactionEmojis(tt.emojis)
			assert.Equal(t, tt.wantThumbsUp, up)
			assert.Equal(t, tt.wantThumbsDown, down)
			assert.Equal(t, tt.wantTotal, total)
		})
	}
}

func TestFetchAwardEmojis(t *testing.T) {
	// Mock GitLab API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		if r.Header.Get("Private-Token") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify URL path
		expectedPath := "/projects/100/merge_requests/42/notes/999/award_emoji"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		emojis := []awardEmoji{
			{Name: "thumbsup"},
			{Name: "thumbsdown"},
			{Name: "thumbsup"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emojis)
	}))
	defer server.Close()

	client := server.Client()
	emojis, err := fetchAwardEmojis(context.Background(), client, server.URL, "test-token", 100, 42, 999)
	assert.NoError(t, err)
	assert.Len(t, emojis, 3)

	up, down, total := countReactionEmojis(emojis)
	assert.Equal(t, 2, up)
	assert.Equal(t, 1, down)
	assert.Equal(t, 3, total)
}

func TestFetchAwardEmojis_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"401 Unauthorized"}`))
	}))
	defer server.Close()

	client := server.Client()
	_, err := fetchAwardEmojis(context.Background(), client, server.URL, "bad-token", 100, 42, 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestFetchAwardEmojis_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := server.Client()
	emojis, err := fetchAwardEmojis(context.Background(), client, server.URL, "token", 100, 42, 999)
	assert.NoError(t, err)
	assert.Empty(t, emojis)
}
