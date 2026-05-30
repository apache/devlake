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

package models

import "testing"

func TestGhCopilotScope_TableName(t *testing.T) {
	s := GhCopilotScope{}
	expected := "_tool_copilot_scopes"
	if s.TableName() != expected {
		t.Errorf("TableName() = %v, want %v", s.TableName(), expected)
	}
}

func TestGhCopilotScope_ScopeName(t *testing.T) {
	tests := []struct {
		name         string
		id           string
		organization string
		expected     string
	}{
		{"returns Id when set", "my-org", "fallback-org", "my-org"},
		{"returns Organization when Id empty", "", "my-org", "my-org"},
		{"returns empty when both empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := GhCopilotScope{
				Id:           tt.id,
				Organization: tt.organization,
			}
			if got := s.ScopeName(); got != tt.expected {
				t.Errorf("ScopeName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGhCopilotScope_ScopeFullName(t *testing.T) {
	s := GhCopilotScope{Id: "test-org"}
	if got := s.ScopeFullName(); got != "test-org" {
		t.Errorf("ScopeFullName() = %v, want test-org", got)
	}
}

func TestGhCopilotScope_BeforeSave_PopulatesName(t *testing.T) {
	tests := []struct {
		name             string
		id               string
		organization     string
		initialName      string
		expectedName     string
		expectedFullName string
	}{
		{
			name:             "populates Name from Id when empty",
			id:               "my-org",
			organization:     "fallback",
			initialName:      "",
			expectedName:     "my-org",
			expectedFullName: "my-org",
		},
		{
			name:             "populates Name from Organization when Id empty",
			id:               "",
			organization:     "my-org",
			initialName:      "",
			expectedName:     "my-org",
			expectedFullName: "my-org",
		},
		{
			name:             "preserves existing Name",
			id:               "org-id",
			organization:     "org",
			initialName:      "custom-name",
			expectedName:     "custom-name",
			expectedFullName: "org-id", // FullName still populated from ScopeName
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &GhCopilotScope{
				Id:           tt.id,
				Organization: tt.organization,
				Name:         tt.initialName,
			}
			err := s.BeforeSave(nil)
			if err != nil {
				t.Errorf("BeforeSave() error = %v, want nil", err)
			}
			if s.Name != tt.expectedName {
				t.Errorf("Name = %v, want %v", s.Name, tt.expectedName)
			}
			if s.FullName != tt.expectedFullName {
				t.Errorf("FullName = %v, want %v", s.FullName, tt.expectedFullName)
			}
		})
	}
}

func TestGhCopilotScope_BeforeSave_BaselinePeriodDays(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"zero defaults to 90", 0, 90},
		{"negative defaults to 90", -10, 90},
		{"below minimum (6) defaults to 90", 6, 90},
		{"minimum valid (7) unchanged", 7, 7},
		{"typical value (90) unchanged", 90, 90},
		{"maximum valid (365) unchanged", 365, 365},
		{"above maximum (400) capped to 365", 400, 365},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &GhCopilotScope{
				Id:                 "test-org",
				BaselinePeriodDays: tt.input,
			}
			err := s.BeforeSave(nil)
			if err != nil {
				t.Errorf("BeforeSave() error = %v, want nil", err)
			}
			if s.BaselinePeriodDays != tt.expected {
				t.Errorf("BaselinePeriodDays = %v, want %v", s.BaselinePeriodDays, tt.expected)
			}
		})
	}
}

func TestGhCopilotScope_ScopeId(t *testing.T) {
	s := GhCopilotScope{Id: "test-id"}
	if got := s.ScopeId(); got != "test-id" {
		t.Errorf("ScopeId() = %v, want test-id", got)
	}
}

func TestGhCopilotScope_ScopeParams(t *testing.T) {
	s := GhCopilotScope{Id: "test-org"}
	s.ConnectionId = 42

	params := s.ScopeParams().(*GhCopilotScopeParams)
	if params.ConnectionId != 42 {
		t.Errorf("ScopeParams().ConnectionId = %v, want 42", params.ConnectionId)
	}
	if params.ScopeId != "test-org" {
		t.Errorf("ScopeParams().ScopeId = %v, want test-org", params.ScopeId)
	}
}
