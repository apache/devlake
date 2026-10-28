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

import (
	"encoding/json"
	"testing"
)

func TestScopeTableName(t *testing.T) {
	s := AgentReadyScope{}
	want := "_tool_agentready_scopes"
	if got := s.TableName(); got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
}

func TestScopeId(t *testing.T) {
	s := AgentReadyScope{FullName: "myorg/myrepo", Name: "myrepo"}
	if got := s.ScopeId(); got != "myorg/myrepo" {
		t.Errorf("ScopeId() = %q, want %q", got, "myorg/myrepo")
	}
	if got := s.ScopeName(); got != "myrepo" {
		t.Errorf("ScopeName() = %q, want %q", got, "myrepo")
	}
	if got := s.ScopeFullName(); got != "myorg/myrepo" {
		t.Errorf("ScopeFullName() = %q, want %q", got, "myorg/myrepo")
	}
}

func TestScopeMarshalJSON(t *testing.T) {
	s := AgentReadyScope{FullName: "myorg/myrepo", Name: "myrepo"}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	if m["id"] != "myorg/myrepo" {
		t.Errorf("MarshalJSON() id = %v, want %q", m["id"], "myorg/myrepo")
	}
}

func TestScopeParams(t *testing.T) {
	s := AgentReadyScope{FullName: "myorg/myrepo"}
	s.ConnectionId = 5
	params := s.ScopeParams().(*AgentReadyApiParams)
	if params.ConnectionId != 5 {
		t.Errorf("ScopeParams().ConnectionId = %d, want 5", params.ConnectionId)
	}
	if params.FullName != "myorg/myrepo" {
		t.Errorf("ScopeParams().FullName = %q, want %q", params.FullName, "myorg/myrepo")
	}
}
