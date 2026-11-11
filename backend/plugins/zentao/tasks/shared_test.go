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
	"reflect"
	"testing"
)

func Test_convertIssueURL(t *testing.T) {
	type args struct {
		apiURL    string
		issueType string
		id        int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"bug",
			args{"http://example.com/api.php/v1/products/16/bugs?limit=100&page=1", "bug", 1169},
			"http://example.com/bug-view-1169.html",
		},
		{
			"bug with prefix",
			args{"http://example.com/prefix1/prefix2/api.php/v1/products/16/bugs?limit=100&page=1", "bug", 1169},
			"http://example.com/prefix1/prefix2/bug-view-1169.html",
		},
		{
			"story",
			args{"http://example.com/api.php/v1/executions/41/stories?limit=100&page=1&status=allstory", "story", 4513},
			"http://example.com/story-view-4513.html",
		},
		{
			"story with prefix",
			args{"http://example.com/prefix1/prefix2/api.php/v1/executions/41/stories?limit=100&page=1&status=allstory", "story", 4513},
			"http://example.com/prefix1/prefix2/story-view-4513.html",
		},
		{
			"task",
			args{"http://example.com/api.php/v1/executions/39/tasks?limit=100&page=1", "task", 2381},
			"http://example.com/task-view-2381.html",
		},
		{
			"task with prefix",
			args{"http://example.com/prefix1/prefix2/api.php/v1/executions/41/stories?limit=100&page=1&status=allstory", "task", 2381},
			"http://example.com/prefix1/prefix2/task-view-2381.html",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertIssueURL(tt.args.apiURL, tt.args.issueType, tt.args.id); got != tt.want {
				t.Errorf("convertIssueURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractIdFromLogComment(t *testing.T) {
	type args struct {
		logCommentType string
		comment        string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "common-1",
			args: args{
				logCommentType: "something-wrong",
				comment:        "random strings",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "story-1",
			args: args{
				logCommentType: "story",
				comment:        "story #<a href='\\/story-view-4590.json'  >4590<\\/a>\\n,<a href='\\/story-view-4572.json'  >4572<\\/a>\\n",
			},
			want:    []string{"4590", "4572"},
			wantErr: false,
		},
		{
			name: "story-2",
			args: args{
				logCommentType: "story",
				comment:        "story #<a href='\\/story-view-4572.json'  >4572<\\/a>\\n,<a href='\\/story-view-4591.json'  >4591<\\/a>\\n \\u6d4b\\u8bd5\\u4e24\\u4e2a\\u5173\\u8054\\u5173\\u7cfb\\u662f\\u5426\\u90fd\\u5199\\u8fdbissuerepocommit\\u8868",
			},
			want:    []string{"4572", "4591"},
			wantErr: false,
		},
		{
			name: "story-3",
			args: args{
				logCommentType: "story",
				comment:        "story #<a href='\\/story-view-4590.json'  >4590<\\/a>",
			},
			want:    []string{"4590"},
			wantErr: false,
		},
		{
			name: "bug-1",
			args: args{
				logCommentType: "bug",
				comment:        "\"bug #<a href='\\/bug-view-6119.json'  >6119<\\/a>\\n,<a href='\\/bug-view-6118.json'  >6118<\\/a>\\n,<a href='\\/bug-view-6117.json'  >6117<\\/a>\\n,<a href='\\/bug-view-6121.json'  >6121<\\/a>\\n",
			},
			want:    []string{"6119", "6118", "6117", "6121"},
			wantErr: false,
		},
		{
			name: "task-1",
			args: args{
				logCommentType: "task",
				comment:        "task #<a href='\\/task-view-004.json'  >004<\\/a>\\n\\uff0c\\u7985\\u9053\\u4efb\\u52a1\\u6d4b\\u8bd5",
			},
			want:    []string{"004"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractIdFromLogComment(tt.args.logCommentType, tt.args.comment)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractIdFromLogComment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractIdFromLogComment() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getZentaoWebURL(t *testing.T) {
	type args struct {
		endpoint string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "without-zentao",
			args:    args{endpoint: "http://54.158.1.10:30001/api.php/v1/"},
			want:    "http://54.158.1.10:30001",
			wantErr: false,
		},
		{
			name:    "with-zentao",
			args:    args{endpoint: "http://54.158.1.10:30001/zentao/api.php/v1/"},
			want:    "http://54.158.1.10:30001/zentao",
			wantErr: false,
		},
		{
			name:    "with-others",
			args:    args{endpoint: "http://54.158.1.10:30001/abc/api.php/v1/"},
			want:    "http://54.158.1.10:30001/abc",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getZentaoHomePage(tt.args.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("getZentaoHomePage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getZentaoHomePage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getSeverity(t *testing.T) {
	tests := []struct {
		severity int
		want     string
	}{
		{1, "1"},
		{3, "3"},
		{4, "4"},
		{10, "10"},
	}
	for _, tt := range tests {
		if got := getSeverity(tt.severity); got != tt.want {
			t.Errorf("getSeverity(%d) = %q, want %q", tt.severity, got, tt.want)
		}
	}
}
