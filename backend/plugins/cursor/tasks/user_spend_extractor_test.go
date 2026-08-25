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
	"math"
	"testing"
)

func TestNormalizeUserSpendCents(t *testing.T) {
	tests := []struct {
		name            string
		spendCents      float64
		includedSpend   float64
		overallSpend    float64
		wantOnDemand    float64
		wantIncluded    float64
	}{
		{
			name:         "team legacy shape",
			spendCents:   3224,
			includedSpend: 1984,
			wantOnDemand: 3224,
			wantIncluded: 1984,
		},
		{
			name:         "enterprise overall only",
			spendCents:   0,
			overallSpend: 1619.087826,
			wantOnDemand: 0,
			wantIncluded: 1619.087826,
		},
		{
			name:         "tiered on-demand plus overall",
			spendCents:   1875.500123,
			overallSpend: 3200.750456,
			wantOnDemand: 1875.500123,
			wantIncluded: 1325.250333,
		},
		{
			name:         "explicit included wins when both present",
			spendCents:   100,
			includedSpend: 50,
			overallSpend: 200,
			wantOnDemand: 100,
			wantIncluded: 50,
		},
		{
			name:         "all zero",
			wantOnDemand: 0,
			wantIncluded: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onDemand, included := normalizeUserSpendCents(tt.spendCents, tt.includedSpend, tt.overallSpend)
			if math.Abs(onDemand-tt.wantOnDemand) > 1e-6 {
				t.Fatalf("onDemand = %v, want %v", onDemand, tt.wantOnDemand)
			}
			if math.Abs(included-tt.wantIncluded) > 1e-6 {
				t.Fatalf("included = %v, want %v", included, tt.wantIncluded)
			}
		})
	}
}

func TestSpendMemberRecordUnmarshalEnterpriseShape(t *testing.T) {
	raw := []byte(`{
		"userId":"user_enterprise123",
		"spendCents":0,
		"overallSpendCents":1619.087826,
		"fastPremiumRequests":0,
		"name":"Enterprise User",
		"email":"enterprise@example.com",
		"role":"owner",
		"monthlyLimitDollars":500,
		"hardLimitOverrideDollars":0
	}`)

	var record spendMemberRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatalf("unmarshal spend member: %v", err)
	}

	onDemand, included := normalizeUserSpendCents(record.SpendCents, record.IncludedSpendCents, record.OverallSpendCents)
	if onDemand != 0 {
		t.Fatalf("onDemand = %v, want 0", onDemand)
	}
	if included != 1619.087826 {
		t.Fatalf("included = %v, want 1619.087826", included)
	}
}
