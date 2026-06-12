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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseAnalyticsDateFromDayInput(t *testing.T) {
	input, err := json.Marshal(claudeCodeDayInput{Day: "2026-03-04"})
	assert.NoError(t, err)

	date, parseErr := parseAnalyticsDate(input)

	assert.Nil(t, parseErr)
	assert.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC), date)
}

func TestParseAnalyticsDateFromRangeInput(t *testing.T) {
	input, err := json.Marshal(claudeCodeDateRangeInput{StartDate: "2026-03-04", EndDate: "2026-03-05"})
	assert.NoError(t, err)

	date, parseErr := parseAnalyticsDate(input)

	assert.Nil(t, parseErr)
	assert.Equal(t, time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC), date)
}

func TestParseAnalyticsDateReturnsErrorForInvalidInput(t *testing.T) {
	date, parseErr := parseAnalyticsDate(json.RawMessage(`{"unknown":"2026-03-04"}`))

	assert.NotNil(t, parseErr)
	assert.True(t, date.IsZero())
}
