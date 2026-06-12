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

package auth

import "testing"

func TestSafeReturnURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "/"},
		{"root", "/", "/"},
		{"normal path", "/projects", "/projects"},
		{"path with query", "/projects?tab=2", "/projects?tab=2"},
		{"path with fragment kept", "/projects#section", "/projects"},
		{"absolute http URL", "http://evil.com/x", "/"},
		{"absolute https URL", "https://evil.com/x", "/"},
		{"protocol-relative", "//evil.com", "/"},
		{"protocol-relative with path", "//evil.com/x", "/"},
		{"backslash variant", `/\evil.com`, "/"},
		{"backslash variant with path", `/\evil.com/x`, "/"},
		{"missing leading slash", "projects", "/"},
		{"javascript scheme", "javascript:alert(1)", "/"},
		{"data scheme", "data:text/html,x", "/"},
		{"unparsable", "://", "/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeReturnURL(tc.in); got != tc.want {
				t.Errorf("safeReturnURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
