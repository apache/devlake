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
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addScopeIdFields)(nil)

type addScopeIdFields struct{}

func (*addScopeIdFields) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Add scope_id column to _tool_q_dev_user_data table
	// This field links user data to QDevS3Slice scope, which can then be mapped to projects via project_mapping
	if !db.HasColumn("_tool_q_dev_user_data", "scope_id") {
		if err := db.AddColumn("_tool_q_dev_user_data", "scope_id", dal.Varchar); err != nil {
			return errors.Default.Wrap(err, "failed to add scope_id to _tool_q_dev_user_data")
		}
	}

	// Add index on scope_id for better query performance
	_ = db.Exec(`CREATE INDEX idx_q_dev_user_data_scope_id ON _tool_q_dev_user_data(scope_id)`)

	// Add scope_id column to _tool_q_dev_s3_file_meta table
	if !db.HasColumn("_tool_q_dev_s3_file_meta", "scope_id") {
		if err := db.AddColumn("_tool_q_dev_s3_file_meta", "scope_id", dal.Varchar); err != nil {
			return errors.Default.Wrap(err, "failed to add scope_id to _tool_q_dev_s3_file_meta")
		}
	}

	// Add index on scope_id
	_ = db.Exec(`CREATE INDEX idx_q_dev_s3_file_meta_scope_id ON _tool_q_dev_s3_file_meta(scope_id)`)

	return nil
}

func (*addScopeIdFields) Version() uint64 {
	return 20251209000001
}

func (*addScopeIdFields) Name() string {
	return "add scope_id field to QDevUserData and QDevS3FileMeta for project association"
}
