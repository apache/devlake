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
	"strings"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
	helper "github.com/apache/devlake/helpers/pluginhelper/api"
	"github.com/apache/devlake/plugins/cursor/models"
)

type spendMemberRecord struct {
	UserId                   string  `json:"userId"`
	Email                    string  `json:"email"`
	Name                     string  `json:"name"`
	Role                     string  `json:"role"`
	SpendCents               float64 `json:"spendCents"`
	IncludedSpendCents       float64 `json:"includedSpendCents"`
	OverallSpendCents        float64 `json:"overallSpendCents"`
	FastPremiumRequests      int     `json:"fastPremiumRequests"`
	MonthlyLimitDollars      float64 `json:"monthlyLimitDollars"`
	HardLimitOverrideDollars float64 `json:"hardLimitOverrideDollars"`
}

// normalizeUserSpendCents maps Cursor /teams/spend fields to on-demand and included
// amounts. Team plans return spendCents + includedSpendCents; Enterprise and newer
// tiered plans often omit includedSpendCents and expose overallSpendCents instead.
func normalizeUserSpendCents(spendCents, includedSpendCents, overallSpendCents float64) (onDemand, included float64) {
	onDemand = spendCents
	included = includedSpendCents

	if overallSpendCents <= 0 {
		return onDemand, included
	}

	derivedIncluded := overallSpendCents - spendCents
	if derivedIncluded < 0 {
		derivedIncluded = 0
	}

	if includedSpendCents > 0 {
		return onDemand, includedSpendCents
	}

	return onDemand, derivedIncluded
}

// ExtractUserSpend parses raw spend records into tool-layer tables.
func ExtractUserSpend(taskCtx plugin.SubTaskContext) errors.Error {
	data, ok := taskCtx.TaskContext().GetData().(*CursorTaskData)
	if !ok {
		return errors.Default.New("task data is not CursorTaskData")
	}

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx:     taskCtx,
			Table:   rawUserSpendTable,
			Options: rawParamsFromTaskData(data),
		},
		Extract: func(row *helper.RawData) ([]interface{}, errors.Error) {
			var wrapped spendRawRecord
			if err := errors.Convert(json.Unmarshal(row.Data, &wrapped)); err != nil {
				return nil, err
			}

			var record spendMemberRecord
			if err := errors.Convert(json.Unmarshal(wrapped.Member, &record)); err != nil {
				return nil, err
			}

			userId := strings.TrimSpace(record.UserId)
			if userId == "" {
				userId = strings.TrimSpace(record.Email)
			}
			if userId == "" {
				return nil, nil
			}

			onDemandCents, includedCents := normalizeUserSpendCents(
				record.SpendCents,
				record.IncludedSpendCents,
				record.OverallSpendCents,
			)

			spend := &models.CursorUserSpend{
				ConnectionId:             data.Options.ConnectionId,
				ScopeId:                  data.Options.ScopeId,
				UserId:                   userId,
				BillingCycleStart:        billingCycleTime(wrapped.SubscriptionCycleStart),
				CollectedAt:              wrapped.CollectedAt,
				Email:                    strings.TrimSpace(record.Email),
				Name:                     strings.TrimSpace(record.Name),
				Role:                     strings.TrimSpace(record.Role),
				SpendCents:               onDemandCents,
				IncludedSpendCents:       includedCents,
				FastPremiumRequests:      record.FastPremiumRequests,
				MonthlyLimitDollars:      record.MonthlyLimitDollars,
				HardLimitOverrideDollars: record.HardLimitOverrideDollars,
			}
			return []interface{}{spend}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
