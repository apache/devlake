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

package e2e

import (
	"testing"

	"github.com/apache/devlake/core/models/common"
	"github.com/apache/devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/devlake/helpers/e2ehelper"
	"github.com/apache/devlake/plugins/youtrack/impl"
	"github.com/apache/devlake/plugins/youtrack/models"
	"github.com/apache/devlake/plugins/youtrack/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYoutrackConvertAccountsSkipsGuest covers the guest trap: YouTrack
// silently degrades to the guest user on missing auth, so the built-in
// guest account must never reach the domain layer. The live
// fixture carries no guest refs, so the tool table is seeded directly.
func TestYoutrackConvertAccountsSkipsGuest(t *testing.T) {
	var youtrack impl.Youtrack
	dataflowTester := e2ehelper.NewDataFlowTester(t, "youtrack", youtrack)

	dataflowTester.ImportCsvIntoTabler("./snapshot_tables/_tool_youtrack_accounts_seed.csv", &models.YoutrackAccount{})
	dataflowTester.FlushTabler(&crossdomain.Account{})
	dataflowTester.Subtask(tasks.ConvertAccountsMeta, newTaskData("0-1", zeroConfig()))

	dataflowTester.VerifyTableWithOptions(crossdomain.Account{}, e2ehelper.TableOptions{
		CSVRelPath:  "./snapshot_tables/accounts_guest_skipped.csv",
		IgnoreTypes: []interface{}{common.NoPKModel{}},
	})

	var accountCount int64
	require.NoError(t, dataflowTester.Db.Model(&crossdomain.Account{}).Count(&accountCount).Error)
	assert.Equal(t, int64(2), accountCount, "the guest account never converts")
}
