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

package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/apache/devlake/core/errors"

	"github.com/stretchr/testify/assert"
)

func rawItems(values ...string) []json.RawMessage {
	items := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		items = append(items, json.RawMessage(value))
	}
	return items
}

// A plugin parser stops collecting by returning the valid prefix of the page it stopped on together
// with ErrFinishCollect. Those records are inside the requested window, so they must survive: dropping
// them loses the last page of a time-bounded collection in full, and nothing reports an error because
// the task still completes.
func TestStatefulResponseParserKeepsPartialPageOnFinish(t *testing.T) {
	kept := rawItems(`{"id":1}`, `{"id":2}`)
	parser := newStatefulResponseParser(
		func(*http.Response) ([]json.RawMessage, errors.Error) {
			return kept, ErrFinishCollect
		},
		nil,
		nil,
	)

	items, err := parser(&http.Response{})

	assert.Equal(t, kept, items)
	assert.True(t, errors.Is(err, ErrFinishCollect), "the stop signal must still reach fetchAsync")
}

// The same, with the incremental filter active: the prefix is in the window, so the filter keeps it.
func TestStatefulResponseParserKeepsPartialPageWhileFiltering(t *testing.T) {
	createdAfter := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	kept := rawItems(`{"created":"2026-03-01"}`, `{"created":"2026-02-01"}`)
	parser := newStatefulResponseParser(
		func(*http.Response) ([]json.RawMessage, errors.Error) {
			return kept, ErrFinishCollect
		},
		func(item json.RawMessage) (time.Time, errors.Error) {
			var record struct {
				Created string `json:"created"`
			}
			if e := json.Unmarshal(item, &record); e != nil {
				return time.Time{}, errors.Convert(e)
			}
			parsed, e := time.Parse("2006-01-02", record.Created)
			return parsed, errors.Convert(e)
		},
		&createdAfter,
	)

	items, err := parser(&http.Response{})

	assert.Equal(t, kept, items)
	assert.True(t, errors.Is(err, ErrFinishCollect))
}

// An empty page plus the stop signal has nothing to keep, but must still stop.
func TestStatefulResponseParserStopsOnEmptyFinishedPage(t *testing.T) {
	parser := newStatefulResponseParser(
		func(*http.Response) ([]json.RawMessage, errors.Error) {
			return nil, ErrFinishCollect
		},
		nil,
		nil,
	)

	items, err := parser(&http.Response{})

	assert.Empty(t, items)
	assert.True(t, errors.Is(err, ErrFinishCollect))
}

// A genuine failure is still a failure, and must not be mistaken for a stop signal.
func TestStatefulResponseParserPropagatesRealErrors(t *testing.T) {
	boom := errors.Default.New("boom")
	parser := newStatefulResponseParser(
		func(*http.Response) ([]json.RawMessage, errors.Error) {
			return rawItems(`{"id":1}`), boom
		},
		nil,
		nil,
	)

	items, err := parser(&http.Response{})

	assert.Nil(t, items, "records must be discarded when the page could not be parsed")
	assert.Equal(t, boom, err)
	assert.False(t, errors.Is(err, ErrFinishCollect))
}

func TestStatefulResponseParserPassesThroughWhenNotFinished(t *testing.T) {
	all := rawItems(`{"id":1}`, `{"id":2}`)
	parser := newStatefulResponseParser(
		func(*http.Response) ([]json.RawMessage, errors.Error) {
			return all, nil
		},
		nil,
		nil,
	)

	items, err := parser(&http.Response{})

	assert.Equal(t, all, items)
	assert.Nil(t, err)
}

// Guards the extraction: the incremental filter still trims a page whose tail predates the last
// successful collection, and still stops once it reaches such a record.
func TestStatefulResponseParserTrimsRecordsBeforeCreatedAfter(t *testing.T) {
	createdAfter := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	parser := newStatefulResponseParser(
		func(*http.Response) ([]json.RawMessage, errors.Error) {
			return rawItems(
				`{"created":"2026-03-01"}`,
				`{"created":"2026-02-15"}`,
				`{"created":"2026-01-20"}`,
			), nil
		},
		func(item json.RawMessage) (time.Time, errors.Error) {
			var record struct {
				Created string `json:"created"`
			}
			if e := json.Unmarshal(item, &record); e != nil {
				return time.Time{}, errors.Convert(e)
			}
			parsed, e := time.Parse("2006-01-02", record.Created)
			return parsed, errors.Convert(e)
		},
		&createdAfter,
	)

	items, err := parser(&http.Response{})

	assert.Equal(t, rawItems(`{"created":"2026-03-01"}`, `{"created":"2026-02-15"}`), items)
	assert.True(t, errors.Is(err, ErrFinishCollect))
}
