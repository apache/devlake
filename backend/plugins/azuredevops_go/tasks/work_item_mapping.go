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
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"
)

var excludedWorkItemTypes = []string{
	"Shared Parameter",
	"Shared Steps",
	"Test Case",
	"Test Design",
	"Test Plan",
	"Test Suite",
}

var stateCategoryToStatus = map[string]string{
	"Proposed":   "TODO",
	"InProgress": "IN_PROGRESS",
	"Resolved":   "IN_PROGRESS",
	"Completed":  "DONE",
}

var commonTypeDefaults = map[string]string{
	"Epic":                  "REQUIREMENT",
	"Feature":               "REQUIREMENT",
	"User Story":            "REQUIREMENT",
	"Product Backlog Item": "REQUIREMENT",
	"Requirement":           "REQUIREMENT",
	"Bug":                   "BUG",
	"Task":                  "TASK",
	"Issue":                 "TASK",
	"Impediment":            "TASK",
	"Change Request":        "REQUIREMENT",
	"Risk":                  "INCIDENT",
	"Review":                "TASK",
}

var templateTypeDefaults = map[string]map[string]string{
	"Basic": {
		"Epic":  "REQUIREMENT",
		"Issue": "TASK",
		"Task":  "TASK",
	},
	"Agile": {
		"Epic":       "REQUIREMENT",
		"Feature":    "REQUIREMENT",
		"User Story": "REQUIREMENT",
		"Bug":        "BUG",
		"Task":       "TASK",
		"Issue":      "TASK",
	},
	"Scrum": {
		"Epic":                  "REQUIREMENT",
		"Feature":               "REQUIREMENT",
		"Product Backlog Item": "REQUIREMENT",
		"Bug":                   "BUG",
		"Task":                  "TASK",
	},
	"CMMI": {
		"Epic":           "REQUIREMENT",
		"Feature":        "REQUIREMENT",
		"Requirement":    "REQUIREMENT",
		"Bug":            "BUG",
		"Task":           "TASK",
		"Change Request": "REQUIREMENT",
		"Risk":           "INCIDENT",
		"Review":         "TASK",
	},
}

var defaultStatusByName = map[string]string{
	"New":         "TODO",
	"Proposed":    "TODO",
	"To Do":       "TODO",
	"Active":      "IN_PROGRESS",
	"Doing":       "IN_PROGRESS",
	"Committed":   "IN_PROGRESS",
	"In Progress": "IN_PROGRESS",
	"Resolved":    "IN_PROGRESS",
	"Closed":      "DONE",
	"Done":        "DONE",
	"Removed":     "DONE",
	"Cut":         "DONE",
}

var changelogFieldNames = map[string]string{
	"System.State":                   "status",
	"System.AssignedTo":              "assignee",
	"System.IterationPath":           "sprint",
	"System.AreaPath":                "component",
	"Microsoft.VSTS.Common.Priority": "priority",
	"System.Title":                   "title",
	"System.Tags":                    "tags",
}

func projectWorkItemsWIQL(projectId string, createdAfter, changedSince *time.Time) string {
	clauses := []string{
		"SELECT [System.Id] FROM WorkItems",
		fmt.Sprintf("WHERE [System.TeamProject] = '%s'", escapeWIQL(projectId)),
		"AND " + wiqlExcludeTypesClause(),
	}
	if createdAfter != nil {
		clauses = append(clauses, fmt.Sprintf("AND [System.CreatedDate] >= '%s'", formatWIQLDate(*createdAfter)))
	}
	if changedSince != nil {
		clauses = append(clauses, fmt.Sprintf("AND [System.ChangedDate] >= '%s'", formatWIQLDate(*changedSince)))
	}
	clauses = append(clauses, "ORDER BY [System.ChangedDate] DESC")
	return strings.Join(clauses, " ")
}

func wiqlExcludeTypesClause() string {
	parts := make([]string, len(excludedWorkItemTypes))
	for i, workItemType := range excludedWorkItemTypes {
		parts[i] = fmt.Sprintf("[System.WorkItemType] <> '%s'", escapeWIQL(workItemType))
	}
	return strings.Join(parts, " AND ")
}

