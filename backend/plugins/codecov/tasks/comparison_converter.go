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

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
)

// ComparisonData stores comparison results for linking to commit coverage per flag
type ComparisonData struct {
	common.NoPKModel          // Includes CreatedAt, UpdatedAt, and RawDataOrigin
	ConnectionId     uint64   `gorm:"primaryKey;type:bigint"`
	RepoId           string   `gorm:"primaryKey;type:varchar(200);index"`
	CommitSha        string   `gorm:"primaryKey;type:varchar(64);index"`
	FlagName         string   `gorm:"primaryKey;type:varchar(100);index"`
	ParentSha        string   `gorm:"type:varchar(64)"`
	ModifiedCoverage float64  `gorm:"type:double"`
	FilesChanged     int      `gorm:"type:int"`
	MethodsCovered   int      `gorm:"type:int"`
	MethodsTotal     int      `gorm:"type:int"`
	LinesCovered     int      `gorm:"type:int"`    // Lines covered in modified code
	LinesTotal       int      `gorm:"type:int"`    // Total lines in modified code
	LinesMissed      int      `gorm:"type:int"`    // Lines missed in modified code
	Patch            *float64 `gorm:"type:double"` // Patch coverage from compare API (can be null)
}

func (ComparisonData) TableName() string {
	return "_tool_codecov_comparisons"
}

var ConvertComparisonMeta = plugin.SubTaskMeta{
	Name:             "ConvertComparison",
	EntryPoint:       ConvertComparison,
	EnabledByDefault: true,
	Description:      "Convert comparison data (modified/patch coverage) from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractCommitsMeta},
	DependencyTables: []string{RAW_COMPARISONS_TABLE},
}

func ConvertComparison(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMPARISONS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			// Read input to get commit and parent SHA
			var input ComparisonInput
			err := errors.Convert(json.Unmarshal(resData.Input, &input))
			if err != nil {
				return nil, err
			}

			// Parse comparison response
			var comparison struct {
				BaseCommitid string `json:"base_commitid"`
				HeadCommitid string `json:"head_commitid"`
				Diff         struct {
					Files []struct {
						Name string `json:"name"`
					} `json:"files"`
					Totals struct {
						Files      int     `json:"files"`
						Lines      int     `json:"lines"`
						Hits       int     `json:"hits"`
						Misses     int     `json:"misses"`
						Partials   int     `json:"partials"`
						Coverage   float64 `json:"coverage"`
						Branches   int     `json:"branches"`
						Methods    int     `json:"methods"`
						Messages   int     `json:"messages"`
						Sessions   int     `json:"sessions"`
						Complexity float64 `json:"complexity"`
					} `json:"totals"`
				} `json:"diff"`
				Totals struct {
					Patch *struct {
						Files    int      `json:"files"`
						Lines    int      `json:"lines"`
						Hits     int      `json:"hits"`
						Misses   int      `json:"misses"`
						Coverage *float64 `json:"coverage"`
					} `json:"patch"`
				} `json:"totals"`
			}
			err = errors.Convert(json.Unmarshal(resData.Data, &comparison))
			if err != nil {
				return nil, err
			}

			// Extract patch coverage and line counts from totals.patch
			// Only store patch coverage when there are actual coverable lines changed
			// (lines > 0). A commit can have files > 0 but lines = 0 if the changed
			// files contain no coverable code (e.g., only comments or config within
			// a source file). In that case patch coverage is N/A, not 0%.
			var patchCoverage *float64
			var patchFiles, patchLines, patchHits, patchMisses int
			if comparison.Totals.Patch != nil {
				patchFiles = comparison.Totals.Patch.Files
				patchLines = comparison.Totals.Patch.Lines
				patchHits = comparison.Totals.Patch.Hits
				patchMisses = comparison.Totals.Patch.Misses
				if comparison.Totals.Patch.Coverage != nil && patchLines > 0 {
					patchCoverage = comparison.Totals.Patch.Coverage
				}
			}

			comparisonData := &ComparisonData{
				NoPKModel:        common.NoPKModel{},
				ConnectionId:     data.Options.ConnectionId,
				RepoId:           data.Options.FullName,
				CommitSha:        input.CommitSha,
				FlagName:         input.FlagName,
				ParentSha:        input.ParentSha,
				ModifiedCoverage: comparison.Diff.Totals.Coverage,
				FilesChanged:     patchFiles,
				MethodsCovered:   comparison.Diff.Totals.Methods,
				MethodsTotal:     comparison.Diff.Totals.Methods,
				LinesCovered:     patchHits,
				LinesTotal:       patchLines,
				LinesMissed:      patchMisses,
				Patch:            patchCoverage,
			}

			return []interface{}{comparisonData}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
