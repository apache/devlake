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

	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
	"github.com/stretchr/testify/assert"
)

// TestStatusFromStateType pins the mapping for every Linear WorkflowState.type
// value. Linear's state types are standardized; "triage" is the inbox state
// issues land in before they are accepted, so it maps to TODO. Any genuinely
// unknown type falls back to OTHER.
func TestStatusFromStateType(t *testing.T) {
	cases := map[string]string{
		"backlog":   ticket.TODO,
		"unstarted": ticket.TODO,
		"triage":    ticket.TODO,
		"started":   ticket.IN_PROGRESS,
		"completed": ticket.DONE,
		"canceled":  ticket.DONE,
		"":          ticket.OTHER,
		"something": ticket.OTHER,
	}
	for stateType, want := range cases {
		assert.Equal(t, want, StatusFromStateType(stateType), "state type %q", stateType)
	}
}
