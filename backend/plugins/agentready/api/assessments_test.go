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

package api

import (
	"testing"
)

func TestParseConnectionId(t *testing.T) {
	tests := []struct {
		name    string
		params  map[string]string
		want    uint64
		wantErr bool
	}{
		{"valid id", map[string]string{"connectionId": "42"}, 42, false},
		{"missing id", map[string]string{}, 0, true},
		{"invalid id", map[string]string{"connectionId": "abc"}, 0, true},
		{"zero id", map[string]string{"connectionId": "0"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConnectionId(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseConnectionId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseConnectionId() = %v, want %v", got, tt.want)
			}
		})
	}
}
