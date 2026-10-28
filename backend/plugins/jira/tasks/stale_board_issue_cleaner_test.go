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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/core/plugin"
	helperapi "github.com/apache/devlake/helpers/pluginhelper/api"
	mockdal "github.com/apache/devlake/mocks/core/dal"
	mocklog "github.com/apache/devlake/mocks/core/log"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/apache/devlake/plugins/jira/models"
)

// buildTestTaskData creates a minimal JiraTaskData pointing ApiClient at the given server URL.
func buildTestTaskData(serverURL string, connectionId, boardId uint64) *JiraTaskData {
	apiClient := &helperapi.ApiClient{}
	apiClient.Setup(serverURL, nil, 10*time.Second)
	return &JiraTaskData{
		Options: &JiraOptions{
			ConnectionId: connectionId,
			BoardId:      boardId,
		},
		ApiClient: &helperapi.ApiAsyncClient{ApiClient: apiClient},
	}
}

// boardResponse is the JSON shape returned by agile/1.0/board/:id/issue.
type boardResponse struct {
	Total  int              `json:"total"`
	Issues []boardRespIssue `json:"issues"`
}

type boardRespIssue struct {
	Key string `json:"key"`
}

func TestFetchBoardMembership_EmptyIssues(t *testing.T) {
	// No issues to check → no HTTP calls needed, should return an empty map.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected HTTP call with empty issue list")
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, nil)
	assert.NoError(t, err)
	assert.NotNil(t, onBoard)
	assert.Empty(t, onBoard)
}

func TestFetchBoardMembership_AllOnBoard(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2},
		{"PROJ-3", 3},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := boardResponse{
			Total: 3,
			Issues: []boardRespIssue{
				{Key: "PROJ-1"},
				{Key: "PROJ-2"},
				{Key: "PROJ-3"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"PROJ-1": true,
		"PROJ-2": true,
		"PROJ-3": true,
	}, onBoard)
}

func TestFetchBoardMembership_SomeNotOnBoard(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2}, // will be missing from API response
		{"PROJ-3", 3},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only PROJ-1 and PROJ-3 are on the board.
		resp := boardResponse{
			Total:  2,
			Issues: []boardRespIssue{{Key: "PROJ-1"}, {Key: "PROJ-3"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"PROJ-1": true,
		"PROJ-3": true,
	}, onBoard)
}

func TestFetchBoardMembership_404ReturnsNil(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Nil(t, onBoard)
}

func TestFetchBoardMembership_NonOKStatusReturnsError(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	_, err := fetchBoardMembership(data, 42, issues)
	assert.Error(t, err)
}

