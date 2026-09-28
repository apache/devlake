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
	"encoding/json"
	"testing"
	"time"

	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdTypeFor(t *testing.T) {
	mappings := map[string]string{"Story": ticket.REQUIREMENT, "Bug": ticket.BUG}

	assert.Equal(t, ticket.REQUIREMENT, StdTypeFor(mappings, "Story"), "configured mapping wins")
	assert.Equal(t, "TECHDEBT", StdTypeFor(mappings, "TechDebt"), "unmapped type falls back to ToUpper (Jira precedent)")
	assert.Equal(t, "SUB-TASK", StdTypeFor(nil, "Sub-task"), "unmapped with no config at all")
	assert.Equal(t, "", StdTypeFor(mappings, ""), "empty type stays empty")
}

func TestStdStatusFor(t *testing.T) {
	mappings := map[string]string{"In Progress": ticket.IN_PROGRESS}

	assert.Equal(t, ticket.IN_PROGRESS, StdStatusFor(mappings, "In Progress", false), "configured mapping wins")
	assert.Equal(t, ticket.DONE, StdStatusFor(mappings, "Closed", true), "unmapped resolved state -> DONE (zero-config)")
	assert.Equal(t, ticket.TODO, StdStatusFor(mappings, "Backlog", false), "unmapped unresolved state -> TODO (zero-config)")
	assert.Equal(t, ticket.DONE, StdStatusFor(nil, "Declined", true), "zero-config with no config at all")
	assert.Equal(t, "", StdStatusFor(mappings, "", false), "empty state stays empty")
}

func TestLeadTimeMinutesFor(t *testing.T) {
	created := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	resolved := time.Date(2026, 9, 1, 12, 30, 0, 0, time.UTC)

	assert.Nil(t, leadTimeMinutesFor(created, nil), "unresolved issue has no lead time")
	require.NotNil(t, leadTimeMinutesFor(created, &resolved))
	assert.Equal(t, uint(150), *leadTimeMinutesFor(created, &resolved))

	before := created.Add(-time.Hour)
	assert.Nil(t, leadTimeMinutesFor(created, &before), "resolution preceding creation yields nil, never negative-as-uint")
}

// field builds an apiCustomField with a JSON-encoded value.
func field(t *testing.T, name, typ string, value interface{}) apiCustomField {
	t.Helper()
	var raw json.RawMessage
	if value != nil {
		var err error
		raw, err = json.Marshal(value)
		require.NoError(t, err)
	} else {
		raw = json.RawMessage("null")
	}
	return apiCustomField{Id: "110-1", Name: name, Type: typ, Value: raw}
}

