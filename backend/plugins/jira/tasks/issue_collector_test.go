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
	"time"
)

func Test_buildJQL(t *testing.T) {
	base := time.Date(2021, 2, 3, 4, 5, 6, 7, time.UTC)
	timeAfter := base
	add48 := base.Add(48 * time.Hour)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	type args struct {
		since    *time.Time
		location *time.Location
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test incremental",
			args: args{
				since:    &add48,
				location: loc,
			},
			want: "updated >= '2021/02/05 12:05' ORDER BY created ASC",
		},
		{
			name: "test incremental",
			args: args{
				since: &timeAfter,
			},
			want: "updated >= '2021/02/02 04:05' ORDER BY created ASC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildJQL(*tt.args.since, tt.args.location); got != tt.want {
				t.Errorf("buildJQL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildBoardIssueJQL(t *testing.T) {
	base := time.Date(2021, 2, 3, 4, 5, 6, 7, time.UTC)
	add48 := base.Add(48 * time.Hour)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	tests := []struct {
		name     string
		boardJQL string
		since    *time.Time
		location *time.Location
		want     string
	}{
		{
			name:     "full sync uses board filter",
			boardJQL: "project = DLK AND status != Done",
			want:     "(project = DLK AND status != Done) ORDER BY created ASC",
		},
		{
			name:     "saved filter order is replaced",
			boardJQL: "project = DLK ORDER BY Rank ASC",
			want:     "(project = DLK) ORDER BY created ASC",
		},
		{
			name:     "incremental sync combines board filter and updated date",
			boardJQL: "project = DLK ORDER BY Rank ASC",
			since:    &add48,
			location: loc,
			want:     "(project = DLK) AND updated >= '2021/02/05 12:05' ORDER BY created ASC",
		},
		{
			name:     "quoted order by is kept",
			boardJQL: `summary ~ "order by" ORDER BY Rank ASC`,
			want:     `(summary ~ "order by") ORDER BY created ASC`,
		},
		{
			name:     "empty board filter keeps stable ordering",
			boardJQL: "",
			want:     "ORDER BY created ASC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildBoardIssueJQL(tt.boardJQL, tt.since, tt.location); got != tt.want {
				t.Errorf("buildBoardIssueJQL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_stripJQLOrderBy(t *testing.T) {
	tests := []struct {
		name string
		jql  string
		want string
	}{
		{
			name: "strips simple order by",
			jql:  "project = DLK ORDER BY Rank ASC",
			want: "project = DLK",
		},
		{
			name: "strips mixed-case order by",
			jql:  "project = DLK order by created DESC",
			want: "project = DLK",
		},
		{
			name: "keeps order by inside double quotes",
			jql:  `summary ~ "order by" ORDER BY Rank ASC`,
			want: `summary ~ "order by"`,
		},
		{
			name: "keeps order by inside single quotes",
			jql:  "description ~ 'order by' ORDER BY created DESC",
			want: "description ~ 'order by'",
		},
		{
			name: "does not strip orderby token",
			jql:  "project = DLK AND component = ORDERBY",
			want: "project = DLK AND component = ORDERBY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripJQLOrderBy(tt.jql); got != tt.want {
				t.Errorf("stripJQLOrderBy() = %v, want %v", got, tt.want)
			}
		})
	}
}
