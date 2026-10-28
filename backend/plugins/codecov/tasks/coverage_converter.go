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

	"github.com/apache/devlake/core/dal"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

var ConvertCoverageMeta = plugin.SubTaskMeta{
	Name:             "ConvertCoverage",
	EntryPoint:       ConvertCoverage,
	EnabledByDefault: true,
	Description:      "Convert coverage data per flag per commit from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertComparisonMeta},
	DependencyTables: []string{RAW_COMMIT_COVERAGES_TABLE},
}

func ConvertCoverage(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()

	// Pre-load all commits for this repo into memory to avoid N+1 queries
	var commits []models.CodecovCommit
	err := db.All(&commits, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return errors.Default.Wrap(err, "failed to pre-load commits for coverage conversion")
	}
	commitMap := make(map[string]*models.CodecovCommit, len(commits))
	for i := range commits {
		commitMap[commits[i].CommitSha] = &commits[i]
	}
	taskCtx.GetLogger().Info("[ConvertCoverage] pre-loaded %d commits into memory", len(commitMap))

	// Pre-load all comparison data for this repo into memory
	var comparisons []ComparisonData
	err = db.All(&comparisons, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return errors.Default.Wrap(err, "failed to pre-load comparisons for coverage conversion")
	}
	comparisonMap := make(map[string]*ComparisonData, len(comparisons))
	for i := range comparisons {
		key := comparisons[i].CommitSha + "|" + comparisons[i].FlagName
		comparisonMap[key] = &comparisons[i]
	}
	taskCtx.GetLogger().Info("[ConvertCoverage] pre-loaded %d comparisons into memory", len(comparisonMap))

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMMIT_COVERAGES_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			var input CommitFlagInput
			err := errors.Convert(json.Unmarshal(resData.Input, &input))
			if err != nil {
				return nil, err
			}

			var totals struct {
				Commitid string `json:"commitid"`
				Totals   struct {
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
				Flags map[string]struct {
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
				} `json:"flags"`
			}
			err = errors.Convert(json.Unmarshal(resData.Data, &totals))
			if err != nil {
				return nil, err
			}

			commitSha := input.CommitSha
			if totals.Commitid != "" {
				commitSha = totals.Commitid
			}

			commit, ok := commitMap[commitSha]
			if !ok {
				return nil, nil
			}

			flagName := input.FlagName
			if flagName == "" {
				return nil, nil
			}

			var coveragePercentage float64
			var linesCovered, linesTotal, linesMissed int
			var hits, partials, misses int
			var methodsCovered, methodsTotal int

			if flagName != "" && totals.Flags != nil {
				if flagTotals, ok := totals.Flags[flagName]; ok {
					coveragePercentage = flagTotals.Coverage
					linesCovered = flagTotals.Hits
					linesTotal = flagTotals.Lines
					linesMissed = flagTotals.Misses
					hits = flagTotals.Hits
					partials = flagTotals.Partials
					misses = flagTotals.Misses
					methodsCovered = flagTotals.Methods
					methodsTotal = flagTotals.Methods
				} else {
					coveragePercentage = totals.Totals.Coverage
					linesCovered = totals.Totals.Hits
					linesTotal = totals.Totals.Lines
					linesMissed = totals.Totals.Misses
					hits = totals.Totals.Hits
					partials = totals.Totals.Partials
					misses = totals.Totals.Misses
					methodsCovered = totals.Totals.Methods
					methodsTotal = totals.Totals.Methods
				}
			} else {
				coveragePercentage = totals.Totals.Coverage
				linesCovered = totals.Totals.Hits
				linesTotal = totals.Totals.Lines
				linesMissed = totals.Totals.Misses
				hits = totals.Totals.Hits
				partials = totals.Totals.Partials
				misses = totals.Totals.Misses
				methodsCovered = totals.Totals.Methods
				methodsTotal = totals.Totals.Methods
			}

			var modifiedCoverage float64
			if comp, ok := comparisonMap[commitSha+"|"+flagName]; ok {
				modifiedCoverage = comp.ModifiedCoverage
			}

			return []interface{}{&models.CodecovCoverage{
				NoPKModel:          common.NoPKModel{},
				ConnectionId:       data.Options.ConnectionId,
				RepoId:             data.Options.FullName,
				FlagName:           flagName,
				Branch:             commit.Branch,
				CommitSha:          commitSha,
				CommitTimestamp:    commit.CommitTimestamp,
				CoveragePercentage: coveragePercentage,
				ModifiedCoverage:   modifiedCoverage,
				LinesCovered:       linesCovered,
				LinesTotal:         linesTotal,
				LinesMissed:        linesMissed,
				Hits:               hits,
				Partials:           partials,
				Misses:             misses,
				MethodsCovered:     methodsCovered,
				MethodsTotal:       methodsTotal,
			}}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
