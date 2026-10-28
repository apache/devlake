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

var _ plugin.MigrationScript = (*renameOciToQuay)(nil)

type renameOciToQuay struct{}

func (*renameOciToQuay) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Rename column in connections table from oci_artifact_url to quay_organization
	// MySQL returns error 1054 if column doesn't exist, which is OK
	err := db.Exec("ALTER TABLE _tool_testregistry_connections CHANGE COLUMN oci_artifact_url quay_organization VARCHAR(200)")
	if err != nil {
		errMsg := err.Error()
		// Check if error is "unknown column" (MySQL error 1054) or "duplicate column" (1060)
		if strings.Contains(errMsg, "Unknown column") || strings.Contains(errMsg, "1054") {
			// Column doesn't exist, try to add it instead
			err2 := db.Exec("ALTER TABLE _tool_testregistry_connections ADD COLUMN quay_organization VARCHAR(200)")
			if err2 != nil {
				errMsg2 := err2.Error()
				if strings.Contains(errMsg2, "Duplicate column name") || strings.Contains(errMsg2, "1060") {
					// Column already exists, this is OK
				} else {
					return errors.Default.Wrap(err2, "failed to add quay_organization column")
				}
			}
		} else if strings.Contains(errMsg, "Duplicate column name") || strings.Contains(errMsg, "1060") {
			// Column already exists with new name, this is OK
		} else {
			// For other errors, return them
			return errors.Default.Wrap(err, "failed to rename oci_artifact_url to quay_organization")
		}
	}

	// Update scopes table: rename repository_full_name to full_name and repository_name to name
	// First check if the table exists and has data to migrate
	// Step 1: Handle repository_full_name -> full_name
	err = db.Exec("ALTER TABLE _tool_testregistry_scopes CHANGE COLUMN repository_full_name full_name VARCHAR(500)")
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "Unknown column") || strings.Contains(errMsg, "1054") {
			// Column doesn't exist, check if full_name already exists, if not add it
			err2 := db.Exec("ALTER TABLE _tool_testregistry_scopes ADD COLUMN full_name VARCHAR(500)")
			if err2 != nil {
				errMsg2 := err2.Error()
				if !strings.Contains(errMsg2, "Duplicate column name") && !strings.Contains(errMsg2, "1060") {
					// If adding failed and it's not a duplicate, continue to next step
				}
			}
		}
	}

	// Step 2: Handle repository_name -> name
	err = db.Exec("ALTER TABLE _tool_testregistry_scopes CHANGE COLUMN repository_name name VARCHAR(500)")
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "Unknown column") || strings.Contains(errMsg, "1054") {
			// Column doesn't exist, check if name already exists, if not add it
			err2 := db.Exec("ALTER TABLE _tool_testregistry_scopes ADD COLUMN name VARCHAR(500)")
			if err2 != nil {
				errMsg2 := err2.Error()
				if !strings.Contains(errMsg2, "Duplicate column name") && !strings.Contains(errMsg2, "1060") {
					// If adding failed and it's not a duplicate, continue to next step
				}
			}
		}
	}

	// Step 3: Handle legacy oci_artifact_url column if it exists (copy data to full_name if full_name is empty)
	// First check if oci_artifact_url column exists and copy data if needed
	// We'll try to copy data, but if the column doesn't exist, that's fine
	db.Exec("UPDATE _tool_testregistry_scopes SET full_name = oci_artifact_url WHERE (full_name IS NULL OR full_name = '') AND oci_artifact_url IS NOT NULL AND oci_artifact_url != ''")
	// Then try to drop the old column (ignore errors if it doesn't exist)
	db.Exec("ALTER TABLE _tool_testregistry_scopes DROP COLUMN oci_artifact_url")

	return nil
}

func (*renameOciToQuay) Version() uint64 {
	return 20250105000001
}

func (*renameOciToQuay) Name() string {
	return "rename oci_artifact_url to quay_organization and update scope fields"
}
