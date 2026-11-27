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

	"github.com/apache/devlake/core/models/domainlayer/code"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/stretchr/testify/assert"
)

func TestConvertGiteeIssueStatus(t *testing.T) {
	cases := map[string]string{
		"open":        ticket.TODO,
		"progressing": ticket.IN_PROGRESS,
		"closed":      ticket.DONE,
		"rejected":    ticket.DONE,
	}
	for state, want := range cases {
		assert.Equal(t, want, convertGiteeIssueStatus(state), state)
	}
}

func TestConvertGiteePrStatus(t *testing.T) {
	cases := map[string]string{
		"open":   code.OPEN,
		"merged": code.MERGED,
		"closed": code.CLOSED,
	}
	for state, want := range cases {
		assert.Equal(t, want, convertGiteePrStatus(state), state)
	}
}
