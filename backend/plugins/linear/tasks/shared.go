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
	"github.com/apache/incubator-devlake/core/models/domainlayer/ticket"
)

// priorityLabels maps Linear's integer priority to its human-readable label.
// Linear: 0 = No priority, 1 = Urgent, 2 = High, 3 = Medium, 4 = Low.
var priorityLabels = map[int]string{
	0: "No priority",
	1: "Urgent",
	2: "High",
	3: "Medium",
	4: "Low",
}

// PriorityLabel returns the human-readable label for a Linear priority value.
func PriorityLabel(priority int) string {
	if label, ok := priorityLabels[priority]; ok {
		return label
	}
	return "No priority"
}

// StatusFromStateType maps a Linear WorkflowState.type to a DevLake standard
// issue status. Linear's state types are standardized, so no user-supplied
// mapping is required:
//
//	triage, backlog, unstarted -> TODO
//	started                    -> IN_PROGRESS
//	completed, canceled        -> DONE
//
// "triage" is the inbox state issues land in before they are accepted into a
// workflow; it is treated as not-yet-started (TODO). Any unrecognized type
// falls back to OTHER so unexpected API values surface rather than silently
// masquerading as a known status.
func StatusFromStateType(stateType string) string {
	switch stateType {
	case "triage", "backlog", "unstarted":
		return ticket.TODO
	case "started":
		return ticket.IN_PROGRESS
	case "completed", "canceled":
		return ticket.DONE
	default:
		return ticket.OTHER
	}
}
