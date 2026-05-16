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

package pushapiaccess

import "testing"

func TestValidateTable(t *testing.T) {
	cases := []struct {
		name          string
		table         string
		allowedTables string
		wantErr       bool
	}{
		{
			name:          "allows configured application table",
			table:         "commits",
			allowedTables: "commits, issues",
		},
		{
			name:          "rejects invalid table name",
			table:         "commits;drop",
			allowedTables: "commits",
			wantErr:       true,
		},
		{
			name:    "default denies when allowlist unset",
			table:   "commits",
			wantErr: true,
		},
		{
			name:          "rejects internal tables even when allowlisted",
			table:         "_devlake_pipelines",
			allowedTables: "_devlake_pipelines,commits",
			wantErr:       true,
		},
		{
			name:          "rejects tables missing from allowlist",
			table:         "pull_requests",
			allowedTables: "commits,issues",
			wantErr:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTable(tc.table, tc.allowedTables)
			if tc.wantErr && err == nil {
				t.Fatal("expected an error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
