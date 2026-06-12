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

import "testing"

func TestSonarqubeSearchProjectsQueryFilter(t *testing.T) {
	t.Parallel()
	if g, w := sonarqubeSearchProjectsQueryFilter("iap"), `query = "iap"`; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
	if g, w := sonarqubeSearchProjectsQueryFilter("  my term  "), `query = "my term"`; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
	// double quotes are stripped to avoid breaking the filter string
	if g, w := sonarqubeSearchProjectsQueryFilter(`a"x`), `query = "ax"`; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
}