func escapeWIQL(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func formatWIQLDate(value time.Time) string {
	return value.UTC().Format("2006-01-02")
}

func shouldConvertWorkItem(workItemType string) bool {
	if workItemType == "" {
		return false
	}
	for _, excluded := range excludedWorkItemTypes {
		if workItemType == excluded {
			return false
		}
	}
	return true
}

func resolveWorkItemType(processTemplate, workItemType string, hasParent bool, typeMappings datatypes.JSONMap) string {
	if configured := mappingValue(typeMappings, workItemType, "standardType", "standard_type"); configured != "" {
		return strings.ToUpper(configured)
	}
	stdType := ""
	if defaults, ok := templateTypeDefaults[strings.TrimSpace(processTemplate)]; ok {
		stdType = defaults[workItemType]
	}
	if stdType == "" {
		stdType = commonTypeDefaults[workItemType]
	}
	if stdType == "" {
		stdType = "TASK"
	}
	if hasParent && stdType == "TASK" {
		return "SUBTASK"
	}
	return stdType
}

func resolveWorkItemStatus(workItemType, stateName string, statusMappings datatypes.JSONMap, categories map[string]map[string]string) string {
	if stateName == "" {
		return "OTHER"
	}
	if configured := mappingValue(statusMappings, stateName, "standardStatus", "standard_status"); configured != "" {
		return strings.ToUpper(configured)
	}
	if category := categories[workItemType][stateName]; category != "" {
		if status, ok := stateCategoryToStatus[category]; ok {
			return status
		}
		return "OTHER"
	}
	if status, ok := defaultStatusByName[stateName]; ok {
		return status
	}
	return "OTHER"
}

func mappingValue(mappings datatypes.JSONMap, name string, keys ...string) string {
	if mappings == nil {
		return ""
	}
	raw, ok := mappings[name]
	if !ok || raw == nil {
		return ""
	}
	entry, ok := raw.(map[string]interface{})
	if !ok {
		return ""
	}
	for _, key := range keys {
		if value := asString(entry[key]); value != "" {
			return value
		}
	}
	return ""
}

func iterationPathKey(path, projectId string) string {
	segments := iterationSegments(path)
	projectSegments := iterationSegments(projectId)
	if len(segments) < len(projectSegments) || len(projectSegments) == 0 {
		return ""
	}
	for i, segment := range projectSegments {
		if segments[i] != segment {
			return ""
		}
	}
	remainder := segments[len(projectSegments):]
	if len(remainder) > 0 && strings.EqualFold(remainder[0], "iteration") {
		remainder = remainder[1:]
	}
	if len(remainder) == 0 {
		return ""
	}
	return strings.Join(append(append([]string{}, projectSegments...), remainder...), `\`)
}

func iterationSegments(path string) []string {
	normalized := strings.Trim(strings.ReplaceAll(path, "/", `\`), `\`)
	if normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, `\`)
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func areaPathToComponent(areaPath string) string {
	segments := iterationSegments(areaPath)
	if len(segments) == 0 {
		return ""
	}
	return strings.Join(segments, `\`)
}

func mapTestOutcome(outcome string) string {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "":
		return "PENDING"
	case "passed", "pass":
		return "SUCCESS"
	case "failed", "error", "timeout", "blocked", "notapplicable":
		return "FAILED"
	case "inprogress", "in progress", "running":
		return "IN_PROGRESS"
	default:
		return "PENDING"
	}
}

func identityFromRef(value interface{}) (string, string) {
	switch typed := value.(type) {
	case nil:
		return "", ""
	case string:
		return typed, typed
	case map[string]interface{}:
		id := asString(typed["uniqueName"])
		if id == "" {
			id = asString(typed["id"])
		}
		return id, asString(typed["displayName"])
	default:
		text := asString(typed)
		return text, text
	}
}

func asString(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		if typed == math.Trunc(typed) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(typed)
	}
}

func asInt(value interface{}) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		parsed, _ := strconv.Atoi(asString(value))
		return parsed
	}
}

func asFloatPtr(value interface{}) *float64 {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		return &typed
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return nil
		}
		return &parsed
	default:
		return nil
	}
}

func hoursToMinutes(value interface{}) *int64 {
	hours := asFloatPtr(value)
	if hours == nil {
		return nil
	}
	minutes := int64(math.Round(*hours * 60))
	return &minutes
}

func parseAdoTime(value interface{}) *time.Time {
	text := asString(value)
	if text == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.9999999Z07:00", "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05"} {
		parsed, err := time.Parse(layout, text)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func isAdoOpenRevisionSentinel(value *time.Time) bool {
	return value != nil && value.Year() >= 9999
}
