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
	"reflect"
	"testing"
	"time"

	mockdal "github.com/apache/devlake/mocks/core/dal"
	mocklog "github.com/apache/devlake/mocks/core/log"
	mockplugin "github.com/apache/devlake/mocks/core/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

func setupCodecovMocks(t *testing.T) (*mockplugin.SubTaskContext, *mockdal.Dal, *mocklog.Logger) {
	t.Helper()

	mockCtx := new(mockplugin.SubTaskContext)
	mockDal := new(mockdal.Dal)
	mockLogger := new(mocklog.Logger)
	nestedLogger := new(mocklog.Logger)

	data := &CodecovTaskData{
		Options: &CodecovOptions{
			ConnectionId: 1,
			FullName:     "owner/repo",
		},
	}

	mockCtx.On("GetData").Return(data)
	mockCtx.On("GetDal").Return(mockDal)
	mockCtx.On("GetLogger").Return(mockLogger)
	mockCtx.On("GetName").Return("test")
	mockCtx.On("GetContext").Return(context.Background())
	mockCtx.On("SetProgress", mock.Anything, mock.Anything).Maybe()
	mockCtx.On("IncProgress", mock.Anything).Maybe()
	mockCtx.On("NestedLogger", mock.Anything).Return(mockCtx).Maybe()
	mockCtx.On("ReplaceLogger", mock.Anything).Return(mockCtx).Maybe()
	mockCtx.On("GetConfig", mock.Anything).Return("").Maybe()
	mockCtx.On("GetConfigReader").Return(nil).Maybe()

	mockLogger.On("Info", mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Debug", mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockLogger.On("Nested", mock.Anything).Return(nestedLogger).Maybe()

	nestedLogger.On("Info", mock.Anything, mock.Anything).Maybe()
	nestedLogger.On("Debug", mock.Anything, mock.Anything).Maybe()
	nestedLogger.On("Warn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	nestedLogger.On("Nested", mock.Anything).Return(nestedLogger).Maybe()

	return mockCtx, mockDal, mockLogger
}

func TestConvertFlags_NoTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockDal.On("HasTable", "_raw_codecov_api_flags").Return(false)

	err := ConvertFlags(mockCtx)
	assert.Nil(t, err)
}

func TestConvertComparison_NoTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockDal.On("HasTable", "_raw_codecov_api_comparisons").Return(false)

	err := ConvertComparison(mockCtx)
	assert.Nil(t, err)
}

func TestExtractCommits_NoTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockDal.On("HasTable", "_raw_codecov_api_commits").Return(false)

	err := ExtractCommits(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverageTrend_NoTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockDal.On("HasTable", "_raw_codecov_api_flag_coverage_trends").Return(false)

	err := ConvertCoverageTrend(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverage_NoTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockDal.On("HasTable", "_raw_codecov_api_commit_coverages").Return(false)

	err := ConvertCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCommitCoverage_NoTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockDal.On("HasTable", "_raw_codecov_api_commit_totals").Return(false)

	err := ConvertCommitCoverage(mockCtx)
	assert.Nil(t, err)
}

// setupConverterPreloadMocks adds All() mock expectations for the pre-loaded
// commit and comparison maps used by ConvertCoverage and ConvertCommitCoverage.
func setupConverterPreloadMocks(mockDal *mockdal.Dal, commits []models.CodecovCommit, comparisons []ComparisonData) {
	mockDal.On("All", mock.AnythingOfType("*[]models.CodecovCommit"), mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(0).(*[]models.CodecovCommit)
		*dst = commits
	}).Return(nil)
	mockDal.On("All", mock.AnythingOfType("*[]tasks.ComparisonData"), mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(0).(*[]ComparisonData)
		*dst = comparisons
	}).Return(nil)
}

func setupEmptyTableMocks(mockDal *mockdal.Dal, tableName string) {
	mockRows := new(mockdal.Rows)
	mockDal.On("HasTable", "_raw_"+tableName).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(0), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)
}

func TestConvertFlags_EmptyTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupEmptyTableMocks(mockDal, RAW_FLAGS_TABLE)

	err := ConvertFlags(mockCtx)
	assert.Nil(t, err)
}

