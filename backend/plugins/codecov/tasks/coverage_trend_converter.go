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
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/codecov/models"
)

var ConvertCoverageTrendMeta = plugin.SubTaskMeta{
	Name:             "ConvertCoverageTrend",
	EntryPoint:       ConvertCoverageTrend,
	EnabledByDefault: true,
	Description:      "Convert coverage trend data per flag from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	DependencyTables: []string{RAW_FLAG_COVERAGE_TRENDS_TABLE},
}

func ConvertCoverageTrend(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_FLAG_COVERAGE_TRENDS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			// Read input to get flag name
			var input struct {
				FlagName string `json:"flag_name"`
			}
			if err := errors.Convert(json.Unmarshal(resData.Input, &input)); err != nil {
				return nil, err
			}

			// Parse individual trend point from results array
			// Each raw data record is a single trend point: {"timestamp": "...", "min": ..., "max": ..., "avg": ...}
			var point struct {
				Timestamp string  `json:"timestamp"`
				Min       float64 `json:"min"`
				Max       float64 `json:"max"`
				Avg       float64 `json:"avg"`
			}
			err := errors.Convert(json.Unmarshal(resData.Data, &point))
			if err != nil {
				return nil, err
			}

			// Extract branch from scope config or default to main
			branch := "main"
			if data.Options.ScopeConfig != nil {
				// Could add branch config here if needed
			}

			// Parse timestamp like "2025-12-02T00:00:00Z"
			date, parseErr := time.Parse(time.RFC3339, point.Timestamp)
			if parseErr != nil {
				// Try alternative format
				date, parseErr = time.Parse("2006-01-02", point.Timestamp[:10])
				if parseErr != nil {
					return nil, nil // Skip invalid dates
				}
			}

			codecovTrend := &models.CodecovCoverageTrend{
				NoPKModel:          common.NoPKModel{},
				ConnectionId:       data.Options.ConnectionId,
				RepoId:             data.Options.FullName,
				FlagName:           input.FlagName,
				Branch:             branch,
				Date:               date,
				CoveragePercentage: point.Avg, // Use average coverage
				LinesCovered:       0,         // Not provided by this endpoint
				LinesTotal:         0,         // Not provided by this endpoint
				MethodsCovered:     0,         // Not provided by this endpoint
				MethodsTotal:       0,         // Not provided by this endpoint
			}

			return []interface{}{codecovTrend}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
