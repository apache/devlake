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

package raw

import (
	"encoding/json"
	"time"
)

// Datadog's Incident Management API speaks JSON:API: every resource
// arrives as {id, type, attributes}.

type Incident struct {
	Id         string              `json:"id"`
	Type       string              `json:"type"`
	Attributes *IncidentAttributes `json:"attributes"`
}

type IncidentAttributes struct {
	PublicId         int64                      `json:"public_id"`
	Title            string                     `json:"title"`
	State            *string                    `json:"state"`
	Severity         *string                    `json:"severity"`
	IncidentTypeUuid *string                    `json:"incident_type_uuid"`
	Created          *time.Time                 `json:"created"`
	Declared         *time.Time                 `json:"declared"`
	Detected         *time.Time                 `json:"detected"`
	Resolved         *time.Time                 `json:"resolved"`
	Modified         *time.Time                 `json:"modified"`
	IsTest           *bool                      `json:"is_test"`
	CustomerImpacted *bool                      `json:"customer_impacted"`
	TimeToDetect     *int64                     `json:"time_to_detect"`
	TimeToRepair     *int64                     `json:"time_to_repair"`
	TimeToResolve    *int64                     `json:"time_to_resolve"`
	Fields           map[string]FieldAttributes `json:"fields"`
}

// FieldAttributes is one custom-field value. Single-select and textbox
// fields put a string in `value`; multi-select fields put an array of
// strings there, so the value stays raw until someone reads it.
type FieldAttributes struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// Values normalizes both shapes to a slice. An unreadable value yields
// nothing rather than an error: a malformed custom field must not fail
// the whole incident.
func (f FieldAttributes) Values() []string {
	if len(f.Value) == 0 {
		return nil
	}
	var single *string
	if err := json.Unmarshal(f.Value, &single); err == nil {
		if single == nil || *single == "" {
			return nil
		}
		return []string{*single}
	}
	var multiple []string
	if err := json.Unmarshal(f.Value, &multiple); err == nil {
		return multiple
	}
	return nil
}

type IncidentType struct {
	Id         string                  `json:"id"`
	Type       string                  `json:"type"`
	Attributes *IncidentTypeAttributes `json:"attributes"`
}

// Datadog mixes conventions here: `createdAt` is camelCase while
// `is_default` is snake_case. Both are as the API sends them.
type IncidentTypeAttributes struct {
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	Prefix      *string    `json:"prefix"`
	IsDefault   *bool      `json:"is_default"`
	CreatedAt   *time.Time `json:"createdAt"`
}
