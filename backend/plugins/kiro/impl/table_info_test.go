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

package impl

import (
	"testing"

	"github.com/apache/devlake/helpers/unithelper"
)

// The repo-wide plugins/table_info_test.go performs this same check for every
// plugin. Running it here as well keeps the feedback local: a model added
// without registering it in GetTablesInfo fails immediately rather than only
// in CI.
func TestKiroTableInfo(t *testing.T) {
	checker := unithelper.NewTableInfoChecker(unithelper.TableInfoCheckerConfig{})
	checker.FeedIn("../models", Kiro{}.GetTablesInfo)
	if err := checker.Verify(); err != nil {
		t.Error(err)
	}
}
