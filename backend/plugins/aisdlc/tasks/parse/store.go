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

package parse

// Store is the in-memory shape of an idempotent upsert.
type Store struct {
	Observations map[string]Observation
	WorkItems    map[string]WorkItem
	Executions   map[string]Execution
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{
		Observations: map[string]Observation{},
		WorkItems:    map[string]WorkItem{},
		Executions:   map[string]Execution{},
	}
}

// Apply upserts a draft set. A second apply of the same identities replaces
// rows and does not add observations.
func (s *Store) Apply(set DraftSet) {
	for _, item := range set.WorkItems {
		if existing, ok := s.WorkItems[item.ID]; ok && existing.MatchStatus == MatchUnique {
			item.MatchStatus = existing.MatchStatus
			item.DomainIssueID = existing.DomainIssueID
		}
		s.WorkItems[item.ID] = item
	}
	for _, exec := range set.Executions {
		s.Executions[exec.ID] = exec
	}
	for _, obs := range set.Observations {
		s.Observations[obs.ID] = obs
	}
}

// ApplyIssueMatch sets the match status from how many existing issue rows share the key.
// It never creates an issue. Zero and many matches stay unresolved.
func ApplyIssueMatch(item *WorkItem, issueIDs []string) {
	switch len(issueIDs) {
	case 1:
		item.MatchStatus = MatchUnique
		item.DomainIssueID = issueIDs[0]
	case 0:
		item.MatchStatus = MatchUnresolved
		item.DomainIssueID = ""
	default:
		item.MatchStatus = MatchAmbiguous
		item.DomainIssueID = ""
	}
}
