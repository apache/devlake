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

package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
	"github.com/apache/incubator-devlake/plugins/q_dev/models/migrationscripts/archived"
)

type fixDedupUserTables struct{}

func (*fixDedupUserTables) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Drop old tables that used auto-increment ID (which caused data duplication)
	err := db.DropTables(
		"_tool_q_dev_user_report",
		"_tool_q_dev_user_data",
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to drop old user tables")
	}

	// Recreate tables with composite primary keys for proper deduplication
	err = migrationhelper.AutoMigrateTables(
		basicRes,
		&archived.QDevUserReportV2{},
		&archived.QDevUserDataV2{},
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to recreate user tables")
	}

	return nil
}

func (*fixDedupUserTables) Version() uint64 {
	return 20260228000001
}

func (*fixDedupUserTables) Name() string {
	return "Rebuild user_report and user_data tables with composite primary keys to fix data duplication"
}