func TestCustomFieldResolverRenamedField(t *testing.T) {
	// the trap the plugin exists for: the instance calls its type field
	// "Client type", not "Type"
	scopeConfig := &models.YoutrackScopeConfig{TypeField: "Client type"}
	resolver := newCustomFieldResolver(scopeConfig, nil)

	fields := []apiCustomField{
		field(t, "Client type", typeEnumCF, map[string]interface{}{"name": "Bug", "id": "92-5", "$type": "EnumBundleElement"}),
		field(t, "State", typeStateCF, map[string]interface{}{"name": "Closed", "isResolved": true, "id": "94-18", "$type": "StateBundleElement"}),
		field(t, "Priority", typeEnumCF, map[string]interface{}{"name": "Major", "id": "92-16", "$type": "EnumBundleElement"}),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	assert.Equal(t, "Bug", out.TypeName)
	assert.Equal(t, "Closed", out.StateName)
	assert.True(t, out.StateIsResolved)
	assert.Equal(t, "Major", out.PriorityName)
}

func TestCustomFieldResolverAcceptsStateMachineField(t *testing.T) {
	// a State field with a state machine attached is reported as
	// StateMachineIssueCustomField; it must resolve like a plain state field
	resolver := newCustomFieldResolver(&models.YoutrackScopeConfig{}, nil)
	fields := []apiCustomField{
		field(t, "State", typeStateMachineCF, map[string]interface{}{"name": "Done", "isResolved": true, "id": "94-7", "$type": "StateBundleElement"}),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	assert.Equal(t, "Done", out.StateName)
	assert.True(t, out.StateIsResolved)
}

func TestCustomFieldResolverDefaults(t *testing.T) {
	// empty slots fall back to the default names for the core four
	resolver := newCustomFieldResolver(&models.YoutrackScopeConfig{}, nil)
	fields := []apiCustomField{
		field(t, "Type", typeEnumCF, map[string]interface{}{"name": "Task"}),
		field(t, "State", typeStateCF, map[string]interface{}{"name": "To Do"}),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	assert.Equal(t, "Task", out.TypeName)
	assert.Equal(t, "To Do", out.StateName)
	assert.False(t, out.StateIsResolved)
	assert.Nil(t, out.Assignee)
	assert.Nil(t, out.StoryPoint, "story points are off when the slot is empty")
	assert.Nil(t, out.DueDate, "due date is off when the slot is empty")
}

func TestCustomFieldResolverWrongTypeTreatedAsAbsent(t *testing.T) {
	// a text field renamed to "Type" must not be read as the type field
	resolver := newCustomFieldResolver(&models.YoutrackScopeConfig{}, nil)
	fields := []apiCustomField{
		field(t, "Type", typeSimpleCF, "not a bundle element"),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	assert.Equal(t, "", out.TypeName)
}

func TestCustomFieldResolverMultiValueAssigneeSkipped(t *testing.T) {
	// the live instance's Assignee is user[*] — multi-value fields are
	// ineligible for every slot, so the dedicated columns stay
	// empty and the run proceeds
	resolver := newCustomFieldResolver(&models.YoutrackScopeConfig{}, nil)
	fields := []apiCustomField{
		field(t, "Assignee", typeMultiUser, []map[string]interface{}{
			{"id": "1-1", "login": "user-1", "fullName": "User 1", "$type": "User"},
		}),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	assert.Nil(t, out.Assignee)
}

func TestCustomFieldResolverSingleUserAssignee(t *testing.T) {
	resolver := newCustomFieldResolver(&models.YoutrackScopeConfig{}, nil)
	fields := []apiCustomField{
		field(t, "Assignee", typeUserCF, map[string]interface{}{"id": "1-1", "login": "user-1", "fullName": "User 1", "$type": "User"}),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	require.NotNil(t, out.Assignee)
	assert.Equal(t, "1-1", out.Assignee.Id)
	assert.Equal(t, "User 1", out.Assignee.FullName)
}

func TestCustomFieldResolverOptionalSlots(t *testing.T) {
	scopeConfig := &models.YoutrackScopeConfig{
		StoryPointField: "Days in stage",
		DueDateField:    "Deadline",
	}
	resolver := newCustomFieldResolver(scopeConfig, nil)
	fields := []apiCustomField{
		field(t, "Days in stage", typeSimpleCF, 119),
		field(t, "Deadline", typeDateCF, int64(1754049600000)),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	require.NotNil(t, out.StoryPoint)
	assert.Equal(t, float64(119), *out.StoryPoint)
	require.NotNil(t, out.DueDate)
	assert.Equal(t, time.UnixMilli(1754049600000).UTC(), *out.DueDate)
}

func TestCustomFieldResolverNullValuesAreEmptyNotErrors(t *testing.T) {
	scopeConfig := &models.YoutrackScopeConfig{DueDateField: "Deadline"}
	resolver := newCustomFieldResolver(scopeConfig, nil)
	fields := []apiCustomField{
		field(t, "State", typeStateCF, nil),
		field(t, "Deadline", typeDateCF, nil),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	assert.Equal(t, "", out.StateName)
	assert.Nil(t, out.DueDate)
}

func TestCustomFieldResolverBadValueShapeWarnsNotFails(t *testing.T) {
	// a string simple field named in the story-point slot: name+$type match,
	// but the value shape is wrong — warn-never-fail applies to values too
	scopeConfig := &models.YoutrackScopeConfig{
		StoryPointField: "Story points",
		DueDateField:    "Deadline",
	}
	resolver := newCustomFieldResolver(scopeConfig, nil)
	fields := []apiCustomField{
		field(t, "Story points", typeSimpleCF, "not a number"),
		field(t, "Deadline", typeDateCF, "next Friday"),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err, "a wrong-shaped value must not fail extraction")
	assert.Nil(t, out.StoryPoint)
	assert.Nil(t, out.DueDate)
}

func TestCustomFieldResolverMixedLanguageStateName(t *testing.T) {
	// mixed-language bundles: a Cyrillic state name resolves like any other
	resolver := newCustomFieldResolver(&models.YoutrackScopeConfig{}, nil)
	fields := []apiCustomField{
		field(t, "State", typeStateCF, map[string]interface{}{"name": "Новый", "isResolved": false}),
	}
	out, err := resolver.resolve(fields)
	require.NoError(t, err)
	assert.Equal(t, "Новый", out.StateName)
	assert.Equal(t, ticket.TODO, StdStatusFor(nil, out.StateName, out.StateIsResolved))
}