func TestFetchBoardMembership_PaginationFetchesAllPages(t *testing.T) {
	// API returns 3 issues total but only 2 per page (short of maxResults=100).
	// Next startAt must be 2, not 100 — otherwise PROJ-3 is skipped and treated as off-board.
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2},
		{"PROJ-3", 3},
	}

	var startAts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		startAts = append(startAts, startAt)
		var resp boardResponse
		if startAt == "0" || startAt == "" {
			resp = boardResponse{
				Total:  3,
				Issues: []boardRespIssue{{Key: "PROJ-1"}, {Key: "PROJ-2"}},
			}
		} else if startAt == "2" {
			resp = boardResponse{
				Total:  3,
				Issues: []boardRespIssue{{Key: "PROJ-3"}},
			}
		} else {
			http.Error(w, "unexpected startAt: "+startAt, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Equal(t, []string{"0", "2"}, startAts)
	assert.Equal(t, map[string]bool{
		"PROJ-1": true,
		"PROJ-2": true,
		"PROJ-3": true,
	}, onBoard)
}

func TestFetchBoardMembership_ShortPageDoesNotSkipIssues(t *testing.T) {
	// Jira often caps a page below maxResults (e.g. 50 of 80). Stepping startAt by 100
	// would skip keys 50–79 and delete their board associations.
	issues := make([]struct {
		IssueKey string
		IssueId  uint64
	}, 80)
	for i := range issues {
		issues[i] = struct {
			IssueKey string
			IssueId  uint64
		}{fmt.Sprintf("PROJ-%d", i+1), uint64(i + 1)}
	}

	var startAts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		startAts = append(startAts, startAt)
		var from, to int
		switch startAt {
		case "0", "":
			from, to = 0, 50
		case "50":
			from, to = 50, 80
		default:
			http.Error(w, "skipped issues; startAt="+startAt, http.StatusBadRequest)
			return
		}
		respIssues := make([]boardRespIssue, 0, to-from)
		for i := from; i < to; i++ {
			respIssues = append(respIssues, boardRespIssue{Key: fmt.Sprintf("PROJ-%d", i+1)})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(boardResponse{Total: 80, Issues: respIssues})
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Equal(t, []string{"0", "50"}, startAts)
	assert.Len(t, onBoard, 80)
	assert.True(t, onBoard["PROJ-51"])
	assert.True(t, onBoard["PROJ-80"])
}

func TestFetchBoardMembership_EmptyPageWithZeroTotalMeansNoneOnBoard(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := boardResponse{Total: 0, Issues: []boardRespIssue{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.NotNil(t, onBoard)
	assert.Empty(t, onBoard)
}

func TestFetchBoardMembership_EmptyPageWithTotalReturnsError(t *testing.T) {
	// Permission filtering / inconsistent API: total>0 but issues=[].
	// Must error (not return an empty map) so cleanup does not delete valid associations.
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2},
	}

	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		resp := boardResponse{
			Total:  5,
			Issues: []boardRespIssue{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.Error(t, err)
	assert.Nil(t, onBoard)
	assert.Equal(t, 1, requestCount)
	assert.Contains(t, err.Error(), "incomplete board membership")
}

func TestFetchBoardMembership_EmptyLaterPageDoesNotReturnPartialMap(t *testing.T) {
	// Full first page (100) with total still higher; next page (startAt=100) is empty.
	issues := make([]struct {
		IssueKey string
		IssueId  uint64
	}, 1)
	issues[0] = struct {
		IssueKey string
		IssueId  uint64
	}{"PROJ-1", 1}

	page1 := make([]boardRespIssue, staleBoardIssueCheckBatchSize)
	for i := range page1 {
		page1[i] = boardRespIssue{Key: fmt.Sprintf("PROJ-%d", i+1)}
	}

	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		requestCount++
		var resp boardResponse
		if startAt == "0" || startAt == "" {
			resp = boardResponse{Total: 150, Issues: page1}
		} else {
			resp = boardResponse{Total: 150, Issues: []boardRespIssue{}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.Error(t, err)
	assert.Nil(t, onBoard)
	assert.Equal(t, 2, requestCount)
}

func TestFetchBoardMembership_BatchesBigIssueListIntoMultipleAPIRequests(t *testing.T) {
	// Build 150 issues — more than staleBoardIssueCheckBatchSize (100).
	// Should result in 2 separate API batch requests with different JQL sets.
	issues := make([]struct {
		IssueKey string
		IssueId  uint64
	}, 150)
	for i := range issues {
		issues[i] = struct {
			IssueKey string
			IssueId  uint64
		}{fmt.Sprintf("PROJ-%d", i+1), uint64(i + 1)}
	}

	jqlBatches := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jql := r.URL.Query().Get("jql")
		if r.URL.Query().Get("startAt") == "0" || r.URL.Query().Get("startAt") == "" {
			jqlBatches = append(jqlBatches, jql)
		}
		// Return all requested keys as "on board" so pagination terminates immediately.
		parts := strings.TrimPrefix(jql, "issue IN (")
		parts = strings.TrimSuffix(parts, ")")
		keys := strings.Split(parts, ",")
		respIssues := make([]boardRespIssue, 0, len(keys))
		for _, k := range keys {
			respIssues = append(respIssues, boardRespIssue{Key: k})
		}
		resp := boardResponse{Total: len(respIssues), Issues: respIssues}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Len(t, jqlBatches, 2)
	assert.Contains(t, jqlBatches[0], "PROJ-1")
	assert.Contains(t, jqlBatches[0], "PROJ-100")
	assert.NotContains(t, jqlBatches[0], "PROJ-101")
	assert.Contains(t, jqlBatches[1], "PROJ-101")
	assert.Contains(t, jqlBatches[1], "PROJ-150")
	assert.Len(t, onBoard, 150)
}

type jiraPluginStub struct{}

func (jiraPluginStub) Name() string { return "jira" }
func (jiraPluginStub) RootPkgPath() string {
	return "github.com/apache/devlake/plugins/jira"
}
func (jiraPluginStub) Description() string { return "" }

var registerJiraOnce sync.Once

func registerJiraPlugin(t *testing.T) {
	t.Helper()
	registerJiraOnce.Do(func() {
		_ = plugin.RegisterPlugin("jira", jiraPluginStub{})
	})
}

type boardIssueRow struct {
	IssueKey string
	IssueId  uint64
}

func setupCleanupMocks(t *testing.T, data *JiraTaskData) (*mockplugin.SubTaskContext, *mockdal.Dal, *mockdal.Transaction) {
	t.Helper()
	registerJiraPlugin(t)

	ctx := new(mockplugin.SubTaskContext)
	db := new(mockdal.Dal)
	logger := new(mocklog.Logger)
	tx := new(mockdal.Transaction)

	ctx.On("GetData").Return(data)
	ctx.On("GetDal").Return(db)
	ctx.On("GetLogger").Return(logger)
	logger.On("Info", mock.Anything, mock.Anything).Maybe()
	logger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	logger.On("Debug", mock.Anything, mock.Anything).Maybe()

	return ctx, db, tx
}

func stubBoardIssues(db *mockdal.Dal, rows []boardIssueRow, allErr errors.Error) {
	if allErr != nil {
		db.On("All", mock.Anything, mock.Anything).Return(allErr)
		return
	}
	db.On("All", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*[]struct {
			IssueKey string
			IssueId  uint64
		})
		out := make([]struct {
			IssueKey string
			IssueId  uint64
		}, len(rows))
		for i, r := range rows {
			out[i] = struct {
				IssueKey string
				IssueId  uint64
			}{r.IssueKey, r.IssueId}
		}
		*dest = out
	}).Return(nil)
}

func newCleanupHTTPServer(boardStatus int, onBoardKeys []string, issueStatus int) *httptest.Server {
	if boardStatus == 0 {
		boardStatus = http.StatusOK
	}
	if issueStatus == 0 {
		issueStatus = http.StatusNotFound
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/board/") {
			if boardStatus != http.StatusOK {
				w.WriteHeader(boardStatus)
				return
			}
			issues := make([]boardRespIssue, 0, len(onBoardKeys))
			for _, k := range onBoardKeys {
				issues = append(issues, boardRespIssue{Key: k})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(boardResponse{Total: len(issues), Issues: issues})
			return
		}
		w.WriteHeader(issueStatus)
	}))
}

func TestCleanupStaleBoardIssues(t *testing.T) {
	tests := []struct {
		name          string
		allErr        errors.Error
		boardIssues   []boardIssueRow
		boardStatus   int
		onBoardKeys   []string
		issueStatus   int
		deleteFailAt  int // 1 = tool-layer delete, 2 = domain-layer delete
		commitErr     errors.Error
		wantErr       bool
		wantBegins    int
		wantDeletes   int
		wantCommits   int
		wantRollbacks int
	}{
		{
			name:    "All query fails",
			allErr:  errors.Default.New("db down"),
			wantErr: true,
		},
		{
			name:        "no board issues",
			boardIssues: nil,
		},
		{
			name:        "board 404 skips cleanup",
			boardIssues: []boardIssueRow{{"PROJ-1", 1}},
			boardStatus: http.StatusNotFound,
		},
		{
			name:        "board API 500",
			boardIssues: []boardIssueRow{{"PROJ-1", 1}},
			boardStatus: http.StatusInternalServerError,
			wantErr:     true,
		},
		{
			name:        "all issues still on board",
			boardIssues: []boardIssueRow{{"PROJ-1", 1}, {"PROJ-2", 2}},
			onBoardKeys: []string{"PROJ-1", "PROJ-2"},
		},
		{
			name:        "stale issue deleted from both tables",
			boardIssues: []boardIssueRow{{"PROJ-1", 1}, {"PROJ-2", 2}},
			onBoardKeys: []string{"PROJ-1"},
			wantBegins:  1,
			wantDeletes: 2,
			wantCommits: 1,
		},
		{
			name:        "issue re-fetch failure still deletes",
			boardIssues: []boardIssueRow{{"PROJ-2", 2}},
			issueStatus: http.StatusInternalServerError,
			wantBegins:  1,
			wantDeletes: 2,
			wantCommits: 1,
		},
		{
			name:          "tool-layer delete failure rolls back",
			boardIssues:   []boardIssueRow{{"PROJ-1", 1}},
			deleteFailAt:  1,
			wantBegins:    1,
			wantDeletes:   1,
			wantRollbacks: 1,
		},
		{
			name:          "domain-layer delete failure rolls back",
			boardIssues:   []boardIssueRow{{"PROJ-1", 1}},
			deleteFailAt:  2,
			wantBegins:    1,
			wantDeletes:   2,
			wantRollbacks: 1,
		},
		{
			name:        "commit failure continues without error",
			boardIssues: []boardIssueRow{{"PROJ-1", 1}},
			commitErr:   errors.Default.New("commit failed"),
			wantBegins:  1,
			wantDeletes: 2,
			wantCommits: 1,
		},
		{
			name:        "two stale issues both deleted",
			boardIssues: []boardIssueRow{{"PROJ-1", 1}, {"PROJ-2", 2}},
			wantBegins:  2,
			wantDeletes: 4,
			wantCommits: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := buildTestTaskData("http://127.0.0.1:1", 1, 42)
			if tt.allErr == nil && len(tt.boardIssues) > 0 {
				srv := newCleanupHTTPServer(tt.boardStatus, tt.onBoardKeys, tt.issueStatus)
				defer srv.Close()
				data = buildTestTaskData(srv.URL, 1, 42)
			}

			ctx, db, tx := setupCleanupMocks(t, data)
			stubBoardIssues(db, tt.boardIssues, tt.allErr)

			if tt.wantBegins > 0 {
				db.On("Begin").Return(tx).Times(tt.wantBegins)

				switch tt.deleteFailAt {
				case 1:
					tx.On("Delete", mock.Anything, mock.Anything).Return(errors.Default.New("fk")).Once()
				case 2:
					tx.On("Delete", mock.Anything, mock.Anything).Return(nil).Once()
					tx.On("Delete", mock.Anything, mock.Anything).Return(errors.Default.New("fk")).Once()
				default:
					tx.On("Delete", mock.Anything, mock.Anything).Return(nil).Times(tt.wantDeletes)
				}

				if tt.wantCommits > 0 {
					tx.On("Commit").Return(tt.commitErr).Times(tt.wantCommits)
				}
				if tt.wantRollbacks > 0 {
					tx.On("Rollback").Return(nil).Times(tt.wantRollbacks)
				}
			}

			err := CleanupStaleBoardIssues(ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			db.AssertNumberOfCalls(t, "Begin", tt.wantBegins)
			if tt.wantBegins > 0 {
				tx.AssertNumberOfCalls(t, "Delete", tt.wantDeletes)
				tx.AssertNumberOfCalls(t, "Commit", tt.wantCommits)
				tx.AssertNumberOfCalls(t, "Rollback", tt.wantRollbacks)
			}
		})
	}
}

func TestCleanupStaleBoardIssues_DeletesToolThenDomain(t *testing.T) {
	srv := newCleanupHTTPServer(http.StatusOK, nil, http.StatusNotFound)
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	ctx, db, tx := setupCleanupMocks(t, data)
	stubBoardIssues(db, []boardIssueRow{{"PROJ-9", 9}}, nil)

	var deleted []string
	db.On("Begin").Return(tx)
	tx.On("Delete", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		switch args.Get(0).(type) {
		case *models.JiraBoardIssue:
			deleted = append(deleted, "tool")
		case *ticket.BoardIssue:
			deleted = append(deleted, "domain")
		default:
			t.Errorf("unexpected delete type %T", args.Get(0))
		}
	}).Return(nil)
	tx.On("Commit").Return(nil)

	err := CleanupStaleBoardIssues(ctx)
	assert.NoError(t, err)
	assert.Equal(t, []string{"tool", "domain"}, deleted)
}

func TestCleanupStaleBoardIssues_IncompleteMembershipDoesNotDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := boardResponse{Total: 5, Issues: []boardRespIssue{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	ctx, db, tx := setupCleanupMocks(t, data)
	stubBoardIssues(db, []boardIssueRow{{"PROJ-1", 1}, {"PROJ-2", 2}}, nil)

	err := CleanupStaleBoardIssues(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete board membership")
	db.AssertNumberOfCalls(t, "Begin", 0)
	tx.AssertNotCalled(t, "Delete")
}