func TestExtractCommits_EmptyTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupEmptyTableMocks(mockDal, RAW_COMMITS_TABLE)

	err := ExtractCommits(mockCtx)
	assert.Nil(t, err)
}

func TestConvertComparison_EmptyTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupEmptyTableMocks(mockDal, RAW_COMPARISONS_TABLE)

	err := ConvertComparison(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverageTrend_EmptyTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupEmptyTableMocks(mockDal, RAW_FLAG_COVERAGE_TRENDS_TABLE)

	err := ConvertCoverageTrend(mockCtx)
	assert.Nil(t, err)
}

func TestConvertFlags_WithOneRow(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_FLAGS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	flagJSON, _ := json.Marshal(map[string]any{
		"flag_name":    "unit-tests",
		"coverage":     85.5,
		"carryforward": false,
		"deleted":      false,
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   flagJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertFlags(mockCtx)
	assert.Nil(t, err)
}

func TestExtractCommits_WithOneRow(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMITS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	commitJSON, _ := json.Marshal(map[string]any{
		"commitid":  "abc123def",
		"branch":    "main",
		"message":   "feat: add tests",
		"parent":    "parent123",
		"timestamp": "2024-06-15T10:00:00Z",
		"author": map[string]string{
			"name": "developer1",
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   commitJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ExtractCommits(mockCtx)
	assert.Nil(t, err)
}

func TestConvertComparison_WithOneRow(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMPARISONS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(ComparisonInput{
		CommitSha: "abc123",
		ParentSha: "parent456",
		FlagName:  "unit-tests",
	})
	dataJSON, _ := json.Marshal(map[string]any{
		"base_commitid": "parent456",
		"head_commitid": "abc123",
		"diff": map[string]any{
			"files": []map[string]string{{"name": "main.go"}},
			"totals": map[string]any{
				"files":    1,
				"lines":    100,
				"hits":     80,
				"misses":   15,
				"partials": 5,
				"coverage": 80.0,
				"methods":  10,
			},
		},
		"totals": map[string]any{
			"patch": map[string]any{
				"files":    1,
				"lines":    50,
				"coverage": 75.0,
			},
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertComparison(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverageTrend_WithOneRow(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_FLAG_COVERAGE_TRENDS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(map[string]string{"flag_name": "unit-tests"})
	dataJSON, _ := json.Marshal(map[string]any{
		"timestamp": "2024-06-15T00:00:00Z",
		"min":       80.0,
		"max":       90.0,
		"avg":       85.5,
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "Date", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverageTrend(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverage_WithOneRow(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_COVERAGES_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitFlagInput{
		CommitSha: "abc123",
		FlagName:  "unit-tests",
	})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"files":    10,
			"lines":    500,
			"hits":     400,
			"misses":   80,
			"partials": 20,
			"coverage": 80.0,
			"methods":  50,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	}).Maybe()
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil).Maybe()

	err := ConvertCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCommitCoverage_WithOneRow(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_TOTALS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitInput{CommitSha: "abc123"})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"files":    10,
			"lines":    500,
			"hits":     400,
			"misses":   80,
			"partials": 20,
			"coverage": 80.0,
			"methods":  50,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	}).Maybe()
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil).Maybe()
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil).Maybe()

	err := ConvertCommitCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestComparisonDataTableName(t *testing.T) {
	cd := ComparisonData{}
	assert.Equal(t, "_tool_codecov_comparisons", cd.TableName())
}

func TestConvertCoverage_EmptyTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	setupEmptyTableMocks(mockDal, RAW_COMMIT_COVERAGES_TABLE)
	err := ConvertCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCommitCoverage_EmptyTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	setupEmptyTableMocks(mockDal, RAW_COMMIT_TOTALS_TABLE)
	err := ConvertCommitCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverageTrend_DateFallback(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_FLAG_COVERAGE_TRENDS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(map[string]string{"flag_name": "unit-tests"})
	dataJSON, _ := json.Marshal(map[string]any{
		"timestamp": "2024-06-15",
		"min":       80.0,
		"max":       90.0,
		"avg":       85.5,
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "Date", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverageTrend(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverageTrend_InvalidDate(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_FLAG_COVERAGE_TRENDS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(map[string]string{"flag_name": "unit-tests"})
	dataJSON, _ := json.Marshal(map[string]any{
		"timestamp": "not-a-date",
		"min":       80.0,
		"max":       90.0,
		"avg":       85.5,
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "Date", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverageTrend(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverage_CommitFound(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	setupConverterPreloadMocks(mockDal, []models.CodecovCommit{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", Branch: "main", CommitTimestamp: &ts},
	}, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_COVERAGES_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitFlagInput{
		CommitSha: "abc123",
		FlagName:  "unit-tests",
	})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"files": 10, "lines": 500, "hits": 400,
			"misses": 80, "partials": 20, "coverage": 80.0, "methods": 50,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverage_EmptyFlagName(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	setupConverterPreloadMocks(mockDal, []models.CodecovCommit{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", Branch: "main", CommitTimestamp: &ts},
	}, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_COVERAGES_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitFlagInput{CommitSha: "abc123", FlagName: ""})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"coverage": 80.0, "hits": 400, "lines": 500,
			"misses": 80, "partials": 20, "methods": 50,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverage_FlagInMap(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	setupConverterPreloadMocks(mockDal, []models.CodecovCommit{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", Branch: "main", CommitTimestamp: &ts},
	}, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_COVERAGES_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitFlagInput{CommitSha: "abc123", FlagName: "unit-tests"})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"coverage": 80.0, "hits": 400, "lines": 500,
			"misses": 80, "partials": 20, "methods": 50,
		},
		"flags": map[string]any{
			"unit-tests": map[string]any{
				"coverage": 90.0, "hits": 450, "lines": 500,
				"misses": 30, "partials": 20, "methods": 60,
			},
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCoverage_ComparisonFound(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	setupConverterPreloadMocks(mockDal, []models.CodecovCommit{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", Branch: "main", CommitTimestamp: &ts},
	}, []ComparisonData{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", FlagName: "unit-tests", ModifiedCoverage: 92.5},
	})
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_COVERAGES_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitFlagInput{CommitSha: "abc123", FlagName: "unit-tests"})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"coverage": 80.0, "hits": 400, "lines": 500,
			"misses": 80, "partials": 20, "methods": 50,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCommitCoverage_CommitFound(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	setupConverterPreloadMocks(mockDal, []models.CodecovCommit{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", Branch: "main", CommitTimestamp: &ts},
	}, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_TOTALS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitInput{CommitSha: "abc123"})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"files": 10, "lines": 500, "hits": 400,
			"misses": 80, "partials": 20, "coverage": 80.0, "methods": 50,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCommitCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCommitCoverage_CommitAndComparison(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	setupConverterPreloadMocks(mockDal, []models.CodecovCommit{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", Branch: "main", CommitTimestamp: &ts},
	}, []ComparisonData{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", FlagName: "", ModifiedCoverage: 75.5, FilesChanged: 3, MethodsCovered: 20, MethodsTotal: 25},
	})
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_TOTALS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitInput{CommitSha: "abc123"})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"files": 10, "lines": 500, "hits": 400,
			"misses": 80, "partials": 20, "coverage": 80.0, "methods": 50,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCommitCoverage(mockCtx)
	assert.Nil(t, err)
}

func TestConvertCommitCoverage_EmptyCommitSha(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_TOTALS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitInput{CommitSha: ""})
	dataJSON, _ := json.Marshal(map[string]any{
		"totals": map[string]any{
			"coverage": 80.0, "hits": 400, "lines": 500,
			"misses": 80, "partials": 20, "methods": 50,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCommitCoverage(mockCtx)
	assert.Nil(t, err)
}

// --- ConvertComparison branch coverage tests ---

func TestConvertComparison_InvalidInputJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMPARISONS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`{"valid":"json"}`),
			Input:  []byte(`<<<not json>>>`),
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertComparison(mockCtx)
	assert.NotNil(t, err)
}

func TestConvertComparison_InvalidDataJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMPARISONS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(ComparisonInput{
		CommitSha: "abc123",
		ParentSha: "parent456",
		FlagName:  "unit-tests",
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`<<<not json>>>`),
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertComparison(mockCtx)
	assert.NotNil(t, err)
}

func TestConvertComparison_PatchNil(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMPARISONS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(ComparisonInput{
		CommitSha: "abc123",
		ParentSha: "parent456",
		FlagName:  "unit-tests",
	})
	// No "totals.patch" at all -> Patch pointer stays nil
	dataJSON, _ := json.Marshal(map[string]any{
		"base_commitid": "parent456",
		"head_commitid": "abc123",
		"diff": map[string]any{
			"files":  []map[string]string{},
			"totals": map[string]any{"coverage": 0.0},
		},
		"totals": map[string]any{},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertComparison(mockCtx)
	assert.Nil(t, err)
}

func TestConvertComparison_PatchCoverageNil(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMPARISONS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(ComparisonInput{
		CommitSha: "abc123",
		ParentSha: "parent456",
		FlagName:  "unit-tests",
	})
	// Patch exists but coverage is null -> Coverage pointer stays nil
	dataJSON, _ := json.Marshal(map[string]any{
		"base_commitid": "parent456",
		"head_commitid": "abc123",
		"diff": map[string]any{
			"files":  []map[string]string{},
			"totals": map[string]any{"coverage": 0.0},
		},
		"totals": map[string]any{
			"patch": map[string]any{
				"files":    1,
				"lines":    10,
				"coverage": nil,
			},
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertComparison(mockCtx)
	assert.Nil(t, err)
}

func TestConvertComparison_PatchZeroFilesAndLines(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMPARISONS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(ComparisonInput{
		CommitSha: "abc123",
		ParentSha: "parent456",
		FlagName:  "unit-tests",
	})
	// Patch exists, Coverage is non-nil, but Files=0 and Lines=0
	// -> patchCoverage should remain nil (treated as no actual changes)
	cov := 0.0
	patchData := map[string]any{
		"files":    0,
		"lines":    0,
		"coverage": cov,
	}
	dataJSON, _ := json.Marshal(map[string]any{
		"base_commitid": "parent456",
		"head_commitid": "abc123",
		"diff": map[string]any{
			"files":  []map[string]string{},
			"totals": map[string]any{"coverage": 0.0},
		},
		"totals": map[string]any{
			"patch": patchData,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertComparison(mockCtx)
	assert.Nil(t, err)
}

// --- ConvertFlags branch coverage tests ---

func TestConvertFlags_InvalidJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_FLAGS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`<<<not json>>>`),
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertFlags(mockCtx)
	assert.NotNil(t, err)
}

func TestConvertFlags_EmptyFlagName(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_FLAGS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	flagJSON, _ := json.Marshal(map[string]any{
		"flag_name":    "",
		"coverage":     85.5,
		"carryforward": false,
		"deleted":      false,
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   flagJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertFlags(mockCtx)
	assert.Nil(t, err)
}

// --- ExtractCommits branch coverage tests ---

func TestExtractCommits_InvalidJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMITS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`<<<not json>>>`),
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ExtractCommits(mockCtx)
	assert.NotNil(t, err)
}

func TestExtractCommits_EmptyTimestamp(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMITS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	commitJSON, _ := json.Marshal(map[string]any{
		"commitid":  "abc123def",
		"branch":    "main",
		"message":   "feat: add tests",
		"parent":    "parent123",
		"timestamp": "",
		"author": map[string]string{
			"name": "developer1",
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   commitJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ExtractCommits(mockCtx)
	assert.Nil(t, err)
}

func TestExtractCommits_InvalidTimestamp(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMITS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	commitJSON, _ := json.Marshal(map[string]any{
		"commitid":  "abc123def",
		"branch":    "main",
		"message":   "feat: add tests",
		"parent":    "parent123",
		"timestamp": "not-a-valid-timestamp",
		"author": map[string]string{
			"name": "developer1",
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   commitJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ExtractCommits(mockCtx)
	assert.Nil(t, err)
}

// --- ConvertCoverageTrend branch coverage tests ---

func TestConvertCoverageTrend_InvalidInputJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_FLAG_COVERAGE_TRENDS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`{"timestamp":"2024-06-15T00:00:00Z","min":80.0,"max":90.0,"avg":85.5}`),
			Input:  []byte(`<<<not json>>>`),
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverageTrend(mockCtx)
	assert.NotNil(t, err)
}

func TestConvertCoverageTrend_InvalidDataJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_FLAG_COVERAGE_TRENDS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(map[string]string{"flag_name": "unit-tests"})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`<<<not json>>>`),
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverageTrend(mockCtx)
	assert.NotNil(t, err)
}

// --- ConvertCoverage branch coverage tests ---

func TestConvertCoverage_InvalidInputJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_COVERAGES_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`{"commitid":"abc123","totals":{"coverage":80.0}}`),
			Input:  []byte(`<<<not json>>>`),
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverage(mockCtx)
	assert.NotNil(t, err)
}

func TestConvertCoverage_InvalidDataJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_COVERAGES_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitFlagInput{CommitSha: "abc123", FlagName: "unit-tests"})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`<<<not json>>>`),
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverage(mockCtx)
	assert.NotNil(t, err)
}

func TestConvertCoverage_FlagNotInMap(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	setupConverterPreloadMocks(mockDal, []models.CodecovCommit{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "abc123", Branch: "main", CommitTimestamp: &ts},
	}, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_COVERAGES_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitFlagInput{CommitSha: "abc123", FlagName: "unit-tests"})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "abc123",
		"totals": map[string]any{
			"coverage": 80.0, "hits": 400, "lines": 500,
			"misses": 80, "partials": 20, "methods": 50,
		},
		"flags": map[string]any{
			"integration-tests": map[string]any{
				"coverage": 70.0, "hits": 350, "lines": 500,
				"misses": 120, "partials": 30, "methods": 40,
			},
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "FlagName", Type: reflect.TypeOf("")},
		{Name: "Branch", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCoverage(mockCtx)
	assert.Nil(t, err)
}

// --- ConvertCommitCoverage branch coverage tests ---

func TestConvertCommitCoverage_InvalidInputJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_TOTALS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`{"commitid":"abc123","totals":{"coverage":80.0}}`),
			Input:  []byte(`<<<not json>>>`),
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCommitCoverage(mockCtx)
	assert.NotNil(t, err)
}

func TestConvertCommitCoverage_InvalidDataJSON(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	setupConverterPreloadMocks(mockDal, nil, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_TOTALS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitInput{CommitSha: "abc123"})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   []byte(`<<<not json>>>`),
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCommitCoverage(mockCtx)
	assert.NotNil(t, err)
}

func TestConvertCommitCoverage_FallbackToTotalsCommitid(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	setupConverterPreloadMocks(mockDal, []models.CodecovCommit{
		{ConnectionId: 1, RepoId: "owner/repo", CommitSha: "fallback123", Branch: "develop", CommitTimestamp: &ts},
	}, nil)
	mockRows := new(mockdal.Rows)

	mockDal.On("HasTable", "_raw_"+RAW_COMMIT_TOTALS_TABLE).Return(true)
	mockDal.On("Count", mock.Anything).Return(int64(1), nil)
	mockDal.On("Cursor", mock.Anything).Return(mockRows, nil)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Next").Return(false)
	mockRows.On("Close").Return(nil)

	inputJSON, _ := json.Marshal(CommitInput{CommitSha: ""})
	dataJSON, _ := json.Marshal(map[string]any{
		"commitid": "fallback123",
		"totals": map[string]any{
			"files": 5, "lines": 200, "hits": 150,
			"misses": 40, "partials": 10, "coverage": 75.0, "methods": 20,
		},
	})

	mockDal.On("Fetch", mockRows, mock.Anything).Run(func(args mock.Arguments) {
		dst := args.Get(1).(*helper.RawData)
		*dst = helper.RawData{
			ID:     1,
			Params: `{"ConnectionId":1,"Name":"owner/repo"}`,
			Data:   dataJSON,
			Input:  inputJSON,
		}
	}).Return(nil)

	mockDal.On("GetPrimaryKeyFields", mock.Anything).Return([]reflect.StructField{
		{Name: "ConnectionId", Type: reflect.TypeOf(uint64(0))},
		{Name: "RepoId", Type: reflect.TypeOf("")},
		{Name: "CommitSha", Type: reflect.TypeOf("")},
	})
	mockDal.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockDal.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil)

	err := ConvertCommitCoverage(mockCtx)
	assert.Nil(t, err)
}
