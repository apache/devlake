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

var _ plugin.MigrationScript = (*addProjectColumn)(nil)

type addProjectColumn struct{}

func (*addProjectColumn) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Add project column to connections table
	// MySQL returns error 1060 if column already exists, which is OK
	err := db.Exec("ALTER TABLE _tool_testregistry_connections ADD COLUMN project VARCHAR(200)")
	if err != nil {
		// Check if error is "duplicate column" (MySQL error 1060)
		errMsg := err.Error()
		if strings.Contains(errMsg, "Duplicate column name") || strings.Contains(errMsg, "1060") {
			// Column already exists, this is OK
		} else {
			// For other errors, return them
			return errors.Default.Wrap(err, "failed to add project column")
		}
	}

	return nil
}

func (*addProjectColumn) Version() uint64 {
	return 20250102000001
}

func (*addProjectColumn) Name() string {
	return "add project column to testregistry connections"
}
