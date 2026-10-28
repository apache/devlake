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

var ConvertCommitCoverageMeta = plugin.SubTaskMeta{
	Name:             "ConvertCommitCoverage",
	EntryPoint:       ConvertCommitCoverage,
	EnabledByDefault: true,
	Description:      "Convert overall and modified coverage per commit from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertComparisonMeta},
	DependencyTables: []string{RAW_COMMIT_TOTALS_TABLE, RAW_COMPARISONS_TABLE},
}

func ConvertCommitCoverage(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()

	// Pre-load all commits for this repo into memory to avoid N+1 queries
	var commits []models.CodecovCommit
	err := db.All(&commits, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return errors.Default.Wrap(err, "failed to pre-load commits for commit coverage conversion")
	}
	commitMap := make(map[string]*models.CodecovCommit, len(commits))
	for i := range commits {
		commitMap[commits[i].CommitSha] = &commits[i]
	}
	taskCtx.GetLogger().Info("[ConvertCommitCoverage] pre-loaded %d commits into memory", len(commitMap))

	// Pre-load comparison data with empty flag (overall comparisons) for this repo
	var comparisons []ComparisonData
	err = db.All(&comparisons, dal.Where("connection_id = ? AND repo_id = ? AND flag_name = ?", data.Options.ConnectionId, data.Options.FullName, ""))
	if err != nil {
		return errors.Default.Wrap(err, "failed to pre-load comparisons for commit coverage conversion")
	}
	comparisonMap := make(map[string]*ComparisonData, len(comparisons))
	for i := range comparisons {
		comparisonMap[comparisons[i].CommitSha] = &comparisons[i]
	}
	taskCtx.GetLogger().Info("[ConvertCommitCoverage] pre-loaded %d comparisons into memory", len(comparisonMap))

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMMIT_TOTALS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			var input CommitInput
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
			}
			err = errors.Convert(json.Unmarshal(resData.Data, &totals))
			if err != nil {
				return nil, err
			}

			commitSha := input.CommitSha
			if commitSha == "" && totals.Commitid != "" {
				commitSha = totals.Commitid
			}
			if commitSha == "" {
				return nil, nil
			}

			commit, ok := commitMap[commitSha]
			if !ok {
				return nil, nil
			}

			var modifiedCoverage float64
			var filesChanged int
			var methodsCovered, methodsTotal int

			if comp, ok := comparisonMap[commitSha]; ok {
				modifiedCoverage = comp.ModifiedCoverage
				filesChanged = comp.FilesChanged
				methodsCovered = comp.MethodsCovered
				methodsTotal = comp.MethodsTotal
			} else {
				methodsCovered = totals.Totals.Methods
				methodsTotal = totals.Totals.Methods
			}

			return []interface{}{&models.CodecovCommitCoverage{
				NoPKModel:        common.NoPKModel{},
				ConnectionId:     data.Options.ConnectionId,
				RepoId:           data.Options.FullName,
				CommitSha:        commitSha,
				Branch:           commit.Branch,
				CommitTimestamp:  commit.CommitTimestamp,
				OverallCoverage:  totals.Totals.Coverage,
				ModifiedCoverage: modifiedCoverage,
				FilesChanged:     filesChanged,
				LinesCovered:     totals.Totals.Hits,
				LinesTotal:       totals.Totals.Lines,
				LinesMissed:      totals.Totals.Misses,
				Hits:             totals.Totals.Hits,
				Partials:         totals.Totals.Partials,
				Misses:           totals.Totals.Misses,
				MethodsCovered:   methodsCovered,
				MethodsTotal:     methodsTotal,
			}}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
