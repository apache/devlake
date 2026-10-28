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

var _ plugin.MigrationScript = (*addTriggerType)(nil)

type addTriggerType struct{}

func (*addTriggerType) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Add trigger_type column to ci_test_jobs table
	err := db.Exec("ALTER TABLE ci_test_jobs ADD COLUMN trigger_type VARCHAR(50)")
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Duplicate column name") && !strings.Contains(errMsg, "1060") {
			return errors.Default.Wrap(err, "failed to add trigger_type column")
		}
	}

	// Add index on trigger_type for efficient filtering
	// MySQL doesn't support IF NOT EXISTS, so we'll try to create it and ignore errors if it already exists
	err = db.Exec("CREATE INDEX idx_ci_test_jobs_trigger_type ON ci_test_jobs(trigger_type)")
	if err != nil {
		errMsg := err.Error()
		// Index might already exist (MySQL error 1061), ignore that case
		if !strings.Contains(errMsg, "Duplicate key name") && !strings.Contains(errMsg, "1061") {
			// For other errors, log but don't fail (index is optional for functionality)
			basicRes.GetLogger().Warn(err, "failed to create index on trigger_type")
		}
	}

	return nil
}

func (*addTriggerType) Version() uint64 {
	return 20250108000001
}

func (*addTriggerType) Name() string {
	return "add trigger_type column to ci_test_jobs table"
}
