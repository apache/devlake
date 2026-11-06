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
	"testing"

	"github.com/apache/devlake/helpers/pluginhelper/api"
)

func TestBuildWorklogQuery(t *testing.T) {
	tests := []struct {
		name       string
		pager      api.Pager
		startAt    string
		maxResults string
	}{
		{"first page", api.Pager{Page: 1, Skip: 0, Size: 50}, "0", "50"},
		{"second page", api.Pager{Page: 2, Skip: 50, Size: 50}, "50", "50"},
		{"tenth page", api.Pager{Page: 10, Skip: 450, Size: 50}, "450", "50"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pager := tt.pager
			query, err := buildWorklogQuery(&api.RequestData{Pager: &pager})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := query.Get("startAt"); got != tt.startAt {
				t.Errorf("startAt = %q, want %q", got, tt.startAt)
			}
			if got := query.Get("maxResults"); got != tt.maxResults {
				t.Errorf("maxResults = %q, want %q", got, tt.maxResults)
			}
		})
	}
}
