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

var _ plugin.MigrationScript = (*addCiToolFields)(nil)

type addCiToolFields struct{}

func (*addCiToolFields) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Add ci_tool column
	err := db.Exec("ALTER TABLE _tool_testregistry_connections ADD COLUMN ci_tool VARCHAR(50)")
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Duplicate column name") && !strings.Contains(errMsg, "1060") {
			return errors.Default.Wrap(err, "failed to add ci_tool column")
		}
	}

	// Add github_organization column
	err = db.Exec("ALTER TABLE _tool_testregistry_connections ADD COLUMN github_organization VARCHAR(200)")
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Duplicate column name") && !strings.Contains(errMsg, "1060") {
			return errors.Default.Wrap(err, "failed to add github_organization column")
		}
	}

	// Add github_token column (for encrypted storage)
	err = db.Exec("ALTER TABLE _tool_testregistry_connections ADD COLUMN github_token TEXT")
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "Duplicate column name") && !strings.Contains(errMsg, "1060") {
			return errors.Default.Wrap(err, "failed to add github_token column")
		}
	}

	// Set default ci_tool to 'Tekton CI' for existing records (migrate existing data)
	// Also ensure quay_organization is set where it exists
	db.Exec("UPDATE _tool_testregistry_connections SET ci_tool = 'Tekton CI' WHERE ci_tool IS NULL OR ci_tool = ''")

	return nil
}

func (*addCiToolFields) Version() uint64 {
	return 20250106000001
}

func (*addCiToolFields) Name() string {
	return "add CI tool fields to testregistry connections"
}
