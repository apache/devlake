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
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/log"
	"github.com/apache/devlake/core/models/domainlayer/ticket"
	"github.com/apache/devlake/plugins/youtrack/models"
)

// parseJsonArrayResponse is the ResponseParser for YouTrack list endpoints
// (issues, comments): the body is one JSON array with no total count, so
// undetermined pagination walks $skip/$top until a short page.
func parseJsonArrayResponse(res *http.Response) ([]json.RawMessage, errors.Error) {
	blob, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, errors.Convert(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(blob, &items); err != nil {
		return nil, errors.Convert(err)
	}
	return items, nil
}

// apiCustomField is one element of an issue's `customFields` array. Value is
// kept raw because its shape depends on $type: a bundle-element object
// (enum/state), a user object (user[1]), an array (multi-value fields), a
// primitive (simple fields), or an epoch-ms number (date fields).
type apiCustomField struct {
	Id    string          `json:"id"`
	Name  string          `json:"name"`
	Type  string          `json:"$type"`
	Value json.RawMessage `json:"value"`
}

// apiBundleElement is the value of a SingleEnumIssueCustomField or
// StateIssueCustomField. IsResolved exists only on StateBundleElement.
type apiBundleElement struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	IsResolved bool   `json:"isResolved"`
	Type       string `json:"$type"`
}

// apiUser is a user reference: an issue's reporter/updater, or the value of
// a SingleUserIssueCustomField.
type apiUser struct {
	Id        string `json:"id"`
	Login     string `json:"login"`
	FullName  string `json:"fullName"`
	Email     string `json:"email"`
	AvatarUrl string `json:"avatarUrl"`
	Guest     bool   `json:"guest"`
	Type      string `json:"$type"`
}

// Default field names for the six scope-config slots. The core
// four fall back to these when their slot is empty; the optional two are
// off when empty.
const (
	defaultTypeField     = "Type"
	defaultStateField    = "State"
	defaultPriorityField = "Priority"
	defaultAssigneeField = "Assignee"
)

// Expected IssueCustomField $type per slot. Multi-value fields
// carry distinct $types (MultiEnumIssueCustomField, MultiUserIssueCustomField),
// so matching on these sets rejects them defensively without a separate check.
// A State field with a state machine attached is reported as
// StateMachineIssueCustomField; its value is still a StateBundleElement.
const (
	typeEnumCF         = "SingleEnumIssueCustomField"
	typeStateCF        = "StateIssueCustomField"
	typeStateMachineCF = "StateMachineIssueCustomField"
	typeUserCF         = "SingleUserIssueCustomField"
	typeSimpleCF       = "SimpleIssueCustomField"
	typeDateCF         = "DateIssueCustomField"
	typeMultiEnum      = "MultiEnumIssueCustomField"
	typeMultiUser      = "MultiUserIssueCustomField"
)

// StdTypeFor derives the standard ticket.Issue.Type: the configured mapping
// when present, else ToUpper(original) (Jira precedent). An empty
// type name stays empty.
func StdTypeFor(typeMappings map[string]string, typeName string) string {
	if typeName == "" {
		return ""
	}
	if mapped, ok := typeMappings[typeName]; ok && mapped != "" {
		return mapped
	}
	return strings.ToUpper(typeName)
}

// StdStatusFor derives the standard ticket.Issue.Status: the configured
// mapping when present, else the zero-config default isResolved ? DONE :
// TODO. IN_PROGRESS/OTHER require an explicit mapping. An empty
// state name stays empty.
func StdStatusFor(statusMappings map[string]string, stateName string, isResolved bool) string {
	if stateName == "" {
		return ""
	}
	if mapped, ok := statusMappings[stateName]; ok && mapped != "" {
		return mapped
	}
	if isResolved {
		return ticket.DONE
	}
	return ticket.TODO
}

// leadTimeMinutesFor computes Resolved − Created in whole minutes, or nil
// when the issue is unresolved. A resolution preceding creation (clock skew
// or migrated data) yields nil rather than a negative duration cast to uint
// (Linear's guard).
func leadTimeMinutesFor(created time.Time, resolved *time.Time) *uint {
	if resolved == nil || resolved.Before(created) {
		return nil
	}
	minutes := uint(resolved.Sub(created).Minutes())
	return &minutes
}

// projectUrl computes the web URL of a YouTrack project from the connection
// endpoint (validated to end in `/api` at save time).
func projectUrl(endpoint string, shortName string) string {
	return strings.TrimSuffix(endpoint, "/api") + "/projects/" + shortName
}

// issueUrl computes the web URL of a YouTrack issue likewise
// ({base}/issue/{idReadable}).
func issueUrl(endpoint string, idReadable string) string {
	return strings.TrimSuffix(endpoint, "/api") + "/issue/" + idReadable
}

// issueCustomFields holds the values resolved from an issue's customFields
// array for the six slots. Fields stay zero-valued when their slot's field
// is absent (warn-never-fail).
type issueCustomFields struct {
	StateName       string
	StateIsResolved bool
	TypeName        string
	PriorityName    string
	Assignee        *apiUser
	StoryPoint      *float64
	DueDate         *time.Time
}

// customFieldResolver resolves the six configurable field-name slots against
// issue customFields arrays. The core four slots fall back to their default
// names when unconfigured; story points and due date are off when
// unconfigured. A configured-but-absent field (or a name matched with the
// wrong $type) warns once per run and never fails.
type customFieldResolver struct {
	stateField      string
	typeField       string
	priorityField   string
	assigneeField   string
	storyPointField string
	dueDateField    string
	logger          log.Logger
	warned          map[string]bool
}

