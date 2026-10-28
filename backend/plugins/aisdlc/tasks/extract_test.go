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
	"strings"
	"testing"
	"time"

	"github.com/apache/devlake/core/plugin"
	"github.com/apache/devlake/plugins/aisdlc/models"
)

func TestFilesFromRawKeepsCollectionTime(t *testing.T) {
	seen := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	later := seen.Add(24 * time.Hour)
	raws := []models.AisdlcRawFile{
		{FilePath: "RHAIRFE/20260719-041307/RHAIRFE-1.result.md", BlobId: "kept", CollectedAt: seen},
		{FilePath: "other/RHAIRFE-2.result.md", BlobId: "skipped", CollectedAt: later},
	}
	files := filesFromRaw(raws, "RHAIRFE")
	if len(files) != 1 {
		t.Fatalf("files = %+v", files)
	}
	if files[0].BlobID != "kept" || !files[0].CollectedAt.Equal(seen) {
		t.Fatalf("file = %+v", files[0])
	}
}

func TestScopeRawParamsMatchScopeDeleteKey(t *testing.T) {
	const connectionID = uint64(7)
	const fullName = "redhat/rhel-ai/agentic-ci/rfe-assess-data"
	scope := models.AisdlcScope{FullName: fullName}
	scope.ConnectionId = connectionID
	got := scopeRawParams(connectionID, fullName)
	want := plugin.MarshalScopeParams(scope.ScopeParams())
	if got != want {
		t.Fatalf("scope params = %s, delete key = %s", got, want)
	}
	if !strings.Contains(got, fullName) || !strings.Contains(got, `"connectionId":7`) {
		t.Fatalf("params = %s", got)
	}
}
