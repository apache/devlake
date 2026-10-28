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
	"strings"

	"github.com/apache/devlake/core/context"
	"github.com/apache/devlake/core/errors"
	"github.com/apache/devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addJUnitRegexColumn)(nil)

type addJUnitRegexColumn struct{}

func (*addJUnitRegexColumn) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Add junit_regex column
	err := db.Exec("ALTER TABLE _tool_testregistry_connections ADD COLUMN junit_regex VARCHAR(500)")
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Duplicate column name") && !strings.Contains(errMsg, "1060") {
			return errors.Default.Wrap(err, "failed to add junit_regex column")
		}
	}

	// Note: We don't set a default value in the database because we handle the default
	// in the application code. This allows existing connections to continue working
	// with the default regex pattern without requiring database updates.

	return nil
}

func (*addJUnitRegexColumn) Version() uint64 {
	return 20250113000001
}

func (*addJUnitRegexColumn) Name() string {
	return "add junit_regex column to testregistry connections for configurable JUnit file matching"
}