func newCustomFieldResolver(scopeConfig *models.YoutrackScopeConfig, logger log.Logger) *customFieldResolver {
	r := &customFieldResolver{
		storyPointField: scopeConfig.StoryPointField,
		dueDateField:    scopeConfig.DueDateField,
		logger:          logger,
		warned:          map[string]bool{},
	}
	r.typeField = valueOrDefault(scopeConfig.TypeField, defaultTypeField)
	r.stateField = valueOrDefault(scopeConfig.StateField, defaultStateField)
	r.priorityField = valueOrDefault(scopeConfig.PriorityField, defaultPriorityField)
	r.assigneeField = valueOrDefault(scopeConfig.AssigneeField, defaultAssigneeField)
	return r
}

func valueOrDefault(configured, fallback string) string {
	if configured != "" {
		return configured
	}
	return fallback
}

// resolve extracts the six slot values from one issue's customFields array.
func (r *customFieldResolver) resolve(customFields []apiCustomField) (*issueCustomFields, errors.Error) {
	out := &issueCustomFields{}
	if cf := r.findField(customFields, r.stateField, typeStateCF, typeStateMachineCF); cf != nil {
		var value apiBundleElement
		if err := unmarshalFieldValue(cf, &value); err != nil {
			return nil, err
		}
		out.StateName = value.Name
		out.StateIsResolved = value.IsResolved
	}
	if cf := r.findField(customFields, r.typeField, typeEnumCF); cf != nil {
		var value apiBundleElement
		if err := unmarshalFieldValue(cf, &value); err != nil {
			return nil, err
		}
		out.TypeName = value.Name
	}
	if cf := r.findField(customFields, r.priorityField, typeEnumCF); cf != nil {
		var value apiBundleElement
		if err := unmarshalFieldValue(cf, &value); err != nil {
			return nil, err
		}
		out.PriorityName = value.Name
	}
	if cf := r.findField(customFields, r.assigneeField, typeUserCF); cf != nil {
		var value apiUser
		if err := unmarshalFieldValue(cf, &value); err != nil {
			return nil, err
		}
		out.Assignee = &value
	}
	// optional slots: off when unconfigured
	if r.storyPointField != "" {
		if cf := r.findField(customFields, r.storyPointField, typeSimpleCF); cf != nil && !isNullValue(cf) {
			if value, ok := numberFieldValue(cf); ok {
				out.StoryPoint = &value
			} else {
				r.warnBadValueShape(cf, "a number")
			}
		}
	}
	if r.dueDateField != "" {
		if cf := r.findField(customFields, r.dueDateField, typeDateCF); cf != nil && !isNullValue(cf) {
			if epochMillis, ok := numberFieldValue(cf); ok {
				dueDate := time.UnixMilli(int64(epochMillis)).UTC()
				out.DueDate = &dueDate
			} else {
				r.warnBadValueShape(cf, "epoch milliseconds")
			}
		}
	}
	return out, nil
}

// numberFieldValue decodes a matched field's value as a JSON number. A
// matched field holding a non-number (e.g. a string simple field named in
// the story-point slot) is reported not-ok, never an error.
func numberFieldValue(cf *apiCustomField) (float64, bool) {
	var value interface{}
	if err := json.Unmarshal(cf.Value, &value); err != nil {
		return 0, false
	}
	number, ok := value.(float64)
	return number, ok
}

// warnBadValueShape warns once per run when a matched field's value has the
// wrong shape — warn-never-fail applies to value shapes too.
func (r *customFieldResolver) warnBadValueShape(cf *apiCustomField, expected string) {
	key := "value:" + cf.Name
	if r.warned[key] || r.logger == nil {
		return
	}
	r.warned[key] = true
	r.logger.Warn(nil, "custom field %q matched but its value is not %s — treated as absent", cf.Name, expected)
}

// isNullValue reports whether a matched field's value is JSON null (field
// present, value unset) — read as "empty", never as a zero timestamp/number.
func isNullValue(cf *apiCustomField) bool {
	return len(cf.Value) == 0 || string(cf.Value) == "null"
}

// findField returns the custom field whose name matches the effective slot
// name AND whose $type is one of the expected ones. A name matched with the
// wrong $type is treated as absent — that includes multi-value
// fields, whose $types never appear in any slot's expected set.
func (r *customFieldResolver) findField(customFields []apiCustomField, name string, expectedTypes ...string) *apiCustomField {
	nameMatched := false
	for i := range customFields {
		cf := &customFields[i]
		if cf.Name != name {
			continue
		}
		nameMatched = true
		for _, expected := range expectedTypes {
			if cf.Type == expected {
				return cf
			}
		}
	}
	r.warnOnce(name, nameMatched)
	return nil
}

// warnOnce logs a per-run warning for a configured-but-absent field — the
// renamed-field trap the plugin exists to survive (warn, never fail).
func (r *customFieldResolver) warnOnce(name string, nameMatchedWrongType bool) {
	if r.warned[name] || r.logger == nil {
		return
	}
	r.warned[name] = true
	if nameMatchedWrongType {
		r.logger.Warn(nil, "custom field %q found but with an unexpected $type — treated as absent; check the scope config's field-name slots", name)
	} else {
		r.logger.Warn(nil, "custom field %q not found on this scope's issues — the dedicated columns stay empty; check the scope config's field-name slots (a renamed field or missing permissions, never a failed run)", name)
	}
}

// unmarshalFieldValue decodes a matched field's value. Callers guard null
// values with isNullValue where the zero value would be observable.
func unmarshalFieldValue(cf *apiCustomField, target interface{}) errors.Error {
	if isNullValue(cf) {
		return nil
	}
	return errors.Convert(json.Unmarshal(cf.Value, target))
}
